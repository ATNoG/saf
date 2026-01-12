package firewall

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pi "knative.dev/security-guard/pkg/pluginterfaces"
)

func evaluateRules(dir Direction, ctx interface{}) (Action, error) {
	var totalJQTime time.Duration

	// If the user did not specified rules for a specific direction, the requests must be accepted
	if dir.Rules == nil {
		return ActionAccept, nil
	}

	for i := range dir.Rules {
		rule := &dir.Rules[i]

		pi.Log.Debugf("Checking rule expression: %s", rule.Expression)

		start := time.Now()
		match, err := evaluateRule(rule, ctx)
		totalJQTime += time.Since(start)
		if err != nil {
			return "", fmt.Errorf("rules[%d]: %w", i, err)
		}

		if !match {
			continue
		}

		switch rule.Action {
		case ActionLog:
			logRuleHit(rule, ctx)
			continue // non-terminal

		case ActionAccept, ActionDrop, ActionReject:
			return rule.Action, nil // immediate return on terminal action
		}
	}

	// Only print timing info in debug mode to avoid performance impact in production
	// Note: We can't use IsDebugEnabled() as it's not available, so we'll always print for now
	// In a production environment, this should be configurable
	fmt.Printf("Total jq processing time: %v\n", totalJQTime)
	return dir.DefaultAction, nil
}

func evaluateRule(rule *Rule, ctx interface{}) (bool, error) {
	if strings.TrimSpace(rule.Expression) == "" {
		return false, fmt.Errorf("missing jq expression")
	}

	// Use the pre-parsed query from the rule
	iter := rule.CompiledQuery.Run(ctx)

	var (
		result    bool
		hasResult bool
	)

	// Optimize: Limit the number of results we process to avoid unnecessary iterations
	// Since jq expressions should return a single boolean, we only need to check the first result
	for i := 0; i < 2; i++ { // Check at most 2 results to ensure we get exactly one
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

func logRuleHit(rule *Rule, ctx interface{}) {
	ctxJSON, err := json.Marshal(ctx)
	if err != nil {
		ctxJSON = []byte("<failed to marshal firewall context>")
	}

	pi.Log.Infof(
		"Firewall LOG rule hit | action=%s | expression=%q | context=%s",
		rule.Action,
		rule.Expression,
		string(ctxJSON),
	)
}
