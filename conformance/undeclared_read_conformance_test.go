package conformance_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/away2go/way2go/cli"
	"github.com/away2go/way2go/param"
	"github.com/away2go/way2go/web"
)

// TestUndeclaredReadProducesSameProgrammerErrorIdentityAcrossTargets is the
// API contract proof. Both the Web and the CLI handler below call
// param.Read with stray, a descriptor never declared on their own fixture
// Activity. Each handler captures the raw panic value with its own local
// recover-then-repanic (no shared execution abstraction — this is ordinary
// Go inside each target's own handler closure) before letting it continue
// into that target's own recovery boundary, so this test can assert on
// identity twice:
//
//   - the captured panic values, from within the handlers, both
//     errors.As into the exact same concrete type, *param.UndeclaredReadError,
//     with the same Name — the programmer-error identity is target-agnostic;
//   - the two targets nonetheless recover that identical error into their
//     own, genuinely distinct results: Web maps it to HTTP 500 (asserted via
//     httptest), CLI maps it to exit code 1 (asserted via App.Execute's
//     returned int).
func TestUndeclaredReadProducesSameProgrammerErrorIdentityAcrossTargets(t *testing.T) {
	stray := param.String("stray-value") // deliberately never declared on either fixture Activity

	var webPanic, cliPanic any

	webGroup, err := web.All(web.Activity("boom", func(ctx web.Context) (resp web.Response) {
		defer func() {
			if r := recover(); r != nil {
				webPanic = r
				panic(r) // let web's own recovery boundary still map this to HTTP 500
			}
		}()
		_ = param.Read(ctx.Context(), stray)
		return web.Render(http.StatusOK, web.Text("unreachable"))
	}))
	if err != nil {
		t.Fatalf("web.All: %v", err)
	}

	cliApp := cli.All(cli.Activity("boom", func(ctx context.Context) (outcome cli.Outcome) {
		defer func() {
			if r := recover(); r != nil {
				cliPanic = r
				panic(r) // let cli's own recovery boundary still map this to exit code 1
			}
		}()
		_ = param.Read(ctx, stray)
		return cli.OK()
	}))

	rec := httptest.NewRecorder()
	webGroup.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("web: status = %d, want 500 (recovered Way2Go programmer error)", rec.Code)
	}

	var out, errBuf bytes.Buffer
	code := cliApp.Execute(context.Background(), []string{"boom"}, nil, &out, &errBuf)
	if code != 1 {
		t.Fatalf("cli: exit code = %d, want 1 (recovered Way2Go programmer error); stderr=%s", code, errBuf.String())
	}

	webErr, ok := webPanic.(error)
	if !ok {
		t.Fatalf("web: panic value = %v (%T), want an error", webPanic, webPanic)
	}
	cliErr, ok := cliPanic.(error)
	if !ok {
		t.Fatalf("cli: panic value = %v (%T), want an error", cliPanic, cliPanic)
	}

	var webUndeclared *param.UndeclaredReadError
	if !errors.As(webErr, &webUndeclared) {
		t.Fatalf("web: panic = %v, want errors.As into *param.UndeclaredReadError", webErr)
	}
	var cliUndeclared *param.UndeclaredReadError
	if !errors.As(cliErr, &cliUndeclared) {
		t.Fatalf("cli: panic = %v, want errors.As into *param.UndeclaredReadError", cliErr)
	}

	if webUndeclared.Name != cliUndeclared.Name {
		t.Fatalf("Name mismatch between targets: web=%q cli=%q", webUndeclared.Name, cliUndeclared.Name)
	}
	if webUndeclared.Name != "stray-value" {
		t.Fatalf("Name = %q, want %q", webUndeclared.Name, "stray-value")
	}
}
