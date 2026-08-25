package web

import (
	"fmt"
	"strings"

	"github.com/away2go/way2go/activity"
)

// reservedPrefix marks the name of a synthetic activity.Middleware entry
// declared internally by this package's own route and binding options (see
// route.go, binding.go). No Way2Go middleware author can produce a name
// with this prefix through the public API (activity.NewWebMiddleware and
// activity.NewMiddleware take a plain human-authored string, and nothing in
// this package ever offers a caller control over this exact byte
// sequence), so it is a safe, collision-free marker: web.Activity strips
// any Middleware entry carrying it from the public Descriptor.Middleware
// view.
const reservedPrefix = "\x00web:"

// routeBox is the private probe type Get/Post/... route options use to
// carry their method and path out of the sealed activity.WebOption they
// are forced to be built as (see option.go). It is never part of any
// public signature.
type routeBox struct {
	method string
	path   string
}

// bindingProbe is the private, empty probe type FromQuery/FromPath/FromForm
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
// Middleware never contains the synthetic entries web.Get/Post/... or
// web.FromQuery/FromPath/FromForm declare internally to carry their data
// through the sealed activity.WebOption contract — only Activity's own
// caller-supplied middleware appears here, in declaration order.
type Descriptor struct {
	Name        string
	Description string
	Params      []activity.ParamBinding
	Middleware  []activity.Middleware
	// Method and Path are the Activity's registered route, set by a route
	// option such as Get. HasRoute reports whether a route option was
	// applied at all: a route is not a core Activity invariant (see
	// package doc and the design's Web target section), so a Definition
	// without one is fully constructible and introspectable — All is what
	// rejects it, at registration.
	Method   string
	Path     string
	HasRoute bool
}

// Definition is one declared Web Activity produced by Activity. It is
// immutable and introspectable via Descriptor without ever executing its
// handler. Pass one or more Definitions to All to build a validated,
// servable Group.
type Definition struct {
	descriptor Descriptor
	handler    HandlerFunc
	// err records the first construction-time conflict Activity's options
	// produced (an activity.Builder.DeclareParam conflict surfaced through
	// activity.NewWebMiddleware's panic path is caught nowhere here on
	// purpose — see route.go/binding.go — this field instead holds
	// web-package-native conflicts such as more than one route option).
	// All surfaces it as a registration failure so Activity itself never
	// needs to return an error.
	err error
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
	method   string
	path     string
	routeSet bool
	firstErr error
}

func (c *activityConfig) addErr(err error) {
	if err != nil && c.firstErr == nil {
		c.firstErr = err
	}
}

// Activity declares a Web Activity named name with handler and options,
// building on activity.New(name, "web") and Builder.ApplyWeb. It accepts
// only Web-capable options (Option, aliasing activity.WebOption) and always
// succeeds: construction never executes handler, and a missing or
// conflicting route fails registration (All), not construction, so a
// routeless Definition remains fully introspectable — e.g. in a test that
// only inspects Descriptor.
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
		if rw, ok := activity.WebWrapper[*routeBox](opt); ok {
			box := rw(&routeBox{})
			if cfg.routeSet {
				cfg.addErr(fmt.Errorf("web: activity %q: multiple routes declared (already %s %s, also got %s %s)",
					name, cfg.method, cfg.path, box.method, box.path))
			} else {
				cfg.method, cfg.path, cfg.routeSet = box.method, box.path, true
			}
		}
		// Binding options (bindingProbe) carry no data through their
		// wrapper at all — their ParamBinding args, already applied above
		// via b.ApplyWeb(opt), are the whole effect. Nothing to extract.
	}

	snap := b.Snapshot()

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
			Method:      cfg.method,
			Path:        cfg.path,
			HasRoute:    cfg.routeSet,
		},
		handler: activity.Chain(handler, cfg.wrappers...),
		err:     cfg.firstErr,
	}
}
