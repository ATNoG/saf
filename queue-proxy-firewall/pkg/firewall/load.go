package firewall

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/goccy/go-yaml"
	pi "knative.dev/security-guard/pkg/pluginterfaces"

	"github.com/santhosh-tekuri/jsonschema/v6"

	_ "embed"
)

//go:embed firewall-schema.yaml
var firewallSchemaYAML []byte

func loadFirewallSchema(schemaYaml []byte) (*jsonschema.Schema, error) {
    var raw interface{}
    if err := yaml.Unmarshal(schemaYaml, &raw); err != nil {
        return nil, fmt.Errorf("invalid YAML schema: %w", err)
    }

    compiler := jsonschema.NewCompiler()
    if err := compiler.AddResource("firewall-schema.json", raw); err != nil {
        return nil, fmt.Errorf("failed to add schema resource: %w", err)
    }

	schema, err := compiler.Compile("firewall-schema.json")
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}

	return schema, nil
}

func loadUserFirewall(config map[string]string) (*Firewall, error) {
    var raw string
    var isYAML bool

    // YAML takes precedence
    if yml, ok := config["rules"]; ok {
        raw = yml
        isYAML = true
        if _, ok := config["rules-json"]; ok {
            pi.Log.Warnf("Both JSON and YAML rules provided. YAML rules take precedence, JSON ignored")
        }
    } else if j, ok := config["rules-json"]; ok {
        raw = j
        isYAML = false
    } else {
        return nil, errors.New("no firewall rules provided")
    }

    // unescape
    unescaped, err := strconv.Unquote("\"" + raw + "\"")
    if err != nil {
        return nil, fmt.Errorf("failed to unescape rules: %w", err)
    }

    // parse
    var data interface{}
    if isYAML {
        if err := yaml.Unmarshal([]byte(unescaped), &data); err != nil {
            return nil, fmt.Errorf("invalid YAML: %w", err)
        }
    } else {
        if err := json.Unmarshal([]byte(unescaped), &data); err != nil {
            return nil, fmt.Errorf("invalid JSON: %w", err)
        }
    }

	firewallSchema, err := loadFirewallSchema(firewallSchemaYAML)
	if err != nil {
		return nil, fmt.Errorf("Error loading schema: %v", err)
	}

    // validate against schema
    err = firewallSchema.Validate(data)
    if err != nil {
        return nil, fmt.Errorf("failed to validate rules against schema: %w", err)
    }

    // now unmarshal into Firewall struct
    var fw Firewall
    if isYAML {
        if err := yaml.Unmarshal([]byte(unescaped), &fw); err != nil {
            return nil, fmt.Errorf("failed to unmarshal YAML into Firewall struct: %w", err)
        }
    } else {
        if err := json.Unmarshal([]byte(unescaped), &fw); err != nil {
            return nil, fmt.Errorf("failed to unmarshal JSON into Firewall struct: %w", err)
        }
    }

    // semantic validation
    if err := fw.Validate(); err != nil {
        return nil, fmt.Errorf("semantic validation failed: %w", err)
    }

    return &fw, nil
}
