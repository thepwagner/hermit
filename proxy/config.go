package proxy

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Rules []*Rule
}

func LoadConfigFile(fn string) (*Config, error) {
	f, err := os.Open(fn)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, err
	}
	return LoadConfig(f)
}

func LoadConfig(in io.Reader) (*Config, error) {
	var config map[string]interface{}
	if err := yaml.NewDecoder(in).Decode(&config); err != nil {
		return nil, err
	}

	rawRules, ok := config["rules"].([]interface{})
	if !ok {
		return nil, nil
	}
	rules := make([]*Rule, 0, len(rawRules))
	for _, rawRule := range rawRules {
		rule, ok := rawRule.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid rule")
		}
		pattern, ok := rule["pattern"].(string)
		if !ok {
			return nil, fmt.Errorf("rule missing pattern")
		}

		rawAction, ok := rule["action"].(string)
		if !ok {
			return nil, fmt.Errorf("rule missing action")
		}

		var action Action
		switch strings.ToUpper(rawAction) {
		case "REJECT":
			action = Reject
		case "LOCKED":
			action = Locked
		case "ALLOW":
			action = Allow
		case "REFRESH":
			action = Refresh
		case "NO_STORE", "REFRESH_NO_STORE":
			action = RefreshNoStore
		default:
			action = Reject
		}

		newRule, err := NewRule(pattern, action)
		if err != nil {
			return nil, err
		}
		rules = append(rules, newRule)
	}
	return &Config{Rules: rules}, nil
}
