package conformance_test

import (
	"net/http"
	"testing"

	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/cli"
	"github.com/away2go/way2go/param"
	"github.com/away2go/way2go/web"
)

// TestCrossTargetDescriptorsShareConceptualIdentity proves that a Web and a
// CLI Activity built with the same Activity
// name, the same activity.Describe(...) text and the same portable
// middleware produce descriptors that share the same conceptual Name,
// Description, Param type/default/description and middleware identity,
// while their Target and the Param binding's Source genuinely differ.
//
// web.Definition.Descriptor() returns web.Descriptor, a distinct Go type
// from activity.Descriptor: it conveys "this is the Web side" structurally,
// through its own derived Method/Path route fields, rather than through a
// Target string field (see web.Descriptor's doc comment) — cli.Definition.
// Descriptor() returns activity.Descriptor directly, which does carry an
// explicit Target field. That structural difference plus CLI's explicit
// Target is where "same conceptual model, different target" genuinely
// shows up in the real, committed API.
func TestCrossTargetDescriptorsShareConceptualIdentity(t *testing.T) {
	limit := param.Int("limit", param.Default(10), param.Describe("max rows to return"))
	mw := newPaginationMiddleware(limit, nil)

	webDef := web.Activity("list", noopWebHandler, activity.Describe("Lists things."), mw)
	cliDef := cli.Activity("list", noopCLIHandler, activity.Describe("Lists things."), mw)

	wd := webDef.Descriptor()
	cd := cliDef.Descriptor()

	if wd.Name != cd.Name {
		t.Fatalf("Name mismatch: web=%q cli=%q", wd.Name, cd.Name)
	}
	if wd.Description != cd.Description {
		t.Fatalf("Description mismatch: web=%q cli=%q", wd.Description, cd.Description)
	}

	if cd.Target != "cli" {
		t.Fatalf("cli Target = %q, want %q", cd.Target, "cli")
	}
	if wd.Method != http.MethodGet || wd.Path != "/list" {
		t.Fatalf("web descriptor missing its Web-specific route: %+v", wd)
	}

	wp := findBinding(t, wd.Params, "limit")
	cp := findBinding(t, cd.Params, "limit")

	if wp.Param.Kind() != cp.Param.Kind() {
		t.Fatalf("Kind mismatch: web=%v cli=%v", wp.Param.Kind(), cp.Param.Kind())
	}
	if wp.Param.HasDefault() != cp.Param.HasDefault() || wp.Param.Default() != cp.Param.Default() {
		t.Fatalf("Default mismatch: web=(%v,%v) cli=(%v,%v)",
			wp.Param.HasDefault(), wp.Param.Default(), cp.Param.HasDefault(), cp.Param.Default())
	}
	if wp.Param.Description() != cp.Param.Description() {
		t.Fatalf("Param Description mismatch: web=%q cli=%q", wp.Param.Description(), cp.Param.Description())
	}

	// Source is where the two targets deliberately diverge.
	if wp.Source != "query" {
		t.Fatalf("web Source = %q, want %q", wp.Source, "query")
	}
	if cp.Source != "option" {
		t.Fatalf("cli Source = %q, want %q", cp.Source, "flag")
	}

	if len(wd.Middleware) != 1 || len(cd.Middleware) != 1 || wd.Middleware[0].Name != cd.Middleware[0].Name {
		t.Fatalf("middleware identity mismatch: web=%+v cli=%+v", wd.Middleware, cd.Middleware)
	}
	if wd.Middleware[0].Name != "pagination" {
		t.Fatalf("Middleware[0].Name = %q, want %q", wd.Middleware[0].Name, "pagination")
	}
}
