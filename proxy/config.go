package proxy

import (
	"errors"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Rules []*Rule
}

type rawRule struct {
	Pattern string `yaml:"pattern"`
	Action  string `yaml:"action"`
}

type configRaw struct {
	Rules []rawRule `yaml:"rules"`
}

func LoadConfigFile(fn string) (*Config, error) {
	f, err := os.Open(fn)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, err
	}
	defer f.Close()
	return LoadConfig(f)
}

func LoadConfig(in io.Reader) (*Config, error) {
	var config configRaw
	if err := yaml.NewDecoder(in).Decode(&config); err != nil {
		return nil, err
	}

	rules := make([]*Rule, 0, len(config.Rules))
	for _, rawRule := range config.Rules {
		fmt.Println("loading", rawRule.Pattern, rawRule.Action, ParseAction(rawRule.Action))
		newRule, err := NewRule(rawRule.Pattern, ParseAction(rawRule.Action))
		if err != nil {
			return nil, err
		}
		rules = append(rules, newRule)
	}
	return &Config{Rules: rules}, nil
}

func (c *Config) Save(fn string) error {
	var config configRaw
	for _, rule := range c.Rules {
		config.Rules = append(config.Rules, rawRule{
			Pattern: rule.pattern.String(),
			Action:  rule.Action.String(),
		})
	}

	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	defer f.Close()
	return yaml.NewEncoder(io.MultiWriter(f, os.Stdout)).Encode(&config)
}
