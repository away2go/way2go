package way2go_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"

	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/cli"
	"github.com/away2go/way2go/file"
	"github.com/away2go/way2go/output"
	"github.com/away2go/way2go/param"
	"github.com/away2go/way2go/prompt"
	"github.com/away2go/way2go/validation"
	"github.com/away2go/way2go/web"
)

// exampleLimit is a Param automatically contributed by withPagination
// below, not declared directly at either Activity's call site. Middleware-
// contributed Params go through exactly the same identity, dedup and
// conflict rules as a Param bound directly (see
// activity.Builder.DeclareParam).
var exampleLimit = param.Int("limit", param.Default(10), param.Describe("Maximum number of results."))

// withPagination is one target-spanning middleware constructor built with
// activity.NewMiddleware: applied to a Web Activity it contributes
// exampleLimit as a query binding and a Web-specific execution wrapper;
// applied to a CLI Activity it contributes the very same exampleLimit
// identity as an option binding and an entirely independent CLI-specific
// wrapper. The returned activity.Portable value is passed directly at the
// Activity call site below, like any other Option — there is no
// Use(...) wrapper.
//
// This is still not a shared execution engine: activity.NewMiddleware only
// pairs one WebSpec wrapper with
// one CLISpec wrapper under a single declared Middleware identity. Each
// target keeps calling its own concrete handler type (web.HandlerFunc,
// cli.HandlerFunc), and each wrapper only ever runs under its own target's
// execution boundary.
//
// Compile-time target safety: a middleware built instead with
// activity.NewWebMiddleware or activity.NewCLIMiddleware returns a bare
// web.Option or cli.Option whose static type carries only one target's
// sealed method. Passing a Web-only middleware to cli.Activity, or a
// CLI-only middleware to web.Activity, is therefore rejected by the Go
// compiler at the call site — a "does not implement" / "missing method"
// diagnostic — never by a runtime registration failure. That rejection
// cannot be shown as a runnable Example (a failing compile can't be a
// passing test), so it is instead proven by `go build`-driven negative
// compile fixtures kept out of this module's own build under
// activity/testdata/crosstarget and conformance/testdata/crosstarget, each
// exercised by its own package's crosstarget_fixture_test.go.
func withPagination() activity.Portable {
	return activity.NewMiddleware[web.HandlerFunc, cli.HandlerFunc](
		"pagination",
		activity.WebSpec[web.HandlerFunc]{
			Wrap: func(next web.HandlerFunc) web.HandlerFunc {
				return func(ctx web.Context) web.Response { return next(ctx) }
			},
			Params: []activity.ParamBinding{{Param: exampleLimit, Source: "query"}},
		},
		activity.CLISpec[cli.HandlerFunc]{
			Wrap: func(next cli.HandlerFunc) cli.HandlerFunc {
				return func(ctx context.Context) cli.Outcome { return next(ctx) }
			},
			Params: []activity.ParamBinding{{Param: exampleLimit, Source: "option"}},
		},
	)
}

var exampleList = web.Activity(
	"list",
	func(ctx web.Context) web.Response {
		n := param.Read(ctx.Context(), exampleLimit)
		return web.Render(http.StatusOK, web.Text(fmt.Sprintf("limit=%d", n)))
	},
	withPagination(),
)

var exampleListCLI = cli.Activity(
	"list",
	func(ctx context.Context) cli.Outcome {
		n := param.Read(ctx, exampleLimit)
		output.Printf(ctx, "limit=%d\n", n)
		return cli.OK()
	},
	withPagination(),
)

// Example_middleware demonstrates withPagination used, unchanged, at both
// a Web and a CLI Activity's call site: one middleware declaration, two
// independent target executions.
func Example_middleware() {
	webGroup, err := web.All(exampleList)
	if err != nil {
		panic(err)
	}
	rec := httptest.NewRecorder()
	webGroup.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/list?limit=5", nil))
	fmt.Println("web:", rec.Code, rec.Body.String())

	cliApp := cli.All(exampleListCLI)
	var out, errOut bytes.Buffer
	code := cliApp.Execute(context.Background(), []string{"list", "--limit", "5"}, nil, &out, &errOut)
	fmt.Print("cli: ")
	fmt.Print(out.String())
	fmt.Println("exit code:", code)

	// Output:
	// web: 200 limit=5
	// cli: limit=5
	// exit code: 0
}

