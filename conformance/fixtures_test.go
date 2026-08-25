// Package conformance_test is a black-box integration test package. It proves
// the shared declarative Activity model
// and the compile-time Web/CLI target capability contract hold end to end,
// using nothing but the public github.com/away2go/way2go/{activity,param,
// web,cli,output} API surface — no internal package of activity, param, web
// or cli is imported anywhere in this directory (API contract).
//
// It deliberately introduces no shared handler, context, response or
// execution-engine abstraction of its own: newPaginationMiddleware below
// carries two entirely independent execution wrappers, one written against
// web.HandlerFunc and one against cli.HandlerFunc, wired together only by
// activity.NewMiddleware's generic Wrapper[H] plumbing.
package conformance_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/cli"
	"github.com/away2go/way2go/param"
	"github.com/away2go/way2go/web"
)

// newPaginationMiddleware builds the ONE test middleware constructor used,
// unchanged, as both a web.Option and a cli.Option throughout this package
// (API contract). Applied to a Web Activity it contributes limit
// as a query binding; applied to a CLI Activity it contributes the very same
// limit identity as an option binding. Each target gets its own independent
// execution wrapper — there is no shared handler type between them, only the
// shared param.Descriptor[int] identity and the shared Middleware{Name:
// "pagination"} descriptor entry. trace may be nil when a test only cares
// about Param resolution, not observing middleware execution order.
func newPaginationMiddleware(limit param.Descriptor[int], trace *[]string) activity.Portable {
	return activity.NewMiddleware[web.HandlerFunc, cli.HandlerFunc](
		"pagination",
		activity.WebSpec[web.HandlerFunc]{
			Wrap: func(next web.HandlerFunc) web.HandlerFunc {
				return func(ctx web.Context) web.Response {
					if trace != nil {
						*trace = append(*trace, "web:pagination")
					}
					return next(ctx)
				}
			},
			Params: []activity.ParamBinding{{Param: limit, Source: "query"}},
		},
		activity.CLISpec[cli.HandlerFunc]{
			Wrap: func(next cli.HandlerFunc) cli.HandlerFunc {
				return func(ctx context.Context) cli.Outcome {
					if trace != nil {
						*trace = append(*trace, "cli:pagination")
					}
					return next(ctx)
				}
			},
			Params: []activity.ParamBinding{{Param: limit, Source: "option"}},
		},
	)
}

// noopWebHandler and noopCLIHandler are handler stubs for fixtures that only
// need a valid, non-nil handler and do not care about its behaviour (e.g.
// introspection-only tests and the descriptor-identity comparison).
func noopWebHandler(web.Context) web.Response { return web.Render(http.StatusOK, web.Text("ok")) }

func noopCLIHandler(context.Context) cli.Outcome { return cli.OK() }

// findBinding locates the ParamBinding for name within bindings, failing the
// test immediately if it is not present.
func findBinding(t *testing.T, bindings []activity.ParamBinding, name string) activity.ParamBinding {
	t.Helper()
	for _, pb := range bindings {
		if pb.Param.Name() == name {
			return pb
		}
	}
	t.Fatalf("no binding named %q found in %+v", name, bindings)
	return activity.ParamBinding{}
}
