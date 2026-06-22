package firewall

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestEnforcementEngineActions(t *testing.T) {
	requestRules := validatedDirection(t, Direction{
		DefaultAction: ActionAccept,
		Rules: []Rule{
			{Action: ActionDrop, Expression: `.REQUEST.METHOD != "POST"`},
			{Action: ActionDrop, Expression: `.REQUEST.BODY.username? == "{{exploit}}"`},
			{Action: ActionReject, Expression: `.REQUEST.BODY.reject? == true`},
		},
	})
	responseRules := validatedDirection(t, Direction{
		DefaultAction: ActionAccept,
		Rules: []Rule{
			{Action: ActionDrop, Expression: `.RESPONSE.BODY.error? | strings | test("stacktrace:")`},
		},
	})

	tests := []struct {
		name  string
		rules Direction
		ctx   map[string]interface{}
		want  Action
	}{
		{
			name:  "method rule drops non-POST request",
			rules: requestRules,
			ctx:   requestContext("GET", map[string]interface{}{}),
			want:  ActionDrop,
		},
		{
			name:  "clean POST uses accept default",
			rules: requestRules,
			ctx:   requestContext("POST", map[string]interface{}{"username": "alice"}),
			want:  ActionAccept,
		},
		{
			name:  "injection rule drops request",
			rules: requestRules,
			ctx:   requestContext("POST", map[string]interface{}{"username": "{{exploit}}"}),
			want:  ActionDrop,
		},
		{
			name:  "reject rule rejects request",
			rules: requestRules,
			ctx:   requestContext("POST", map[string]interface{}{"reject": true}),
			want:  ActionReject,
		},
		{
			name:  "stack trace rule drops response",
			rules: responseRules,
			ctx:   responseContext(map[string]interface{}{"error": "stacktrace: details"}),
			want:  ActionDrop,
		},
		{
			name:  "ordinary error uses response accept default",
			rules: responseRules,
			ctx:   responseContext(map[string]interface{}{"error": "not found"}),
			want:  ActionAccept,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evaluateRules(tt.rules, tt.ctx)
			if err != nil {
				t.Fatalf("evaluateRules() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("evaluateRules() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEnforcementEngineStopsAtFirstTerminalMatch(t *testing.T) {
	rules := validatedDirection(t, Direction{
		DefaultAction: ActionReject,
		Rules: []Rule{
			{Action: ActionAccept, Expression: `true`},
			{Action: ActionDrop, Expression: `true`},
		},
	})

	got, err := evaluateRules(rules, map[string]interface{}{})
	if err != nil {
		t.Fatalf("evaluateRules() error = %v", err)
	}
	if got != ActionAccept {
		t.Fatalf("evaluateRules() = %q, want first matching action %q", got, ActionAccept)
	}
}

func TestMalformedJSONIsBlockedAndBodyIsRestored(t *testing.T) {
	const malformed = `{"username":`
	var restored []byte

	action, err := evaluateJSONFirewall(
		io.NopCloser(strings.NewReader(malformed)),
		func(body []byte) { restored = append([]byte(nil), body...) },
		Direction{},
		"test",
		func(body interface{}) map[string]interface{} { return map[string]interface{}{"BODY": body} },
	)

	if err == nil {
		t.Fatal("evaluateJSONFirewall() error = nil, want malformed JSON error")
	}
	if action != "" {
		t.Fatalf("evaluateJSONFirewall() action = %q, want no action on malformed JSON", action)
	}
	if !bytes.Equal(restored, []byte(malformed)) {
		t.Fatalf("restored body = %q, want %q", restored, malformed)
	}
}

func validatedDirection(t *testing.T, direction Direction) Direction {
	t.Helper()
	if err := direction.Validate("test"); err != nil {
		t.Fatalf("invalid test rules: %v", err)
	}
	return direction
}

func requestContext(method string, body interface{}) map[string]interface{} {
	return map[string]interface{}{
		"REQUEST": map[string]interface{}{
			"METHOD": method,
			"BODY":   body,
		},
	}
}

func responseContext(body interface{}) map[string]interface{} {
	return map[string]interface{}{
		"RESPONSE": map[string]interface{}{
			"BODY": body,
		},
	}
}
