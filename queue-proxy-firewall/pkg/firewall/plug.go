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
	"strconv"
	"strings"

	pi "knative.dev/security-guard/pkg/pluginterfaces"
	"knative.dev/serving/pkg/queue/sharedmain"
)

const version string = "0.0.1"
const name string = "flow"

var annotationsFilePath = sharedmain.PodInfoAnnotationsPath
var qpOptionPrefix = "qpoption.knative.dev/"

const (
	functionType = "function"
	eventType = "event"
)

type plug struct {
	name    		string
	version 		string

	RequestRules	Direction
	ResponseRules	Direction
}

var errRequest error = errors.New("Request blocked by firewall")
var errResponse error = errors.New("Response blocked by firewall")

// https://github.com/knative-extensions/security-guard/blob/v0.6.1/pkg/test-gate/test-gate.go

// ApproveRequest verifies the incoming JSON body against the defined firewall rules
func (p *plug) ApproveRequest(req *http.Request) (*http.Request, error) {
	restore := func(b []byte) {
		req.Body = io.NopCloser(bytes.NewReader(b))
		req.ContentLength = int64(len(b))
		req.Header.Set("Content-Length", strconv.FormatInt(int64(len(b)), 10))
		req.Header.Set("Content-Type", "application/json")
	}

	action, err := evaluateJSONFirewall(
		req.Body,
		restore,
		p.RequestRules,
		"ApproveRequest",
	)
	if err != nil {
		pi.Log.Errorf("%v", err)
		return nil, errRequest
	}

	if err := applyAction(action); err != nil {
		return nil, err
	}

	return req, nil
}


// ApproveResponse verifies the returned JSON body against the defined firewall rules
func (p *plug) ApproveResponse(
	req *http.Request,
	resp *http.Response,
) (*http.Response, error) {
	restore := func(b []byte) {
		resp.Body = io.NopCloser(bytes.NewReader(b))
		resp.ContentLength = int64(len(b))
		resp.Header.Set("Content-Length", strconv.FormatInt(int64(len(b)), 10))
	}

	action, err := evaluateJSONFirewall(
		resp.Body,
		restore,
		p.ResponseRules,
		"ApproveResponse",
	)
	if err != nil {
		pi.Log.Errorf("%v", err)
		return nil, errResponse
	}

	if err := applyAction(action); err != nil {
		return nil, err
	}

	return resp, nil
}

func evaluateJSONFirewall(
	bodyReader io.ReadCloser,
	restore func([]byte),
	rules Direction,
	logPrefix string,
) (Action, error) {
	bodyBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		return "", fmt.Errorf("%s: failed to read body: %w", logPrefix, err)
	}
	_ = bodyReader.Close()

	defer restore(bodyBytes)

	var body interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return "", fmt.Errorf("%s: invalid JSON body: %w", logPrefix, err)
	}

	action, err := evaluateRules(rules, body)
	if err != nil {
		return "", fmt.Errorf("%s: rule evaluation error: %w", logPrefix, err)
	}

	return action, nil
}

func applyAction(action Action) error {
	switch action {
		case ActionAccept:
			return nil
		// TODO -> IN THE FUTURE, THE BEHAVIOR OF DROP MUST BE CHANGED TO A REAL DROP (SILENT REJECT). HOWEVER, THIS NEEDS DEEPER MODIFICATIONS IN THE QUEUE-PROXY ITSELF AND PROBABLY IN THE SECURITY GUARD EXTENSION
		case ActionDrop:
			return errRequest
		case ActionReject:
			return errRequest
		default:
			return fmt.Errorf("unknown action: %s", action)
	}
}

// Init implements pluginterfaces.RoundTripPlug.
func (p *plug) Init(ctx context.Context, config map[string]string, serviceName string, namespace string, logger pi.Logger) context.Context {
	pi.Log.Infof("Plug %s: Never use in production", p.name)
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
	for scanner.Scan() {
		txt := scanner.Text()
		txt = strings.ToLower(txt)

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

	// get the firewall rules
	raw, ok := config["rules"]
	if !ok {
		pi.Log.Errorf("Key rules not found in config")
		return false
	}
	unescaped, err := strconv.Unquote("\"" + raw + "\"")
	if err != nil {
		pi.Log.Errorf("Failed to unescape JSON: %v", err)
		return false
	}

	var firewall Firewall

	decoder := json.NewDecoder(strings.NewReader(unescaped))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&firewall); err != nil {
		pi.Log.Errorf("Invalid firewall rules JSON: %v", err)
		return false
	}

	if err := firewall.Validate(); err != nil {
		pi.Log.Errorf("Firewall rules validation failed: %v", err)
		return false
	}

	return true
}

func init() {
	p := &plug{
		version: version,
		name:    name,
	}

	fName := os.Getenv("SERVING_SERVICE")
	if fName == "" {
		pi.Log.Errorf("Could not retrieve function name")
		return
	}

	if !p.ProcessAnnotations() {
		pi.Log.Errorf("Error reading the Pod annotations")
		return
	}

	pi.RegisterPlug(p)
}
