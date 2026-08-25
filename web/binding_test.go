package web_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/away2go/way2go/param"
	"github.com/away2go/way2go/web"
)

func echoHandler(field func(web.Context) string) web.HandlerFunc {
	return func(ctx web.Context) web.Response {
		return web.Render(http.StatusOK, web.Text(field(ctx)))
	}
}

func TestFromQueryResolvesPresenceAbsenceAndEmptySemantics(t *testing.T) {
	q := param.String("q")
	group, err := web.All(web.Activity("search", echoHandler(func(ctx web.Context) string {
		return param.Read(ctx.Context(), q)
	}), web.FromQuery(q)))
	if err != nil {
		t.Fatalf("All: %v", err)
	}

	cases := []struct {
		name       string
		url        string
		wantStatus int
		wantBody   string
	}{
		{"present", "/search?q=hello", http.StatusOK, "hello"},
		{"present-empty", "/search?q=", http.StatusOK, ""},
		{"absent", "/search", http.StatusBadRequest, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, c.url, nil)
			group.ServeHTTP(rec, req)
			if rec.Code != c.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, c.wantStatus, rec.Body.String())
			}
			if c.wantStatus == http.StatusOK && rec.Body.String() != c.wantBody {
				t.Fatalf("body = %q, want %q", rec.Body.String(), c.wantBody)
			}
		})
	}
}

func TestFromFormResolvesPresenceAbsenceAndEmptySemantics(t *testing.T) {
	name := param.String("name")
	group, err := web.All(web.Activity("create", echoHandler(func(ctx web.Context) string {
		return param.Read(ctx.Context(), name)
	}), web.FromForm(name)))
	if err != nil {
		t.Fatalf("All: %v", err)
	}

	cases := []struct {
		name       string
		body       string
		wantStatus int
		wantBody   string
	}{
		{"present", "name=Ada", http.StatusOK, "Ada"},
		{"present-empty", "name=", http.StatusOK, ""},
		{"absent", "", http.StatusBadRequest, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/create", strings.NewReader(c.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			group.ServeHTTP(rec, req)
			if rec.Code != c.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, c.wantStatus, rec.Body.String())
			}
			if c.wantStatus == http.StatusOK && rec.Body.String() != c.wantBody {
				t.Fatalf("body = %q, want %q", rec.Body.String(), c.wantBody)
			}
		})
	}
}

// TestFromQueryVariadicDeclaresMultipleParamsAtOnce proves one FromQuery
// call both declares and binds several Params together.
func TestFromQueryVariadicDeclaresMultipleParamsAtOnce(t *testing.T) {
	q := param.String("q")
	limit := param.Int("limit", param.Default(10))
	verbose := param.Bool("verbose", param.Default(false))

	def := web.Activity("search", noopHandler, web.FromQuery(q, limit, verbose))
	d := def.Descriptor()
	if len(d.Params) != 3 {
		t.Fatalf("Params = %+v, want 3 entries from one variadic FromQuery call", d.Params)
	}
	for _, pb := range d.Params {
		if pb.Source != "query" {
			t.Fatalf("Params[%s].Source = %q, want query", pb.Param.Name(), pb.Source)
		}
	}
}

// TestDefaultsAndValidatorsApplyThroughBinding proves a bound Param's
// Default and Validate options take effect end to end through query
// resolution.
func TestDefaultsAndValidatorsApplyThroughBinding(t *testing.T) {
	limit := param.Int("limit", param.Default(10), param.Validate(func(v int) error {
		if v < 0 {
			return errTooSmall
		}
		return nil
	}))
	group, err := web.All(web.Activity("list", func(ctx web.Context) web.Response {
		v := param.Read(ctx.Context(), limit)
		return web.Render(http.StatusOK, web.Text(itoa(v)))
	}, web.FromQuery(limit)))
	if err != nil {
		t.Fatalf("All: %v", err)
	}

	t.Run("default applies when absent", func(t *testing.T) {
		rec := httptest.NewRecorder()
		group.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/list", nil))
		if rec.Code != http.StatusOK || rec.Body.String() != "10" {
			t.Fatalf("status=%d body=%q, want 200 \"10\"", rec.Code, rec.Body.String())
		}
	})

	t.Run("supplied value overrides default", func(t *testing.T) {
		rec := httptest.NewRecorder()
		group.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/list?limit=5", nil))
		if rec.Code != http.StatusOK || rec.Body.String() != "5" {
			t.Fatalf("status=%d body=%q, want 200 \"5\"", rec.Code, rec.Body.String())
		}
	})

	t.Run("validator rejects invalid supplied value with 400", func(t *testing.T) {
		rec := httptest.NewRecorder()
		group.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/list?limit=-1", nil))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body %q)", rec.Code, rec.Body.String())
		}
	})

	t.Run("unparsable value maps to 400", func(t *testing.T) {
		rec := httptest.NewRecorder()
		group.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/list?limit=abc", nil))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body %q)", rec.Code, rec.Body.String())
		}
	})
}

var errTooSmall = &validationErr{"limit must be >= 0"}

type validationErr struct{ msg string }

func (e *validationErr) Error() string { return e.msg }

func itoa(v int) string { return strconv.Itoa(v) }

// TestDuplicateParamNameConflictPanics proves two distinct descriptors
// sharing the same external name are always rejected, even when declared
// through two separate binding options.
func TestDuplicateParamNameConflictPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Activity: expected panic for a duplicate external param name")
		}
	}()
	a := param.String("q")
	b := param.String("q")
	web.Activity("search", noopHandler, web.FromQuery(a), web.FromQuery(b))
}

// TestConflictingSourceConflictPanics proves the same Param identity bound
// to two different sources is rejected.
func TestConflictingSourceConflictPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Activity: expected panic for a Param bound to conflicting sources")
		}
	}()
	q := param.String("q")
	web.Activity("search", noopHandler, web.FromQuery(q), web.FromForm(q))
}

// TestRedeclaringSameBindingIsDeduplicated proves declaring the identical
// (descriptor, source) pair twice is a no-op, not a conflict.
func TestRedeclaringSameBindingIsDeduplicated(t *testing.T) {
	q := param.String("q")
	def := web.Activity("search", noopHandler, web.FromQuery(q), web.FromQuery(q))
	if len(def.Descriptor().Params) != 1 {
		t.Fatalf("Params = %+v, want exactly one deduplicated entry", def.Descriptor().Params)
	}
}
