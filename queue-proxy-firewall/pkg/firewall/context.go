package firewall

import (
	"net/http"
	"strings"
)

type firewallCtxKey struct{}

type RequestContext map[string]interface{}
type ResponseContext map[string]interface{}
type FirewallContext map[string]interface{}


type JQContext map[string]interface{}

func buildRequestContext(req *http.Request, body interface{}) RequestContext {
	headers := make(map[string][]string)
	for k, v := range req.Header {
		headers[strings.ToLower(k)] = v
	}

	query := make(map[string][]string)
	for k, v := range req.URL.Query() {
		query[k] = v
	}

	return RequestContext{
		"BODY": body,
		"HEADERS": headers,
		"URI": map[string]interface{}{
			"scheme":   req.URL.Scheme,
			"host":     req.Host,
			"path":     req.URL.Path,
			"rawQuery": req.URL.RawQuery,
			"query":    query,
		},
		"METHOD": req.Method,
	}
}

func buildResponseContext(resp *http.Response, body interface{}) JQContext {
	headers := make(map[string][]string)
	for k, v := range resp.Header {
		headers[strings.ToLower(k)] = v
	}

	return JQContext{
		"BODY":    body,
		"HEADERS": headers,
		"STATUS":  resp.StatusCode,
	}
}

