package firewall

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/itchyny/gojq"
	pi "knative.dev/security-guard/pkg/pluginterfaces"
)

// func evaluateRules(dir Direction, body interface{}) (Action, error) {
// 	for i, rule := range dir.Rules {
// 		if rule.Body == nil {
// 			continue
// 		}

// 		match, err := evaluateBodyRule(rule.Body, body)
// 		if err != nil {
// 			return "", fmt.Errorf("rule[%d]: %w", i, err)
// 		}

// 		if match {
// 			pi.Log.Debugf("Rule[%d] matched → action=%s", i, rule.Body.Action)
// 			return rule.Body.Action, nil
// 		}
// 	}

// 	// No rule matched → default action
// 	return dir.DefaultAction, nil
// }

func evaluateRules(dir Direction, body interface{}) (Action, error) {
	for i := range dir.Rules {
		rule := &dir.Rules[i]

		pi.Log.Debugf("Checking rule expression: %s", rule.Expression)

		match, err := evaluateRule(rule, body)
		if err != nil {
			return "", fmt.Errorf("rules[%d]: %w", i, err)
		}

		if !match {
			continue
		}

		switch rule.Action {
		case ActionLog:
			logRuleHit(rule, body)
			continue // logging is non-terminal

		case ActionAccept, ActionDrop, ActionReject:
			return rule.Action, nil
		}
	}

	// No rule matched → default action
	return dir.DefaultAction, nil
}

func evaluateRule(rule *Rule, body interface{}) (bool, error) {
	if strings.TrimSpace(rule.Expression) == "" {
		return false, fmt.Errorf("missing jq expression")
	}

	query, err := gojq.Parse(rule.Expression)
	if err != nil {
		return false, fmt.Errorf("invalid jq expression: %w", err)
	}

	iter := query.Run(body)

	var (
		result    bool
		hasResult bool
	)

	for {
		v, ok := iter.Next()
		if !ok {
			break
		}

		if err, isErr := v.(error); isErr {
			return false, err
		}

		b, ok := v.(bool)
		if !ok {
			return false, fmt.Errorf("jq expression did not return boolean")
		}

		if hasResult {
			return false, fmt.Errorf("jq expression returned multiple values")
		}

		result = b
		hasResult = true
	}

	if !hasResult {
		return false, fmt.Errorf("jq expression returned no result")
	}

	return result, nil
}

func logRuleHit(rule *Rule, body interface{}) {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		bodyJSON = []byte("<failed to marshal body>")
	}

	pi.Log.Infof(
		"Firewall LOG rule hit | action=%s | expression=%q | body=%s",
		rule.Action,
		rule.Expression,
		string(bodyJSON),
	)
}
