package conformance_test

import (
	"testing"

	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/cli"
	"github.com/away2go/way2go/param"
	"github.com/away2go/way2go/web"
)

// TestMiddlewareDeclarationOrderIsIdenticalAcrossTargets is half of
// API contract: declaring the same two middleware — auth, then
// pagination — in the same order on a Web and a CLI Activity produces the
// same declaration order in both effective descriptors.
func TestMiddlewareDeclarationOrderIsIdenticalAcrossTargets(t *testing.T) {
	limit := param.Int("limit", param.Default(1))
	pagination := newPaginationMiddleware(limit, nil)
	auth := activity.NewMiddleware[web.HandlerFunc, cli.HandlerFunc](
		"auth",
		activity.WebSpec[web.HandlerFunc]{Wrap: func(next web.HandlerFunc) web.HandlerFunc { return next }},
		activity.CLISpec[cli.HandlerFunc]{Wrap: func(next cli.HandlerFunc) cli.HandlerFunc { return next }},
	)

	webDef := web.Activity("ordered", noopWebHandler, web.Get("/ordered"), auth, pagination)
	cliDef := cli.Activity("ordered", noopCLIHandler, auth, pagination)

	wantOrder := []string{"auth", "pagination"}
	checkOrder := func(t *testing.T, got []activity.Middleware) {
		t.Helper()
		if len(got) != len(wantOrder) {
			t.Fatalf("Middleware = %+v, want %v", got, wantOrder)
		}
		for i, name := range wantOrder {
			if got[i].Name != name {
				t.Fatalf("Middleware = %+v, want %v", got, wantOrder)
			}
		}
	}
	checkOrder(t, webDef.Descriptor().Middleware)
	checkOrder(t, cliDef.Descriptor().Middleware)
}

// TestPortableMiddlewareParamDedupAcrossTargets is the other half of
// API contract's positive case: declaring the same portable
// middleware value twice on the same Activity does not duplicate the Param
// it contributes, on either target.
func TestPortableMiddlewareParamDedupAcrossTargets(t *testing.T) {
	limit := param.Int("limit", param.Default(1))
	mw := newPaginationMiddleware(limit, nil)

	webDef := web.Activity("dedup", noopWebHandler, web.Get("/dedup"), mw, mw)
	cliDef := cli.Activity("dedup", noopCLIHandler, mw, mw)

	if n := len(webDef.Descriptor().Params); n != 1 {
		t.Fatalf("web Params = %+v, want exactly one deduplicated entry", webDef.Descriptor().Params)
	}
	if n := len(cliDef.Descriptor().Params); n != 1 {
		t.Fatalf("cli Params = %+v, want exactly one deduplicated entry", cliDef.Descriptor().Params)
	}
}

// TestConflictingParamSourceFailsConsistentlyOnBothTargets proves that
// binding the same Param identity the
// portable middleware already bound (query on Web, flag on CLI) to a
// second, different source is a genuine conflict, and it fails registration
// — deterministically, at Activity construction — the same way on both
// targets.
func TestConflictingParamSourceFailsConsistentlyOnBothTargets(t *testing.T) {
	limit := param.Int("limit", param.Default(1))
	mw := newPaginationMiddleware(limit, nil) // query (web) / flag (cli)

	t.Run("web: query binding then a conflicting path binding panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected a panic for a conflicting Param source on Web")
			}
		}()
		web.Activity("conflict", noopWebHandler, web.Get("/conflict/{limit}"), mw, web.FromPath(limit))
	})

	t.Run("cli: flag binding then a conflicting arg binding panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected a panic for a conflicting Param source on CLI")
			}
		}()
		cli.Activity("conflict", noopCLIHandler, mw, cli.FromArg(limit))
	})
}
