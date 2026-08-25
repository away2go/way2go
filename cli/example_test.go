package cli_test

import (
	"bytes"
	"context"
	"fmt"

	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/cli"
	"github.com/away2go/way2go/output"
	"github.com/away2go/way2go/param"
)

// exampleQuery is the same conceptual search Param as
// web/example_test.go's exampleQuery — a required-by-default string — but
// a distinct param.Descriptor[string] identity bound here to a CLI option
// instead of a URL query parameter. Params have no cross-package identity
// in v1: only the concept, name and type match.
var exampleQuery = param.String("query", param.Describe("Search query text."))

// search is exampleSearch's target-specific handler. Unlike web's handler
// it writes through the context-bound output package instead of returning
// a body, and reports its result as a cli.Outcome instead of a
// web.Response — there is no shared handler signature between targets.
func search(ctx context.Context) cli.Outcome {
	q := param.Read(ctx, exampleQuery)
	output.Printf(ctx, "results for %q\n", q)
	return cli.OK()
}

// exampleSearch is the CLI counterpart of web/example_test.go's
// exampleSearch: same declarative shape (name, handler, then Options), a
// different target binding (cli.FromOptions instead of web.FromQuery).
var exampleSearch = cli.Activity(
	"search",
	search,
	activity.Describe("Searches for things."),
	cli.FromOptions(exampleQuery),
)

// Example_search runs the command tree built from one CLI Activity exactly
// as a real process invocation would, but through Execute's explicit
// args/output-sink parameters so the example stays deterministic: no
// os.Args, no real process stdout.
func Example_search() {
	app := cli.All(exampleSearch)

	var out, errOut bytes.Buffer
	code := app.Execute(context.Background(), []string{"search", "--query", "go"}, nil, &out, &errOut)

	fmt.Print(out.String())
	fmt.Println("exit code:", code)
	// Output:
	// results for "go"
	// exit code: 0
}
