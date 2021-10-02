package proxy

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"github.com/go-logr/logr"
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
	action  Action
}

func NewRule(pattern string, action Action) (*Rule, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &Rule{
		pattern: re,
		action:  action,
	}, nil
}

func MustNewRule(pattern string, action Action) *Rule {
	rule, err := NewRule(pattern, action)
	if err != nil {
		panic(err)
	}
	return rule
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
		if !rule.pattern.MatchString(url) {
			continue
		}

		switch rule.action {
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
