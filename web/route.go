package web

import (
	"net/http"

	"github.com/away2go/way2go/activity"
)

// route builds the Web-capable Option that records method and path on
// whichever Web Activity it is applied to. It is built on
// activity.NewWebMiddleware — the only exported way to mint a value
// satisfying the sealed activity.WebOption contract from outside package
// activity — instantiated at the private routeBox probe type instead of
// HandlerFunc, purely to carry method/path out through
// activity.WebWrapper[*routeBox] (see activity.go). The wrapper itself has
// no request-time behaviour: it only ever runs once, synchronously, when
// Activity extracts it during construction.
func route(method, path string) Option {
	wrap := func(next *routeBox) *routeBox {
		next.method, next.path = method, path
		return next
	}
	return activity.NewWebMiddleware[*routeBox](reservedPrefix+"route:"+method, wrap)
}

// Get declares the Activity's route as an HTTP GET to path. path must be
// non-empty; registration (All) additionally requires exactly one route
// per Activity and rejects duplicate method/path pairs and Go 1.22+
// net/http ServeMux "{name}" path placeholders that do not match a
// FromPath binding (or vice versa).
func Get(path string) Option { return route(http.MethodGet, path) }

// Post declares the Activity's route as an HTTP POST to path.
func Post(path string) Option { return route(http.MethodPost, path) }

// Put declares the Activity's route as an HTTP PUT to path.
func Put(path string) Option { return route(http.MethodPut, path) }

// Patch declares the Activity's route as an HTTP PATCH to path.
func Patch(path string) Option { return route(http.MethodPatch, path) }

// Delete declares the Activity's route as an HTTP DELETE to path.
func Delete(path string) Option { return route(http.MethodDelete, path) }
