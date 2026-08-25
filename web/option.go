// Package web implements Way2Go's net/http HTML GUI Activity target: derived
// GET/POST routes, query/form Param bindings, middleware execution, Way2Go-owned
// Context and Response types, a selective panic-recovery boundary and
// registration into a validated, dependency-free http.Handler Group.
package web

import "github.com/away2go/way2go/activity"

// Option is every value valid to pass as a Web Activity's option: an
// activity.Describe(...) call, a Web-only middleware built with
// activity.NewWebMiddleware, the Web side of a portable middleware built
// with activity.NewMiddleware, or one of this package's own binding
// (FromQuery/FromForm) options.
//
// Option is a direct alias for activity.WebOption — see that type's doc
// comment for why the contract is closed to Way2Go-owned values. Kept under
// this package's own name so web.Activity's signature reads web.Option
// rather than activity.WebOption, and so activity.Describe(...) — whose
// static type, activity.Portable, already carries WebOption's sealed method
// — assigns here with no conversion (see
// activity.TestDescribeIsUsableOnBothTargetContracts).
//
// Binding options (FromQuery, ...) are built on
// activity.NewWebMiddleware: it is the only exported way to mint a new
// value satisfying the sealed WebOption contract from outside package
// activity. They carry their route/binding data out through the same
// generic activity.WebWrapper[H] extraction mechanism real middleware uses
// to hand back its Wrapper — just instantiated at a probe type private to
// this package (see bindingProbe in activity.go) instead of HandlerFunc, so
// web.Activity can tell "this option is one of my own binding declarations"
// apart from "this option carries a request
// wrapper" without any unsafe trick. Because they still go through
// activity.NewWebMiddleware, they also declare a synthetic
// activity.Middleware entry; web.Activity filters those (identified by the
// reservedPrefix name they share) out of the public Descriptor.Middleware
// view, so they never appear as user-visible middleware.
//
// A CLI-only Option (cli.Option / activity.CLIOption) does not satisfy this
// type: passing one where web.Option is expected is a compile error.
type Option = activity.WebOption

// Describe sets the Web Activity's optional human-readable description. It
// is a thin, Web-flavoured re-export of activity.Describe so call sites
// that already import web do not also need activity for this one option;
// activity.Describe(...) itself remains equally valid here.
func Describe(text string) Option { return activity.Describe(text) }
