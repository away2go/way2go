package activity_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/cli"
	"github.com/away2go/way2go/param"
	"github.com/away2go/way2go/web"
)

// handler stands in for a real target's handler type in these tests. It
// exercises Wrapper/Chain and the middleware constructors generically,
// exactly as the real web and cli handler types do.
type handler func(int) int

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestChainMakesFirstDeclaredMiddlewareOutermost(t *testing.T) {
	var trace []string
	record := func(name string) activity.Wrapper[handler] {
		return func(next handler) handler {
			return func(v int) int {
				trace = append(trace, "enter:"+name)
				r := next(v)
				trace = append(trace, "exit:"+name)
				return r
			}
		}
	}
	base := handler(func(v int) int { trace = append(trace, "handler"); return v })

	chained := activity.Chain(base, record("auth"), record("audit"))
	chained(1)

	want := []string{"enter:auth", "enter:audit", "handler", "exit:audit", "exit:auth"}
	if !equalStrings(trace, want) {
		t.Fatalf("trace = %v, want %v", trace, want)
	}
}

func TestNewWebMiddlewareAppliesOnlyToWebAndCarriesWrapper(t *testing.T) {
	limit := param.Int("limit", param.Default(10))
	mw := activity.NewWebMiddleware[handler]("pagination", func(next handler) handler { return next },
		activity.ParamBinding{Param: limit, Source: "query"})

	// Compile-time proof: mw is a valid web.Option. (Its non-satisfaction of
	// cli.Option is proved out-of-band by the negative compile fixtures in
	// testdata/crosstarget, since a failing assignment can't appear in a
	// test that must itself compile and run.)
	var _ web.Option = mw

	b := activity.New("list", "web")
	b.ApplyWeb(mw)
	d := b.Snapshot()

	if len(d.Middleware) != 1 || d.Middleware[0].Name != "pagination" {
		t.Fatalf("Middleware = %+v, want one pagination entry", d.Middleware)
	}
	if len(d.Params) != 1 || d.Params[0].Param.Name() != "limit" || d.Params[0].Source != "query" {
		t.Fatalf("Params = %+v, want one limit/query binding", d.Params)
	}

	if _, ok := activity.WebWrapper[handler](mw); !ok {
		t.Fatalf("WebWrapper: expected ok=true for a NewWebMiddleware value")
	}
}

func TestNewCLIMiddlewareAppliesOnlyToCLIAndCarriesWrapper(t *testing.T) {
	limit := param.Int("limit", param.Default(10))
	mw := activity.NewCLIMiddleware[handler]("pagination", func(next handler) handler { return next },
		activity.ParamBinding{Param: limit, Source: "flag"})

	var _ cli.Option = mw

	b := activity.New("list", "cli")
	b.ApplyCLI(mw)
	d := b.Snapshot()

	if len(d.Middleware) != 1 || d.Middleware[0].Name != "pagination" {
		t.Fatalf("Middleware = %+v, want one pagination entry", d.Middleware)
	}
	if len(d.Params) != 1 || d.Params[0].Param.Name() != "limit" || d.Params[0].Source != "flag" {
		t.Fatalf("Params = %+v, want one limit/flag binding", d.Params)
	}

	if _, ok := activity.CLIWrapper[handler](mw); !ok {
		t.Fatalf("CLIWrapper: expected ok=true for a NewCLIMiddleware value")
	}
}

// TestPortableMiddlewareContributesDistinctParamsAndWrappersPerTarget is the
// API contract proof: one concrete middleware option value
// contributes a single logical Middleware descriptor and Param identity
// while supplying distinct Web/CLI bindings (source) and wrappers.
func TestPortableMiddlewareContributesDistinctParamsAndWrappersPerTarget(t *testing.T) {
	limit := param.Int("limit", param.Default(10))

	mw := activity.NewMiddleware[handler, handler](
		"pagination",
		activity.WebSpec[handler]{
			Wrap:   func(next handler) handler { return next },
			Params: []activity.ParamBinding{{Param: limit, Source: "query"}},
		},
		activity.CLISpec[handler]{
			Wrap:   func(next handler) handler { return next },
			Params: []activity.ParamBinding{{Param: limit, Source: "flag"}},
		},
	)

	// Compile-time proof: the same mw value satisfies both web.Option and
	// cli.Option statically, with no conversion.
	var _ web.Option = mw
	var _ cli.Option = mw

	wb := activity.New("list", "web")
	wb.ApplyWeb(mw)
	wd := wb.Snapshot()
	if len(wd.Middleware) != 1 || wd.Middleware[0].Name != "pagination" {
		t.Fatalf("web Middleware = %+v, want one pagination entry", wd.Middleware)
	}
	if len(wd.Params) != 1 || wd.Params[0].Source != "query" {
		t.Fatalf("web Params = %+v, want source %q", wd.Params, "query")
	}

	cb := activity.New("list", "cli")
	cb.ApplyCLI(mw)
	cd := cb.Snapshot()
	if len(cd.Middleware) != 1 || cd.Middleware[0].Name != "pagination" {
		t.Fatalf("cli Middleware = %+v, want one pagination entry", cd.Middleware)
	}
	if len(cd.Params) != 1 || cd.Params[0].Source != "flag" {
		t.Fatalf("cli Params = %+v, want source %q", cd.Params, "flag")
	}

	if cd.Params[0].Param != wd.Params[0].Param {
		t.Fatalf("web and cli bindings must share the same Param identity (same underlying descriptor)")
	}

	if _, ok := activity.WebWrapper[handler](mw); !ok {
		t.Fatalf("WebWrapper: expected ok=true for a NewMiddleware value")
	}
	if _, ok := activity.CLIWrapper[handler](mw); !ok {
		t.Fatalf("CLIWrapper: expected ok=true for a NewMiddleware value")
	}
}

