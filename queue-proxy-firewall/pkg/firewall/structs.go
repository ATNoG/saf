package firewall

import (
	"fmt"
	"strings"

	"github.com/itchyny/gojq"
)


type Firewall struct {
	Request  *Direction `json:"request,omitempty"`
	Response *Direction `json:"response,omitempty"`
}


type Direction struct {
	DefaultAction 	Action `json:"default-action"`
	Rules         	[]Rule `json:"rules"`
}

type Action string

const (
	ActionDrop   Action = "drop"
	ActionAccept Action = "accept"
	ActionReject Action = "reject"
	ActionLog    Action = "log"
)

type Rule struct {
	Action     			Action      `json:"action"`
	Expression 			string      `json:"expression,omitempty"`
	Schema     			*BodySchema `json:"schema,omitempty"`
	CompiledQuery      	*gojq.Code `json:"-"` // Parsed query for later usage
}

type BodySchema struct {
	Type 	SchemaType 	`json:"type"`
	Path 	string     	`json:"path"`
}

type SchemaType string

const (
	SchemaTypeJSON SchemaType = "json"
	SchemaTypeYAML SchemaType = "yaml"
)

func (r *Rule) Validate() error {
	// Action validation
	switch r.Action {
	case ActionAccept, ActionDrop, ActionReject, ActionLog:
		// ok
	default:
		return fmt.Errorf("invalid action: %q", r.Action)
	}

	// Expression is mandatory
	if strings.TrimSpace(r.Expression) == "" {
		return fmt.Errorf("expression must not be empty")
	}

	// Validate jq syntax early (IMPORTANT)
	parsedQuery, err := gojq.Parse(r.Expression)
	if err != nil {
		return fmt.Errorf("invalid jq expression: %w", err)
	}
	
	// Compile the parsed query
	code, err := gojq.Compile(parsedQuery)
	if err != nil {
		return fmt.Errorf("could not compile the jq query: %w", err)
	}

	// Save the compiled query for later usage
	r.CompiledQuery = code

	// Schema validation (if present)
	if r.Schema != nil {
		switch r.Schema.Type {
		case SchemaTypeJSON, SchemaTypeYAML:
			// ok
		default:
			return fmt.Errorf("invalid schema.type: %q", r.Schema.Type)
		}

		if strings.TrimSpace(r.Schema.Path) == "" {
			return fmt.Errorf("schema.path must not be empty")
		}
	}

	return nil
}

func (d *Direction) Validate(name string) error {
	if d == nil {
		return nil
	}

	// default-action must be terminal
	switch d.DefaultAction {
	case ActionAccept, ActionDrop, ActionReject:
		// ok
	default:
		return fmt.Errorf("%s.default-action must be accept, drop, or reject", name)
	}

	if len(d.Rules) == 0 {
		return fmt.Errorf("%s.rules must not be empty", name)
	}

	for i := range d.Rules {
		if err := d.Rules[i].Validate(); err != nil {
			return fmt.Errorf("%s.rules[%d]: %w", name, i, err)
		}
	}

	return nil
}

func (f *Firewall) Validate() error {
	if f.Request == nil && f.Response == nil {
		return fmt.Errorf("at least one of request or response must be defined")
	}

	if err := f.Request.Validate("request"); err != nil {
		return err
	}

	if err := f.Response.Validate("response"); err != nil {
		return err
	}

	return nil
}


