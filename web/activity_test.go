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
		web.FromQuery(q, limit),
	)

	d := def.Descriptor()
	if d.Name != "search" {
		t.Fatalf("Name = %q, want %q", d.Name, "search")
	}
	if d.Description != "Searches for things." {
		t.Fatalf("Description = %q", d.Description)
	}
	if d.Method != "GET" || d.Path != "/search" {
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
		t.Fatalf("Middleware = %+v, want none (binding synthetic entries must be filtered)", d.Middleware)
	}
	if def.Name() != "search" {
		t.Fatalf("Definition.Name() = %q, want %q", def.Name(), "search")
	}
}

func TestActivityWithoutBindingsDerivesGETRoute(t *testing.T) {
	def := web.Activity("orphan", noopHandler)
	d := def.Descriptor()
	if d.Method != "GET" || d.Path != "/orphan" {
		t.Fatalf("route = %s %s, want GET /orphan", d.Method, d.Path)
	}
	if _, err := web.All(def); err != nil {
		t.Fatalf("All: %v", err)
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

func TestActivityWithFormDerivesPOSTRoute(t *testing.T) {
	name := param.String("name")
	d := web.Activity("create", noopHandler, web.FromForm(name)).Descriptor()
	if d.Method != "POST" || d.Path != "/create" {
		t.Fatalf("route = %s %s, want POST /create", d.Method, d.Path)
	}
}