// TestMiddlewareParamDedupsSameIdentitySameSourceAcrossReapplication is the
// API contract proof (dedup half): reapplying the same
// middleware option to the same Builder does not duplicate the Param it
// declares, exactly like a directly-bound Param (see
// TestDeclareParamDedupsSameIdentitySameSource in activity_test.go).
func TestMiddlewareParamDedupsSameIdentitySameSourceAcrossReapplication(t *testing.T) {
	limit := param.Int("limit", param.Default(10))
	mw := activity.NewWebMiddleware[handler]("pagination", func(next handler) handler { return next },
		activity.ParamBinding{Param: limit, Source: "query"})

	b := activity.New("list", "web")
	b.ApplyWeb(mw)
	b.ApplyWeb(mw)

	d := b.Snapshot()
	if len(d.Params) != 1 {
		t.Fatalf("Params = %+v, want exactly one entry after dedup", d.Params)
	}
}

// TestMiddlewareParamConflictsWithDirectBindingPanics is the
// API contract proof (conflict half): a middleware-declared Param
// goes through the exact same DeclareParam conflict rule as a directly
// bound Param. Because Options apply at Activity construction time (there is
// no error return available to the caller then), the conflict surfaces as a
// panic rather than a returned error — see declareMiddleware's doc comment
// in middleware.go.
func TestMiddlewareParamConflictsWithDirectBindingPanics(t *testing.T) {
	other := param.Int("limit", param.Default(5)) // distinct identity, same external name
	limit := param.Int("limit", param.Default(10))
	mw := activity.NewWebMiddleware[handler]("pagination", func(next handler) handler { return next },
		activity.ParamBinding{Param: limit, Source: "query"})

	b := activity.New("list", "web")
	if err := b.DeclareParam(other, "query"); err != nil {
		t.Fatalf("DeclareParam(other): %v", err)
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected ApplyWeb to panic on a conflicting middleware-declared Param")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "pagination") || !strings.Contains(msg, "limit") {
			t.Fatalf("panic message = %q, want it to mention the middleware and param names", msg)
		}
	}()
	b.ApplyWeb(mw)
}

// TestDeclaredMiddlewareOrderMatchesChainWrappingOrder is the
// API contract proof end to end: the declaration order recorded
// in Descriptor.Middleware is the same order WebWrapper/Chain use to make
// the first declared middleware outermost at execution time.
func TestDeclaredMiddlewareOrderMatchesChainWrappingOrder(t *testing.T) {
	var trace []string
	wrap := func(name string) activity.Wrapper[handler] {
		return func(next handler) handler {
			return func(v int) int {
				trace = append(trace, name)
				return next(v)
			}
		}
	}
	auth := activity.NewWebMiddleware[handler]("auth", wrap("auth"))
	audit := activity.NewWebMiddleware[handler]("audit", wrap("audit"))

	b := activity.New("list", "web")
	b.ApplyWeb(auth, audit)
	d := b.Snapshot()
	if len(d.Middleware) != 2 || d.Middleware[0].Name != "auth" || d.Middleware[1].Name != "audit" {
		t.Fatalf("Middleware = %+v, want [auth audit] in declaration order", d.Middleware)
	}

	authWrap, _ := activity.WebWrapper[handler](auth)
	auditWrap, _ := activity.WebWrapper[handler](audit)
	base := handler(func(v int) int { trace = append(trace, "handler"); return v })
	chained := activity.Chain(base, authWrap, auditWrap)

	chained(0)
	want := []string{"auth", "audit", "handler"}
	if !equalStrings(trace, want) {
		t.Fatalf("execution order = %v, want %v (declaration order == outer-to-inner)", trace, want)
	}
}
