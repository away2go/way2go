# Way2Go

Way2Go is a Go module for declaring Web and CLI activities through a shared
conceptual model. Activities combine typed parameters, validation, middleware,
introspection, and target-specific execution.

Web and CLI retain their native handlers, contexts, bindings, and result types.

```sh
go get github.com/away2go/way2go@v0.1.0
```

## Quick start

A Web Activity binds typed parameters to an HTML GUI route derived from its
Activity name and bindings:

```go
var Query = param.String("q", param.Describe("Search query text."))

var Search = web.Activity(
	"search",
	search,
	activity.Describe("Searches for things."),
	web.FromQuery(Query),
)

func search(ctx web.Context) web.Response {
	q := param.Read(ctx.Context(), Query)
	return web.Render(http.StatusOK, web.Text("results for "+q))
}
```

The corresponding CLI Activity binds the same conceptual input to an option and
uses a CLI-specific handler and outcome:

```go
var Query = param.String("query", param.Describe("Search query text."))

var Search = cli.Activity(
	"search",
	search,
	activity.Describe("Searches for things."),
	cli.FromOptions(Query),
)

func search(ctx context.Context) cli.Outcome {
	q := param.Read(ctx, Query)
	output.Printf(ctx, "results for %s\n", q)
	return cli.OK()
}
```

Runnable examples are available for [Web](web/example_test.go),
[CLI](cli/example_test.go), [portable middleware](example_test.go), and
[introspection](example_test.go).

## Activity model

An Activity has a name, a target-specific handler, an optional description,
typed Params and their bindings, middleware, and target-specific registration
options. Options preserve declaration order.

Constructing an Activity does not execute its handler or middleware.
`Definition.Descriptor()` returns an immutable snapshot containing the name,
description, target, Param bindings, and middleware:

```go
def := web.Activity(
	"search",
	search,
	web.FromQuery(Query),
)

descriptor := def.Descriptor()
fmt.Println(descriptor.Name, len(descriptor.Params))
```

Middleware order is deterministic: the first declared middleware is the
outermost wrapper.

## Typed parameters

The `param` package provides typed String, Int, Bool, File, Directory,
InputFile, and OutputFile Params. Each Param can carry a description, default,
and validators:

```go
limit := param.Int(
	"limit",
	param.Describe("Maximum number of results."),
	param.Default(10),
	param.Validate(validation.Between(1, 100)),
)
```

Params without a default are required. Supplied values and defaults pass
through the same parsing and validation process. Before middleware or a
handler runs, the Activity prepares a complete validated Param set that is
read with `param.Read`.

Bindings map Params to target inputs:

```go
web.FromQuery(Query, Limit)
web.FromForm(Name, Email)

cli.FromOptions(Query, Limit)
cli.FromArgs(ID)
```

Reusable application-specific Param types can be defined with
`param.DefineType` and instantiated with `param.Of`.

## Validation, prompts, and files

`validation.Validator[T]` is shared by declarative Params and imperative
prompts. Built-in helpers cover ordered ranges, choices, non-empty strings,
and slices.

Handlers use `prompt.Read` for sequential or conditional input. The `input`
and `output` packages provide context-bound streams, while prompt labels and
validation feedback are written through the bound error output.

Semantic path Params validate common filesystem roles:

- `param.Directory` accepts an existing directory.
- `param.InputFile` accepts an existing regular file.
- `param.OutputFile` accepts a new target in an existing directory.

`file.ReadAll` reads complete files. `file.WriteNew` creates a file
exclusively and cleans up a partially written file when writing or closing
fails.

## Middleware

Way2Go supports Web middleware, CLI middleware, and portable middleware with
separate target implementations:

```go
webOnly := activity.NewWebMiddleware("request-log", webWrapper)
cliOnly := activity.NewCLIMiddleware("command-log", cliWrapper)
portable := activity.NewMiddleware("audit", webSpec, cliSpec)
```

Portable middleware has one logical identity and can contribute Params to
both targets. Its Web and CLI specs define independent wrappers and bindings.
Target-specific middleware and portable middleware are passed as Activity
options and appear in the Activity descriptor.

See [`Example_middleware`](example_test.go) for a complete runnable example.

## Web activities

Web Activities are deliberately HTML-GUI-oriented. Their path is `/<activity
name>`; they use GET when all Params come from the query string, and POST when
one or more Params come from a form:

```go
web.Activity("search", search, web.FromQuery(Query)) // GET /search
web.Activity("create-item", createItem, web.FromForm(Name)) // POST /create-item
```

`web.All` validates routes and bindings, then returns an `http.Handler`.
Handlers return a `web.Response` created with `web.Render` or `web.Redirect`.
The package includes JSON and text renderers.

## CLI activities

CLI Activities use long options and positional arguments and can be arranged in
nested command groups:

```go
app := cli.All(
	cli.Group("items",
		cli.Activity("list", listItems),
		cli.Activity("create", createItem),
	),
	cli.Activity("search", search, cli.FromOptions(Query)),
)

status := app.Execute(ctx, args, in, out, errOut)
```

Handlers write through the context-bound `output` package and return one of
the following outcomes:

| Result | Exit code |
|---|---:|
| `cli.OK()` | 0 |
| `cli.NOK()` | 1 |
| `cli.Error(err)` | 1 |
| Invalid input | 2 |

`App.Run` connects execution to process arguments and standard streams and
handles interrupt signals while input is blocked.

## Execution safety

Way2Go distinguishes user input failures from programmer errors. Missing,
unparseable, or invalid values are reported as typed Param errors. Reading a
Param that the effective Activity did not declare raises
`param.UndeclaredReadError`, which implements `activity.ProgrammerError`.

Web and CLI install target-specific execution boundaries that convert Way2Go
programmer errors into their native failure representation. Other panics
continue unchanged.

## Packages

| Package | Purpose |
|---|---|
| [`activity`](activity) | Activity descriptors and middleware contracts |
| [`param`](param) | Typed parameters, bindings, and prepared values |
| [`validation`](validation) | Reusable typed validators |
| [`web`](web) | HTTP activities, bindings, middleware, and responses |
| [`cli`](cli) | Command activities, groups, middleware, and outcomes |
| [`input`](input) | Context-bound CLI input |
| [`output`](output) | Context-bound CLI output |
| [`prompt`](prompt) | Interactive parsing, validation, and retry handling |
| [`file`](file) | File operations and path validation |

## Development

```sh
gofmt -l .
go vet ./...
go build ./...
go test ./...
go test -race ./...
```

CI runs the same checks on Go 1.25 for every push and pull request. Way2Go is
available under the [MIT License](LICENSE).
