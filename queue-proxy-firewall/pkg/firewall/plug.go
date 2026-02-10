package firewall

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"runtime"
	"runtime/debug"
	"strconv"
	"strings"

	pi "knative.dev/security-guard/pkg/pluginterfaces"
	"knative.dev/serving/pkg/queue/sharedmain"
)

const version string = "0.0.1"
const name string = "firewall"

var annotationsFilePath = sharedmain.PodInfoAnnotationsPath
var qpOptionPrefix = "qpoption.knative.dev/"

type plug struct {
	name    string
	version string

	RequestRules  Direction
	ResponseRules Direction
}

var errRequest error = errors.New("Request blocked by firewall")
var errResponse error = errors.New("Response blocked by firewall")

func (p *plug) ApproveRequest(req *http.Request) (*http.Request, error) {
	restore := func(b []byte) {
		req.Body = io.NopCloser(bytes.NewReader(b))
		req.ContentLength = int64(len(b))
		req.Header.Set("Content-Length", strconv.FormatInt(int64(len(b)), 10))
		req.Header.Set("Content-Type", "application/json")
	}

	var reqCtx map[string]interface{}

	action, err := evaluateJSONFirewall(
		req.Body,
		restore,
		p.RequestRules,
		"ApproveRequest",
		func(body interface{}) map[string]interface{} {
			reqCtx = buildRequestContext(req, body)
			return map[string]interface{}{
				"REQUEST": reqCtx,
			}
		},
	)

	if err != nil {
		pi.Log.Errorf("%v", err)
		return nil, errors.New("request blocked by firewall")
	}

	// Persist request context for response phase
	req = req.WithContext(
		context.WithValue(req.Context(), firewallCtxKey{}, reqCtx),
	)

	if err := applyAction(action, errRequest); err != nil {
		return nil, err
	}

	return req, nil
}

func (p *plug) ApproveResponse(req *http.Request, resp *http.Response) (*http.Response, error) {
	restore := func(b []byte) {
		resp.Body = io.NopCloser(bytes.NewReader(b))
		resp.ContentLength = int64(len(b))
		resp.Header.Set("Content-Length", strconv.FormatInt(int64(len(b)), 10))
	}

	// Load request context
	var reqCtx map[string]interface{}
	if v := req.Context().Value(firewallCtxKey{}); v != nil {
		reqCtx, _ = v.(map[string]interface{})
	}

	action, err := evaluateJSONFirewall(
		resp.Body,
		restore,
		p.ResponseRules,
		"ApproveResponse",
		func(body interface{}) map[string]interface{} {
			return map[string]interface{}{
				"REQUEST":  reqCtx,
				"RESPONSE": buildResponseContext(resp, body),
			}
		},
	)

	if err != nil {
		pi.Log.Errorf("%v", err)
		return nil, errors.New("response blocked by firewall")
	}

	if err := applyAction(action, errResponse); err != nil {
		return nil, err
	}

	return resp, nil
}

func evaluateJSONFirewall(
	bodyReader io.ReadCloser,
	restore func([]byte),
	rules Direction,
	logPrefix string,
	contextBuilder func(body interface{}) map[string]interface{},
) (Action, error) {
	bodyBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		return "", fmt.Errorf("%s: failed to read body: %w", logPrefix, err)
	}
	_ = bodyReader.Close()

	defer restore(bodyBytes)

	// Performance optimization: Only parse JSON if body is not empty
	var body interface{}
	if len(bodyBytes) > 0 {
		// Performance optimization: Use json.Decoder for streaming parsing
		decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
		decoder.UseNumber() // Preserve number types for jq compatibility
		// Optimization: Pre-allocate buffer for decoder to reduce allocations
		decoder.Buffered()
		if err := decoder.Decode(&body); err != nil {
			return "", fmt.Errorf("%s: invalid JSON body: %w", logPrefix, err)
		}
	} else {
		body = nil
	}

	ctx := contextBuilder(body)

	action, err := evaluateRules(rules, ctx)
	if err != nil {
		return "", fmt.Errorf("%s: rule evaluation error: %w", logPrefix, err)
	}

	return action, nil
}

