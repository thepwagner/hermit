package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
)

type Action int

const (
	// Do not allow the request under any circumstances
	Reject Action = iota
	// Only allow requests that have a captured response
	Locked
	// Allow requests matching the pattern, preferring cached responses
	Allow
	// Always refresh, even when in cache
	Refresh
	// Always allow the request, and never cache the response. This should only be used for authentication tokens.
	RefreshNoStore
)

type Rule struct {
	pattern *regexp.Regexp
	Action  Action
}

func NewRule(pattern string, action Action) (*Rule, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &Rule{
		pattern: re,
		Action:  action,
	}, nil
}

func LoadRules(in io.Reader) ([]*Rule, error) {
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
	return rules, nil
}

func (r *Rule) MatchString(url string) bool {
	return r.pattern.MatchString(url)
}

type Filter struct {
	log     logr.Logger
	handler http.Handler
	Rules   []*Rule
}

func NewFilter(log logr.Logger, handler http.Handler) *Filter {
	return &Filter{
		log:     log,
		handler: handler,
	}
}

func (f *Filter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	url := fmt.Sprintf("%s%s", r.URL.Host, r.URL.Path)
	f.log.Info("request", "url", url)
	for _, rule := range f.Rules {
		if !rule.MatchString(url) {
			continue
		}

		switch rule.Action {
		case Locked:
			f.handler.ServeHTTP(w, newLockedRequest(r))
		case Allow:
			f.handler.ServeHTTP(w, r)
		case Refresh:
			f.handler.ServeHTTP(w, newRefreshRequest(r))
		case RefreshNoStore:
			f.handler.ServeHTTP(w, newNoStoreRequest(r))
		case Reject:
			fallthrough
		default:
			f.log.Info("reject", "pattern", rule.pattern.String(), "url", url)
			w.WriteHeader(http.StatusForbidden)
		}
		return
	}
}

type ctxKey string

var (
	locked  = ctxKey("locked")
	refresh = ctxKey("refresh")
	noStore = ctxKey("no-store")
)

func newLockedRequest(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), locked, struct{}{})
	return r.WithContext(ctx)
}

func newRefreshRequest(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), refresh, struct{}{})
	return r.WithContext(ctx)
}

func newNoStoreRequest(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), noStore, struct{}{})
	return r.WithContext(ctx)
}

func lockedRequest(r *http.Request) bool {
	_, ok := r.Context().Value(locked).(struct{})
	return ok
}

func refreshRequest(r *http.Request) bool {
	_, ok := r.Context().Value(refresh).(struct{})
	return ok
}

func noStoreRequest(r *http.Request) bool {
	_, ok := r.Context().Value(noStore).(struct{})
	return ok
}
