package web_test

import (
	"testing"

	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/param"
	"github.com/away2go/way2go/web"
)

func noopHandler(web.Context) web.Response {
	panic("handler must never execute during introspection tests")
}

// TestActivityDescriptorWithoutHandlerExecution proves Definition.Descriptor
// is fully introspectable from declarative configuration alone: the
// handler passed to Activity panics if ever called, and this test never
// triggers that panic.
func TestActivityDescriptorWithoutHandlerExecution(t *testing.T) {
	q := param.String("q", param.Describe("query text"))
	limit := param.Int("limit", param.Default(10))

	def := web.Activity("search", noopHandler,
		activity.Describe("Searches for things."),
		web.Get("/search"),
		web.FromQuery(q, limit),
	)

	d := def.Descriptor()
	if d.Name != "search" {
		t.Fatalf("Name = %q, want %q", d.Name, "search")
	}
	if d.Description != "Searches for things." {
		t.Fatalf("Description = %q", d.Description)
	}
	if !d.HasRoute || d.Method != "GET" || d.Path != "/search" {
		t.Fatalf("route = %+v, want GET /search", d)
	}
	if len(d.Params) != 2 {
		t.Fatalf("Params = %+v, want 2 entries", d.Params)
	}
	for _, pb := range d.Params {
		if pb.Source != "query" {
			t.Fatalf("Params[%s].Source = %q, want %q", pb.Param.Name(), pb.Source, "query")
		}
	}
	if len(d.Middleware) != 0 {
		t.Fatalf("Middleware = %+v, want none (route/binding synthetic entries must be filtered)", d.Middleware)
	}
	if def.Name() != "search" {
		t.Fatalf("Definition.Name() = %q, want %q", def.Name(), "search")
	}
}

// TestActivityWithoutRouteIsConstructibleButNotRegistrable proves a route
// is not a core Activity invariant: construction always succeeds, and only
// registration (All) enforces "missing route" — see API contract
// #2.
func TestActivityWithoutRouteIsConstructibleButNotRegistrable(t *testing.T) {
	def := web.Activity("orphan", noopHandler)
	if def.Descriptor().HasRoute {
		t.Fatalf("HasRoute = true, want false for an Activity with no route option")
	}

	if _, err := web.All(def); err == nil {
		t.Fatalf("All: expected an error for a missing route")
	}
}

func TestActivityPanicsOnNilHandler(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Activity: expected panic for a nil handler")
		}
	}()
	web.Activity("nil-handler", nil)
}

// TestActivityMultipleRoutesIsRegistrationError proves that declaring more
// than one route on a single Activity is caught, and surfaced by All, not
// silently overwritten.
func TestActivityMultipleRoutesIsRegistrationError(t *testing.T) {
	def := web.Activity("dual-route", noopHandler,
		web.Get("/a"),
		web.Post("/b"),
	)
	if _, err := web.All(def); err == nil {
		t.Fatalf("All: expected an error for an Activity with more than one route")
	}
}