// Example_introspection builds one Web Activity's effective
// activity.Descriptor and inspects it directly — Name, Description, every
// Param binding with its target source and required/optional status —
// without ever calling web.All or serving a request, so the Activity's
// handler never runs. Descriptor is an immutable, defensively-copied
// snapshot: repeated calls are safe and always return the same data.
func Example_introspection() {
	query := param.String("q", param.Describe("Search query text."))

	def := web.Activity(
		"search",
		func(web.Context) web.Response { panic("handler must never run for introspection") },
		activity.Describe("Searches for things."),
		web.FromQuery(query),
	)

	d := def.Descriptor()
	fmt.Println("name:", d.Name)
	fmt.Println("description:", d.Description)
	for _, pb := range d.Params {
		fmt.Printf("param: %s (%s) required=%v source=%s\n",
			pb.Param.Name(), pb.Param.Kind(), !pb.Param.HasDefault(), pb.Source)
	}
	// Output:
	// name: search
	// description: Searches for things.
	// param: q (string) required=true source=query
}

// Example_undeclaredRead demonstrates param.Read's programmer-error panic:
// reading a Param the effective Activity never declared. This is a Way2Go
// programmer error (activity.ProgrammerError), never user input — Read
// never infers a source, returns a zero value, or reports the condition as
// invalid input.
//
// Application handlers and middleware must NOT recover this panic.
// Recovery is each target's own execution boundary's job (web's request
// handling, cli's command dispatch): it is the only code that knows how to
// map a recovered Way2Go programmer error to that target's own failure
// representation (Web: HTTP 500; CLI: exit code 1) without mislabelling it
// as ordinary input. The recover below exists solely so this documentation
// example can finish and print its result instead of crashing the test
// binary — it is not a pattern to copy into handler code.
func Example_undeclaredRead() {
	declared := param.String("declared")
	undeclared := param.String("undeclared")

	values, err := param.Prepare([]param.AnyDescriptor{declared}, map[param.AnyDescriptor]param.RawValue{
		declared: {Value: "ok", Present: true},
	})
	if err != nil {
		panic(err)
	}
	ctx := param.NewContext(context.Background(), values)

	func() {
		defer func() {
			r := recover()
			rerr, ok := r.(error)
			if !ok {
				panic(r)
			}
			var pe *param.UndeclaredReadError
			if !errors.As(rerr, &pe) {
				panic(r)
			}
			fmt.Println("panic:", pe)
		}()
		_ = param.Read(ctx, undeclared) // undeclared was never bound on any Activity
	}()
	// Output:
	// panic: param: read of undeclared param "undeclared"
}

// Example_promptAndFileParam demonstrates the deliberate split between a
// documented file-path Param and an imperative prompt. The same typed
// validator value is usable by param.Validate and prompt.Validate, but the
// prompted count is not a Param and is never added to command help.
func Example_promptAndFileParam() {
	positive := validation.Validator[int](func(n int) error {
		if n < 1 {
			return errors.New("must be positive")
		}
		return nil
	})
	outputPath := param.File("output", param.Validate(file.Extension(".txt")))
	limit := param.Int("limit", param.Default(1), param.Validate(positive))

	activity := cli.Activity("write", func(ctx context.Context) cli.Outcome {
		count, err := prompt.Read(ctx, "Count:", strconv.Atoi,
			prompt.Validate(positive),
			prompt.RetryInvalid[int](),
		)
		if err != nil {
			return cli.Error(err)
		}
		output.Printf(ctx, "path=%s limit=%d count=%d\n",
			param.Read(ctx, outputPath), param.Read(ctx, limit), count)
		return cli.OK()
	}, cli.FromOptions(outputPath, limit))

	app := cli.All(activity)
	var out, errOut bytes.Buffer
	code := app.Execute(context.Background(), []string{"write", "--output", "result.txt"},
		strings.NewReader("0\n2\n"), &out, &errOut)
	fmt.Print(errOut.String())
	fmt.Print(out.String())
	fmt.Println("exit:", code)

	// Output:
	// Count:
	// must be positive
	// Count:
	// path=result.txt limit=1 count=2
	// exit: 0
}
