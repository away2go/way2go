package web

import (
	"strings"

	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/param"
)

// fromSource builds the Web-capable Option that both declares and binds
// each of descs to source on whichever Web Activity it is applied to, via
// activity.Builder.DeclareParam (reached through
// activity.NewWebMiddleware's own param-declaring path, applied when
// Activity calls Builder.ApplyWeb on the returned Option — see
// activity.go). descs' external name is always its own declared
// param.AnyDescriptor.Name; there is no rename in v1.
//
// Re-declaring the same Param identity with the same source is a
// deduplicated no-op. Re-declaring the same identity with a conflicting
// source, or declaring a distinct identity that shares another declared
// Param's external name, panics deterministically at Web Activity
// construction: activity.NewWebMiddleware's declareMiddleware helper
// surfaces Builder.DeclareParam's descriptive error this way, the same
// fail-fast contract activity.Describe/NewMiddleware's own conflicting
// declarations already use — see activity/middleware.go's declareMiddleware
// doc comment.
//
// The returned Option is instantiated at the private bindingProbe type so
// its no-op wrapper is never mistaken by Activity for a real request-time
// middleware wrapper (see bindingProbe's doc comment) and never appears in
// the Web Activity's request-time middleware chain.
func fromSource(source string, descs []param.AnyDescriptor) Option {
	names := make([]string, len(descs))
	bindings := make([]activity.ParamBinding, len(descs))
	for i, d := range descs {
		names[i] = d.Name()
		bindings[i] = activity.ParamBinding{Param: d, Source: source}
	}
	name := reservedPrefix + "bind:" + source + ":" + strings.Join(names, ",")
	wrap := func(next *bindingProbe) *bindingProbe { return next }
	return activity.NewWebMiddleware[*bindingProbe](name, wrap, bindings...)
}

// FromQuery declares and binds one or more Params to the request's URL
// query string. Absence and an explicitly supplied empty string are
// distinct: only absence falls back to a Param's default (see
// param.Prepare).
//
// A repeated query key (e.g. "?q=a&q=b") resolves to only its first value
// ("a"); every later occurrence is silently discarded. This is a v1
// limitation, not a validated choice — callers should not rely on
// multi-value query params.
func FromQuery(descs ...param.AnyDescriptor) Option { return fromSource("query", descs) }

// FromPath declares and binds one or more Params to named net/http
// ServeMux route placeholders ("{name}") in the Activity's route path.
// Registration (All) rejects a path placeholder without a matching
// FromPath binding, and a FromPath binding without a matching placeholder.
// A matched path segment is always present.
func FromPath(descs ...param.AnyDescriptor) Option { return fromSource("path", descs) }

// FromForm declares and binds one or more Params to the request's parsed
// form body (application/x-www-form-urlencoded or multipart/form-data).
//
// A repeated form field (e.g. "q=a&q=b" in the body) resolves to only its
// first value ("a"); every later occurrence is silently discarded. This is a
// v1 limitation, not a validated choice — callers should not rely on
// multi-value form fields.
func FromForm(descs ...param.AnyDescriptor) Option { return fromSource("form", descs) }
