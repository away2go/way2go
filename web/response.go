package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Renderer produces a Response body using only standard-library
// interfaces. See JSON and Text for the minimal renderers this package
// supplies; a caller may implement its own.
type Renderer interface {
	// ContentType is the value written as the response's Content-Type
	// header.
	ContentType() string
	// Render writes the body to w. A returned error fails the whole
	// Response with HTTP 500 (see Response.writeTo) — nothing is written
	// to the real http.ResponseWriter until Render has already succeeded
	// in full, so a failing Renderer never leaves a partially written body
	// on the wire.
	Render(w io.Writer) error
}

// Response is the only value a Web Activity handler returns. Construct one
// with Render or Redirect; there is no other way to produce one, and a
// handler never receives a writable http.ResponseWriter to bypass it.
type Response struct {
	status   int
	renderer Renderer
	location string
}

// Render is the sole body-producing Response primitive: it renders body's
// output with the given HTTP status.
func Render(status int, body Renderer) Response {
	return Response{status: status, renderer: body}
}

// Redirect returns a Response whose semantic result is status and the
// Location header, not a rendered body. It is a separate Response kind
// from Render because its status and Location are the whole result.
func Redirect(status int, location string) Response {
	return Response{status: status, location: location}
}

// writeTo commits r to w. It renders the body into an internal buffer
// first: if the Renderer returns an error, w receives HTTP 500 and never
// sees any of the failed render's output, and r's own requested status
// (which may have been 2xx) is never written.
func (r Response) writeTo(w http.ResponseWriter) {
	if r.renderer == nil {
		if r.location != "" {
			w.Header().Set("Location", r.location)
		}
		w.WriteHeader(r.status)
		return
	}

	var buf bytes.Buffer
	if err := r.renderer.Render(&buf); err != nil {
		http.Error(w, fmt.Sprintf("web: render error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", r.renderer.ContentType())
	w.WriteHeader(r.status)
	_, _ = w.Write(buf.Bytes())
}

// JSON returns a Renderer that encodes v as a JSON document.
func JSON(v any) Renderer { return jsonRenderer{v: v} }

type jsonRenderer struct{ v any }

func (j jsonRenderer) ContentType() string { return "application/json; charset=utf-8" }

func (j jsonRenderer) Render(w io.Writer) error { return json.NewEncoder(w).Encode(j.v) }

// Text returns a Renderer that writes s as a plain-text body.
func Text(s string) Renderer { return textRenderer(s) }

type textRenderer string

func (t textRenderer) ContentType() string { return "text/plain; charset=utf-8" }

func (t textRenderer) Render(w io.Writer) error {
	_, err := io.WriteString(w, string(t))
	return err
}
