package conformance_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/away2go/way2go/cli"
	"github.com/away2go/way2go/param"
	"github.com/away2go/way2go/web"
)

// TestPortableMiddlewareResolvesSharedParamAcrossWebAndCLI is the
// API contract proof: the same newPaginationMiddleware value
// is passed, unchanged, to both a Web Activity constructor and a CLI
// Activity constructor. It contributes the same param.Descriptor[int]
// identity to both — bound to a real query string parameter on Web (via
// httptest) and a real "--limit" flag on CLI (via cli.App.Execute with
// injected buffers) — and both targets show equivalent default and
// validation behaviour: same default when absent, same value when supplied,
// and the same validation-error class (an ordinary input error, not a
// Way2Go programmer error) rejecting an invalid value.
func TestPortableMiddlewareResolvesSharedParamAcrossWebAndCLI(t *testing.T) {
	limit := param.Int("limit", param.Default(10), param.Validate(func(v int) error {
		if v < 0 {
			return fmt.Errorf("limit must be >= 0")
		}
		return nil
	}))
	mw := newPaginationMiddleware(limit, nil)

	var gotWeb int
	webGroup, err := web.All(web.Activity("list", func(ctx web.Context) web.Response {
		gotWeb = param.Read(ctx.Context(), limit)
		return web.Render(http.StatusOK, web.Text(strconv.Itoa(gotWeb)))
	}, web.Get("/list"), mw))
	if err != nil {
		t.Fatalf("web.All: %v", err)
	}

	var gotCLI int
	cliApp := cli.All(cli.Activity("list", func(ctx context.Context) cli.Outcome {
		gotCLI = param.Read(ctx, limit)
		return cli.OK()
	}, mw))

	t.Run("same default applies on both targets when absent", func(t *testing.T) {
		rec := httptest.NewRecorder()
		webGroup.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/list", nil))
		if rec.Code != http.StatusOK || rec.Body.String() != "10" {
			t.Fatalf("web: status=%d body=%q, want 200 \"10\"", rec.Code, rec.Body.String())
		}

		var out, errBuf bytes.Buffer
		if code := cliApp.Execute(context.Background(), []string{"list"}, nil, &out, &errBuf); code != 0 {
			t.Fatalf("cli: code = %d, want 0; stderr=%s", code, errBuf.String())
		}
		if gotCLI != 10 {
			t.Fatalf("cli: resolved = %d, want default 10", gotCLI)
		}
		if gotWeb != gotCLI {
			t.Fatalf("web resolved %d, cli resolved %d; want equal defaults", gotWeb, gotCLI)
		}
	})

	t.Run("an explicitly supplied value overrides the default on both targets", func(t *testing.T) {
		rec := httptest.NewRecorder()
		webGroup.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/list?limit=5", nil))
		if rec.Code != http.StatusOK || rec.Body.String() != "5" {
			t.Fatalf("web: status=%d body=%q, want 200 \"5\"", rec.Code, rec.Body.String())
		}

		var out, errBuf bytes.Buffer
		if code := cliApp.Execute(context.Background(), []string{"list", "--limit=5"}, nil, &out, &errBuf); code != 0 {
			t.Fatalf("cli: code = %d, want 0; stderr=%s", code, errBuf.String())
		}
		if gotCLI != 5 {
			t.Fatalf("cli: resolved = %d, want 5", gotCLI)
		}
	})

	t.Run("the same validator rejects an invalid value on both targets", func(t *testing.T) {
		rec := httptest.NewRecorder()
		webGroup.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/list?limit=-1", nil))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("web: status = %d, want 400 (ordinary input error); body=%q", rec.Code, rec.Body.String())
		}

		var out, errBuf bytes.Buffer
		code := cliApp.Execute(context.Background(), []string{"list", "--limit=-1"}, nil, &out, &errBuf)
		if code != 2 {
			t.Fatalf("cli: code = %d, want 2 (ordinary input error); stderr=%s", code, errBuf.String())
		}
	})
}
