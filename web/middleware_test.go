package web_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/param"
	"github.com/away2go/way2go/web"
)

func recordingMiddleware(trace *[]string, name string) web.Option {
	wrap := func(next web.HandlerFunc) web.HandlerFunc {
		return func(ctx web.Context) web.Response {
			*trace = append(*trace, "enter:"+name)
			resp := next(ctx)
			*trace = append(*trace, "exit:"+name)
			return resp
		}
	}
	return activity.NewWebMiddleware[web.HandlerFunc](name, wrap)
}

// TestMiddlewareRunsInDeclaredOrderWithFirstOutermost proves that the first
// declared middleware is the outermost
// wrapper (auth(audit(handler))).
func TestMiddlewareRunsInDeclaredOrderWithFirstOutermost(t *testing.T) {
	var trace []string
	group, err := web.All(web.Activity("order", func(web.Context) web.Response {
		trace = append(trace, "handler")
		return web.Render(http.StatusOK, web.Text("ok"))
	}, recordingMiddleware(&trace, "auth"), recordingMiddleware(&trace, "audit")))
	if err != nil {
		t.Fatalf("All: %v", err)
	}

	rec := httptest.NewRecorder()
	group.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/order", nil))

	want := []string{"enter:auth", "enter:audit", "handler", "exit:audit", "exit:auth"}
	if len(trace) != len(want) {
		t.Fatalf("trace = %v, want %v", trace, want)
	}
	for i := range want {
		if trace[i] != want[i] {
			t.Fatalf("trace = %v, want %v", trace, want)
		}
	}
}

// TestMiddlewareCanShortCircuitWithAResponse proves a middleware can return
// a Response directly without calling next, and the handler (and any
// inner middleware) never runs.
func TestMiddlewareCanShortCircuitWithAResponse(t *testing.T) {
	handlerRan := false
	shortCircuit := activity.NewWebMiddleware[web.HandlerFunc]("gate", func(next web.HandlerFunc) web.HandlerFunc {
		return func(ctx web.Context) web.Response {
			return web.Render(http.StatusForbidden, web.Text("denied"))
		}
	})

	group, err := web.All(web.Activity("gated", func(web.Context) web.Response {
		handlerRan = true
		return web.Render(http.StatusOK, web.Text("ok"))
	}, shortCircuit))
	if err != nil {
		t.Fatalf("All: %v", err)
	}

	rec := httptest.NewRecorder()
	group.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/gated", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if handlerRan {
		t.Fatalf("handler ran despite the outer middleware short-circuiting")
	}
}

// TestParamsAreResolvedBeforeMiddlewareRuns is the API contract
// pre-middleware-resolution proof: a middleware that reads a declared
// Param (including one falling back to its default) observes the fully
// prepared value with no panic, before the handler itself runs.
func TestParamsAreResolvedBeforeMiddlewareRuns(t *testing.T) {
	limit := param.Int("limit", param.Default(7))

	var sawInMiddleware int
	probe := activity.NewWebMiddleware[web.HandlerFunc]("probe", func(next web.HandlerFunc) web.HandlerFunc {
		return func(ctx web.Context) web.Response {
			// If Params were not yet resolved, Read would panic with an
			// UndeclaredReadError (no prepared Values reachable from
			// ctx.Context() at all) rather than returning this value.
			sawInMiddleware = param.Read(ctx.Context(), limit)
			return next(ctx)
		}
	})

	group, err := web.All(web.Activity("probe-activity", func(ctx web.Context) web.Response {
		return web.Render(http.StatusOK, web.Text("ok"))
	}, web.FromQuery(limit), probe))
	if err != nil {
		t.Fatalf("All: %v", err)
	}

	rec := httptest.NewRecorder()
	group.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/probe-activity", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if sawInMiddleware != 7 {
		t.Fatalf("middleware observed limit = %d, want the resolved default 7", sawInMiddleware)
	}
}

// TestMiddlewareContributedParamIsIntrospectableWithoutExecution proves a
// middleware's own declared Params show up on the effective Descriptor
// without ever running the middleware or the handler.
func TestMiddlewareContributedParamIsIntrospectableWithoutExecution(t *testing.T) {
	flag := param.Bool("beta", param.Default(false))
	mw := activity.NewWebMiddleware[web.HandlerFunc]("feature-flag",
		func(next web.HandlerFunc) web.HandlerFunc { return next },
		activity.ParamBinding{Param: flag, Source: "query"},
	)

	def := web.Activity("flagged", noopHandler, mw)
	d := def.Descriptor()

	if len(d.Middleware) != 1 || d.Middleware[0].Name != "feature-flag" {
		t.Fatalf("Middleware = %+v, want one feature-flag entry", d.Middleware)
	}
	found := false
	for _, pb := range d.Params {
		if pb.Param.Name() == "beta" && pb.Source == "query" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Params = %+v, want a beta/query binding contributed by the middleware", d.Params)
	}
}

// TestUndeclaredParamReadIsRecoveredAsHTTP500 is the API contract
// #8 proof: param.Read's UndeclaredReadError implements the Way2Go
// programmer-error contract, so the recovery boundary maps it to HTTP 500.
func TestUndeclaredParamReadIsRecoveredAsHTTP500(t *testing.T) {
	undeclared := param.String("never-declared")

	group, err := web.All(web.Activity("boom", func(ctx web.Context) web.Response {
		_ = param.Read(ctx.Context(), undeclared)
		return web.Render(http.StatusOK, web.Text("unreachable"))
	}))
	if err != nil {
		t.Fatalf("All: %v", err)
	}

	rec := httptest.NewRecorder()
	group.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body %q)", rec.Code, rec.Body.String())
	}
}

// TestUnrelatedPanicIsRepanicked is the other half of API contract
// #8: a panic that is not a Way2Go programmer error must propagate
// unchanged, not be swallowed or mislabeled as an input error.
func TestUnrelatedPanicIsRepanicked(t *testing.T) {
	boom := errors.New("unrelated failure")
	group, err := web.All(web.Activity("panics", func(web.Context) web.Response {
		panic(boom)
	}))
	if err != nil {
		t.Fatalf("All: %v", err)
	}

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		group.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/panics", nil))
	}()

	if recovered == nil {
		t.Fatalf("expected the unrelated panic to propagate out of ServeHTTP")
	}
	if recovered != any(boom) {
		t.Fatalf("recovered = %v, want the original panic value %v", recovered, boom)
	}
}

// TestNonErrorPanicIsRepanicked proves a panic value that is not even an
// error (so it can never satisfy the programmer-error contract) also
// propagates unchanged.
func TestNonErrorPanicIsRepanicked(t *testing.T) {
	group, err := web.All(web.Activity("string-panic", func(web.Context) web.Response {
		panic("not an error at all")
	}))
	if err != nil {
		t.Fatalf("All: %v", err)
	}

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		group.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/string-panic", nil))
	}()

	if recovered != "not an error at all" {
		t.Fatalf("recovered = %v, want the original string panic value", recovered)
	}
}
