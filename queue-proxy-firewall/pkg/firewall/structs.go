package firewall

import "fmt"


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
	Body *BodyRule `json:"body,omitempty"`
}

type BodyRule struct {
	Type       BodyType    `json:"type"`
	Action     Action      `json:"action"`
	Expression string      `json:"expression,omitempty"`
	Schema     *BodySchema `json:"schema,omitempty"`
}

type BodyType string

const (
	BodyTypeJSON BodyType = "application/json"
)

type BodySchema struct {
	Type SchemaType `json:"type"`
	Path string     `json:"path"`
}

type SchemaType string

const (
	SchemaTypeJSON SchemaType = "json"
	SchemaTypeYAML SchemaType = "yaml"
)

func (b *BodyRule) Validate() error {
	if b.Type == "" {
		return fmt.Errorf("body.type is required")
	}

	if b.Action == "" {
		return fmt.Errorf("body.action is required")
	}

	if b.Schema != nil {
		// schema present → expression optional
		if b.Schema.Type == "" || b.Schema.Path == "" {
			return fmt.Errorf("schema.type and schema.path are required")
		}
	} else {
		// schema absent → expression required
		if b.Expression == "" {
			return fmt.Errorf("expression is required when schema is not present")
		}
	}

	return nil
}


func (f *Firewall) Validate() error {
	validateDir := func(name string, d *Direction) error {
		if d == nil {
			return nil
		}
		for i, r := range d.Rules {
			if r.Body != nil {
				if err := r.Body.Validate(); err != nil {
					return fmt.Errorf("%s.rules[%d]: %w", name, i, err)
				}
			}
		}
		return nil
	}

	if err := validateDir("request", f.Request); err != nil {
		return err
	}
	if err := validateDir("response", f.Response); err != nil {
		return err
	}
	return nil
}

