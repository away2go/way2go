package activity_test

import (
	"testing"

	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/param"
)

func TestNewPanicsOnEmptyNameOrTarget(t *testing.T) {
	cases := []struct {
		name, target string
	}{
		{"", "web"},
		{"  ", "web"},
		{"search", ""},
		{"search", "  "},
	}
	for _, c := range cases {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("New(%q, %q): expected panic", c.name, c.target)
				}
			}()
			activity.New(c.name, c.target)
		}()
	}
}

func TestSnapshotRecordsNameDescriptionTarget(t *testing.T) {
	b := activity.New("search", "web")
	b.ApplyWeb(activity.Describe("Searches for things."))

	d := b.Snapshot()
	if d.Name != "search" {
		t.Fatalf("Name = %q, want %q", d.Name, "search")
	}
	if d.Description != "Searches for things." {
		t.Fatalf("Description = %q", d.Description)
	}
	if d.Target != "web" {
		t.Fatalf("Target = %q, want %q", d.Target, "web")
	}
}

func TestSnapshotWithoutHandlerExecution(t *testing.T) {
	// This test's whole point is that a Descriptor is fully introspectable
	// from declarative configuration alone: there is no handler anywhere in
	// this test, and Snapshot must not need one.
	q := param.String("q", param.Describe("query text"))
	b := activity.New("search", "web")
	if err := b.DeclareParam(q, "query"); err != nil {
		t.Fatalf("DeclareParam: %v", err)
	}
	b.DeclareMiddleware(activity.Middleware{Name: "auth"})

	d := b.Snapshot()
	if len(d.Params) != 1 || d.Params[0].Param.Name() != "q" || d.Params[0].Source != "query" {
		t.Fatalf("Params = %+v, want one binding for q/query", d.Params)
	}
	if len(d.Middleware) != 1 || d.Middleware[0].Name != "auth" {
		t.Fatalf("Middleware = %+v, want one auth entry", d.Middleware)
	}
}

func TestSnapshotIsDefensiveCopy(t *testing.T) {
	q := param.String("q")
	b := activity.New("search", "web")
	if err := b.DeclareParam(q, "query"); err != nil {
		t.Fatalf("DeclareParam: %v", err)
	}
	b.DeclareMiddleware(activity.Middleware{Name: "auth"})

	d := b.Snapshot()
	d.Params[0] = activity.ParamBinding{}
	d.Middleware[0] = activity.Middleware{Name: "mutated"}

	d2 := b.Snapshot()
	if d2.Params[0].Param.Name() != "q" {
		t.Fatalf("mutating a returned Descriptor's Params leaked back into the Builder")
	}
	if d2.Middleware[0].Name != "auth" {
		t.Fatalf("mutating a returned Descriptor's Middleware leaked back into the Builder")
	}
}

func TestMiddlewareOrderIsPreserved(t *testing.T) {
	b := activity.New("search", "web")
	b.DeclareMiddleware(activity.Middleware{Name: "auth"})
	b.DeclareMiddleware(activity.Middleware{Name: "audit"})

	d := b.Snapshot()
	if len(d.Middleware) != 2 || d.Middleware[0].Name != "auth" || d.Middleware[1].Name != "audit" {
		t.Fatalf("Middleware = %+v, want [auth audit] in declaration order", d.Middleware)
	}
}

func TestDeclareParamDedupsSameIdentitySameSource(t *testing.T) {
	q := param.String("q")
	b := activity.New("search", "web")
	if err := b.DeclareParam(q, "query"); err != nil {
		t.Fatalf("first DeclareParam: %v", err)
	}
	if err := b.DeclareParam(q, "query"); err != nil {
		t.Fatalf("re-declaring the same param/source should be deduplicated, got error: %v", err)
	}

	d := b.Snapshot()
	if len(d.Params) != 1 {
		t.Fatalf("Params = %+v, want exactly one entry after dedup", d.Params)
	}
}

func TestDeclareParamRejectsConflictingSourceForSameIdentity(t *testing.T) {
	q := param.String("q")
	b := activity.New("search", "web")
	if err := b.DeclareParam(q, "query"); err != nil {
		t.Fatalf("first DeclareParam: %v", err)
	}
	if err := b.DeclareParam(q, "form"); err == nil {
		t.Fatalf("expected an error binding the same param identity to a second source")
	}
}

func TestDeclareParamRejectsDistinctIdentitiesWithSameExternalName(t *testing.T) {
	a := param.String("q")
	c := param.String("q") // distinct identity, same name
	b := activity.New("search", "web")
	if err := b.DeclareParam(a, "query"); err != nil {
		t.Fatalf("first DeclareParam: %v", err)
	}
	if err := b.DeclareParam(c, "query"); err == nil {
		t.Fatalf("expected an error: two distinct param identities share the external name %q", "q")
	}
}

func TestProgrammerErrorContractIsStructural(t *testing.T) {
	// activity.ProgrammerError must be satisfiable by a type in another
	// package (here, this test package) without importing activity's
	// concrete error types — this is exactly what param.UndeclaredReadError
	// relies on to avoid a param->activity import.
	var _ activity.ProgrammerError = (*fakeProgrammerError)(nil)
}

type fakeProgrammerError struct{}

func (*fakeProgrammerError) Error() string          { return "fake" }
func (*fakeProgrammerError) Way2GoProgrammerError() {}
