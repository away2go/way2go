package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/away2go/way2go/activity"
)

// reservedPrefix marks the name of a synthetic activity.Middleware entry
// declared internally by this package's own binding options (see binding.go).
// No Way2Go middleware author can produce a name
// with this prefix through the public API (activity.NewWebMiddleware and
// activity.NewMiddleware take a plain human-authored string, and nothing in
// this package ever offers a caller control over this exact byte
// sequence), so it is a safe, collision-free marker: web.Activity strips
// any Middleware entry carrying it from the public Descriptor.Middleware
// view.
const reservedPrefix = "\x00web:"

// bindingProbe is the private, empty probe type FromQuery/FromForm
// use as their activity.NewWebMiddleware type parameter. It carries no
// data itself — the ParamBinding declarations passed to NewWebMiddleware
// do the real work of declaring and binding Params on the Builder — it
// exists only so extraction via activity.WebWrapper[*bindingProbe] can tell
// a binding option's wrapper apart from an ordinary HandlerFunc middleware
// wrapper, so binding options never contribute a no-op entry to a Web
// Activity's request-time middleware chain.
type bindingProbe struct{}

// HandlerFunc is a Web Activity's handler shape. It reads resolved Params
// from ctx with param.Read and returns the Response to send; it never
// receives a writable http.ResponseWriter (see Context, Response).
type HandlerFunc func(ctx Context) Response

// Descriptor is the immutable, defensively-copied snapshot of a Web
// Activity's effective configuration: the target-neutral Param and
// middleware declarations plus the Web-specific route. It is
// introspectable without executing the Activity's handler.
//
// Middleware never contains the synthetic entries web.FromQuery/FromForm
// declare internally to carry their data
// through the sealed activity.WebOption contract — only Activity's own
// caller-supplied middleware appears here, in declaration order.
type Descriptor struct {
	Name        string
	Description string
	Params      []activity.ParamBinding
	Middleware  []activity.Middleware
	// Method and Path are the Activity's derived route. An Activity with a
	// form binding uses POST; every other Activity uses GET. Path is the
	// Activity name prefixed by a slash.
	Method string
	Path   string
}

// Definition is one declared Web Activity produced by Activity. It is
// immutable and introspectable via Descriptor without ever executing its
// handler. Pass one or more Definitions to All to build a validated,
// servable Group.
type Definition struct {
	descriptor Descriptor
	handler    HandlerFunc
}

// Descriptor returns d's immutable effective descriptor.
func (d Definition) Descriptor() Descriptor { return d.descriptor }

// Name returns the Activity's declared name.
func (d Definition) Name() string { return d.descriptor.Name }

// activityConfig accumulates a Web Activity's construction-time state as
// Options are applied, in declaration order.
type activityConfig struct {
	builder  *activity.Builder
	wrappers []activity.Wrapper[HandlerFunc]
}

// Activity declares a Web Activity named name with handler and options,
// building on activity.New(name, "web") and Builder.ApplyWeb. It accepts
// only Web-capable options (Option, aliasing activity.WebOption) and always
// succeeds: construction never executes the handler. Its HTTP route is
// derived from the Activity name and bindings: the path is "/" + name, and
// the method is POST when at least one Param is bound from a form, otherwise
// GET. This package deliberately models HTML GUIs rather than general HTTP
// APIs, so callers cannot override either part of the route.
func Activity(name string, handler HandlerFunc, options ...Option) Definition {
	if handler == nil {
		panic(fmt.Sprintf("web: activity %q: handler must not be nil", name))
	}

	b := activity.New(name, "web")
	cfg := &activityConfig{builder: b}

	for _, opt := range options {
		b.ApplyWeb(opt)

		if w, ok := activity.WebWrapper[HandlerFunc](opt); ok {
			cfg.wrappers = append(cfg.wrappers, w)
		}
		// Binding options (bindingProbe) carry no data through their
		// wrapper at all — their ParamBinding args, already applied above
		// via b.ApplyWeb(opt), are the whole effect. Nothing to extract.
	}

	snap := b.Snapshot()
	method := http.MethodGet
	for _, pb := range snap.Params {
		if pb.Source == "form" {
			method = http.MethodPost
			break
		}
	}

	middleware := make([]activity.Middleware, 0, len(snap.Middleware))
	for _, m := range snap.Middleware {
		if strings.HasPrefix(m.Name, reservedPrefix) {
			continue
		}
		middleware = append(middleware, m)
	}

	return Definition{
		descriptor: Descriptor{
			Name:        snap.Name,
			Description: snap.Description,
			Params:      snap.Params,
			Middleware:  middleware,
			Method:      method,
			Path:        "/" + snap.Name,
		},
		handler: activity.Chain(handler, cfg.wrappers...),
	}
}
