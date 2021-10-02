package proxy

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
)

type Action string

const (
	// Do not allow the request under any circumstances
	Reject Action = "REJECT"
	// Only allow requests that have a captured response
	Locked Action = "LOCKED"
	// Allow requests matching the pattern, preferring cached responses
	Allow Action = "ALLOW"
	// Always refresh, even when in cache
	Refresh Action = "REFRESH"
	// Always allow the request, and never cache the response. This should only be used for authentication tokens.
	RefreshNoStore Action = "NO_STORE"
)

func (a Action) String() string {
	switch a {
	case Reject, Locked, Allow, Refresh, RefreshNoStore:
		return string(a)
	default:
		return "REJECT"
	}
}

func ParseAction(s string) Action {
	switch a := Action(strings.ToUpper(s)); a {
	case Reject, Locked, Allow, Refresh, RefreshNoStore:
		return a
	default:
		return Reject
	}
}

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

func (r *Rule) MatchString(url string) bool {
	return r.pattern.MatchString(url)
}

type Filter struct {
	log     logr.Logger
	handler http.Handler
	Rules   []*Rule
}

func NewFilter(log logr.Logger, handler http.Handler, rules ...*Rule) *Filter {
	return &Filter{
		log:     log,
		handler: handler,
		Rules:   rules,
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
			f.log.Info("locked", "pattern", rule.pattern.String(), "url", url)
			f.handler.ServeHTTP(w, newLockedRequest(r))
		case Allow:
			f.log.Info("allow", "pattern", rule.pattern.String(), "url", url)
			f.handler.ServeHTTP(w, r)
		case Refresh:
			f.log.Info("refresh", "pattern", rule.pattern.String(), "url", url)
			f.handler.ServeHTTP(w, newRefreshRequest(r))
		case RefreshNoStore:
			f.log.Info("no store", "pattern", rule.pattern.String(), "url", url)
			f.handler.ServeHTTP(w, newNoStoreRequest(r))
		case Reject:
			fallthrough
		default:
			f.log.Info("reject", "pattern", rule.pattern.String(), "url", url)
			w.WriteHeader(http.StatusForbidden)
		}
		return
	}

	// No rules match, go away
	w.WriteHeader(http.StatusForbidden)
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
