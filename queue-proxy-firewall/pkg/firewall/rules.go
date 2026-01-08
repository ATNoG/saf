package firewall

import (
	"encoding/json"
	"fmt"

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
	for i, rule := range dir.Rules {
		if rule.Body == nil {
			continue
		}

		match, err := evaluateBodyRule(rule.Body, body)
		if err != nil {
			return "", fmt.Errorf("rule[%d]: %w", i, err)
		}

		if !match {
			continue
		}

		switch rule.Body.Action {

			case ActionLog:
				logRuleHit(rule.Body, body)
				continue // keep evaluating

			case ActionAccept, ActionDrop, ActionReject:
				return rule.Body.Action, nil
		}
	}

	// No terminal rule matched → default action
	return dir.DefaultAction, nil
}


func evaluateBodyRule(rule *BodyRule, body interface{}) (bool, error) {
	// If schema is present, expression may be absent (future extension)
	if rule.Expression == "" {
		return false, nil
	}

	query, err := gojq.Parse(rule.Expression)
	if err != nil {
		return false, fmt.Errorf("invalid jq expression: %w", err)
	}

	iter := query.Run(body)

	for {
		v, ok := iter.Next()
		if !ok {
			break
		}

		if err, isErr := v.(error); isErr {
			return false, err
		}

		// jq expressions must evaluate to boolean
		b, ok := v.(bool)
		if !ok {
			return false, fmt.Errorf("jq expression did not return boolean")
		}

		return b, nil
	}

	return false, nil
}

func logRuleHit(rule *BodyRule, body interface{}) {
	bodyJSON, _ := json.Marshal(body)

	pi.Log.Infof(
		"Firewall LOG rule hit | action=%s | type=%s | expression=%q | body=%s",
		rule.Action,
		rule.Type,
		rule.Expression,
		string(bodyJSON),
	)
}