func applyAction(action Action, err error) error {
	switch action {
	case ActionAccept:
		return nil
	// TODO -> IN THE FUTURE, THE BEHAVIOR OF DROP MUST BE CHANGED TO A REAL DROP (SILENT REJECT). HOWEVER, THIS NEEDS DEEPER MODIFICATIONS IN THE QUEUE-PROXY ITSELF AND PROBABLY IN THE SECURITY GUARD EXTENSION
	case ActionDrop:
		return err
	case ActionReject:
		return err
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

// Init implements pluginterfaces.RoundTripPlug.
func (p *plug) Init(ctx context.Context, config map[string]string, serviceName string, namespace string, logger pi.Logger) context.Context {
	pi.Log.Infof("Plug %s: Never use in production", p.name)

	/* UNCOMMENT THE FOLLOWING SNIPPET TO MANUALLY TRIGGER THE GARBAGE COLLECTOR */
	pi.Log.Debugf("Running garbage collector")

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	pi.Log.Infof("Before FreeOSMemory: HeapAlloc=%d HeapSys=%d", ms.HeapAlloc, ms.HeapSys)
	runtime.GC()
	debug.FreeOSMemory()

	runtime.ReadMemStats(&ms)
	pi.Log.Infof("After FreeOSMemory: HeapAlloc=%d HeapSys=%d", ms.HeapAlloc, ms.HeapSys)
	pi.Log.Debugf("Garbage collector run")

	return ctx
}

// PlugName implements pluginterfaces.RoundTripPlug.
func (p *plug) PlugName() string {
	return p.name
}

// PlugVersion implements pluginterfaces.RoundTripPlug.
func (p *plug) PlugVersion() string {
	return p.version
}

// Shutdown implements pluginterfaces.RoundTripPlug.
func (p *plug) Shutdown() {
	pi.Log.Infof("Plug %s: Shutdown", p.name)
	// TODO -> implement shutdown logic
}

// Adapted from https://github.com/knative-extensions/security-guard/blob/1286b16537ffd94a4e2462e99e2f402bcbe9abe7/pkg/qpoption/qpoption.go#L41
func (p *plug) ProcessAnnotations() bool {
	file, err := os.Open(annotationsFilePath)
	if err != nil {
		pi.Log.Errorf("File %s cannot be opened - is PodInfo mounted? os.Open Error: %s", annotationsFilePath, err.Error())
		return false
	}
	defer file.Close()
	config := make(map[string]string)

	scanner := bufio.NewScanner(file)

	// To support very big instructions (configs up to 1MB)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		txt := scanner.Text()

		// Annotation structure:
		// 		either: <qpOptionPrefix><extension>-activate=s<val>
		// 		or:     <qpOptionPrefix><extension>-config-<key>=<val>
		parts := strings.SplitN(txt, "=", 2)

		k := parts[0] // <qpOptionPrefix><extension>-*
		v := parts[1] // <val>
		if strings.HasPrefix(k, qpOptionPrefix) && len(k) > len(qpOptionPrefix) {
			k = k[len(qpOptionPrefix):]

			// k structure: <extenion>-activate or <extension>-config-<key>
			keyParts := strings.Split(k, "-")
			if len(keyParts) < 2 {
				continue
			}
			extension := keyParts[0] // <extension>
			action := keyParts[1]    // activate or config
			if strings.EqualFold(extension, p.name) {
				// remove quotes if exists
				v = strings.TrimSuffix(strings.TrimPrefix(v, "\""), "\"")
				v = strings.TrimSuffix(strings.TrimPrefix(v, "'"), "'")
				if action == "config" && len(keyParts) >= 3 {
					extensionKey := strings.Join(keyParts[2:], "-")
					config[extensionKey] = v
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		pi.Log.Errorf("File %s - scanner Error %s", annotationsFilePath, err.Error())
		return false
	}

	firewall, err := loadUserFirewall(config)
	if err != nil {
		pi.Log.Errorf("Error loading firewall rules: %v", err)
		return false
	}

	if firewall.Request != nil {
		p.RequestRules = *firewall.Request
	} else {
		p.RequestRules = Direction{}
	}

	if firewall.Response != nil {
		p.ResponseRules = *firewall.Response
	} else {
		p.ResponseRules = Direction{}
	}

	return true
}

func init() {
	p := &plug{
		version: version,
		name:    name,
	}

	if !p.ProcessAnnotations() {
		pi.Log.Errorf("Error reading the Pod annotations")
		return
	}

	prettyRequest, err := json.MarshalIndent(p.RequestRules, "", "  ")
	if err != nil {
		pi.Log.Errorf("Failed to marshal request rules structure: %v", err)
	} else {
		pi.Log.Debugf("Request rules structure:\n%s\n", string(prettyRequest))
	}

	prettyResponse, err := json.MarshalIndent(p.ResponseRules, "", "  ")
	if err != nil {
		pi.Log.Errorf("Failed to marshal response rules structure: %v", err)
	} else {
		pi.Log.Debugf("Response rules structure:\n%s\n", string(prettyResponse))
	}

	pi.RegisterPlug(p)
}
