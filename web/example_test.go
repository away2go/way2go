package web_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/param"
	"github.com/away2go/way2go/web"
)

// exampleQuery is the search text. It has no param.Default, so it is
// required by default: a request against Example_search's route without a
// "q" query parameter would fail with an ordinary HTTP 400 input error
// before search's handler ever ran — see param.MissingValueError.
var exampleQuery = param.String("q", param.Describe("Search query text."))

// search is exampleSearch's target-specific handler. It reads the
// resolved, validated exampleQuery value with param.Read and returns only
// a web.Response — it never receives a writable http.ResponseWriter.
func search(ctx web.Context) web.Response {
	q := param.Read(ctx.Context(), exampleQuery)
	return web.Render(http.StatusOK, web.Text(fmt.Sprintf("results for %q", q)))
}

// exampleSearch declares a Web Activity: a non-empty name, the handler
// above, an optional description and a query binding (web.FromQuery). Its
// route is derived as GET /search. Everything after the handler is an
// ordinary Option — there is no mandatory Use(...) or Params(...) wrapper
// at the declaration site.
var exampleSearch = web.Activity(
	"search",
	search,
	activity.Describe("Searches for things."),
	web.FromQuery(exampleQuery),
)

// Example_search builds a validated web.Group from one Web Activity with
// web.All and serves one request through it exactly as a real net/http
// server would: exampleQuery is resolved, parsed and validated from the
// request's URL query string before search's handler ever runs.
//
// See cli/example_test.go's Example_search for the same conceptual Query
// Param bound to a CLI option instead, with its own independent handler,
// context type and result kind — there is no shared execution engine
// between the two.
func Example_search() {
	group, err := web.All(exampleSearch)
	if err != nil {
		panic(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search?q=go", nil)
	group.ServeHTTP(rec, req)

	fmt.Println(rec.Code, rec.Body.String())
	// Output: 200 results for "go"
}
