package web_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/away2go/way2go/web"
)

func TestRenderJSONSuccess(t *testing.T) {
	resp := web.Render(http.StatusOK, web.JSON(map[string]any{"name": "Ada", "age": 37}))
	rec := httptest.NewRecorder()
	writeResponse(resp, rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", ct)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (body %q)", err, rec.Body.String())
	}
	if got["name"] != "Ada" {
		t.Fatalf("got %+v", got)
	}
}

func TestRenderJSONFailureMapsTo500(t *testing.T) {
	// A Go func value cannot be JSON-marshalled; json.Marshal returns an
	// UnsupportedTypeError, exercising the render-failure path.
	resp := web.Render(http.StatusOK, web.JSON(func() {}))
	rec := httptest.NewRecorder()
	writeResponse(resp, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body %q)", rec.Code, rec.Body.String())
	}
}

func TestRenderTextSuccess(t *testing.T) {
	resp := web.Render(http.StatusCreated, web.Text("hello"))
	rec := httptest.NewRecorder()
	writeResponse(resp, rec)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q", ct)
	}
	if rec.Body.String() != "hello" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

// failingRenderer proves Response.writeTo maps any Renderer failure to 500,
// not just JSON's own encoding errors.
type failingRenderer struct{}

func (failingRenderer) ContentType() string    { return "text/plain" }
func (failingRenderer) Render(io.Writer) error { return errors.New("boom") }

func TestRenderGenericFailureMapsTo500(t *testing.T) {
	resp := web.Render(http.StatusOK, failingRenderer{})
	rec := httptest.NewRecorder()
	writeResponse(resp, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestRedirectSetsStatusAndLocation(t *testing.T) {
	resp := web.Redirect(http.StatusFound, "/elsewhere")
	rec := httptest.NewRecorder()
	writeResponse(resp, rec)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/elsewhere" {
		t.Fatalf("Location = %q, want /elsewhere", loc)
	}
}

// writeResponse drives a Response through a real registered Activity so
// the test exercises Response.writeTo the same way production code does,
// without needing an exported hook onto the unexported method.
func writeResponse(resp web.Response, rec *httptest.ResponseRecorder) {
	group, err := web.All(web.Activity("respond", func(web.Context) web.Response {
		return resp
	}))
	if err != nil {
		panic(err)
	}
	group.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/respond", nil))
}
