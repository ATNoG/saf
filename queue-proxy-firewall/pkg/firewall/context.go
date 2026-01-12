package firewall

import (
	"net/http"
)

type firewallCtxKey struct{}

func buildRequestContext(req *http.Request, body interface{}) map[string]interface{} {
	// Pre-allocate maps with reasonable initial capacity to reduce rehashing
	headers := make(map[string][]string, len(req.Header))
	for k, v := range req.Header {
		headers[http.CanonicalHeaderKey(k)] = v
	}

	query := make(map[string][]string, len(req.URL.Query()))
	for k, v := range req.URL.Query() {
		query[k] = v
	}

	// Pre-allocate URI map with exact capacity needed
	uriMap := make(map[string]interface{}, 5)
	uriMap["scheme"] = req.URL.Scheme
	uriMap["host"] = req.Host
	uriMap["path"] = req.URL.Path
	uriMap["rawQuery"] = req.URL.RawQuery
	uriMap["query"] = query

	// Reuse the same map instance to avoid creating new maps
	result := make(map[string]interface{}, 4)
	result["BODY"] = body
	result["HEADERS"] = headers
	result["URI"] = uriMap
	result["METHOD"] = req.Method

	return result
}

func buildResponseContext(resp *http.Response, body interface{}) map[string]interface{} {
	// Pre-allocate headers map with reasonable initial capacity
	headers := make(map[string][]string, len(resp.Header))
	for k, v := range resp.Header {
		headers[http.CanonicalHeaderKey(k)] = v
	}

	// Reuse the same map instance to avoid creating new maps
	result := make(map[string]interface{}, 3)
	result["BODY"] = body
	result["HEADERS"] = headers
	result["STATUS"] = resp.StatusCode

	return result
}
