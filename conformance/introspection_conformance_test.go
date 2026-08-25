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

// TestIntrospectionNeverExecutesHandlerOrMiddleware proves that building
// every fixture Activity's effective
// Descriptor — repeatedly, across both targets — never runs its handler or
// its middleware. handlerCalls/middlewareCalls must stay at zero throughout,
// even though Descriptor() is called multiple times on each Definition.
func TestIntrospectionNeverExecutesHandlerOrMiddleware(t *testing.T) {
	var handlerCalls, middlewareCalls int

	limit := param.Int("limit", param.Default(3))
	counting := activity.NewMiddleware[web.HandlerFunc, cli.HandlerFunc](
		"counting",
		activity.WebSpec[web.HandlerFunc]{
			Wrap: func(next web.HandlerFunc) web.HandlerFunc {
				return func(ctx web.Context) web.Response {
					middlewareCalls++
					return next(ctx)
				}
			},
			Params: []activity.ParamBinding{{Param: limit, Source: "query"}},
		},
		activity.CLISpec[cli.HandlerFunc]{
			Wrap: func(next cli.HandlerFunc) cli.HandlerFunc {
				return func(ctx context.Context) cli.Outcome {
					middlewareCalls++
					return next(ctx)
				}
			},
			Params: []activity.ParamBinding{{Param: limit, Source: "option"}},
		},
	)

	webDef := web.Activity("count", func(web.Context) web.Response {
		handlerCalls++
		return web.Render(http.StatusOK, web.Text("ok"))
	}, counting)

	cliDef := cli.Activity("count", func(context.Context) cli.Outcome {
		handlerCalls++
		return cli.OK()
	}, counting)

	// A target-specific fixture too, so introspection covers more than just
	// the portable case.
	webOnly := web.Activity("web-only", func(web.Context) web.Response {
		handlerCalls++
		return web.Render(http.StatusOK, web.Text("ok"))
	})
	cliOnly := cli.Activity("cli-only", func(context.Context) cli.Outcome {
		handlerCalls++
		return cli.OK()
	})

	for i := 0; i < 3; i++ {
		_ = webDef.Descriptor()
		_ = cliDef.Descriptor()
		_ = webOnly.Descriptor()
		_ = cliOnly.Descriptor()
	}

	if handlerCalls != 0 {
		t.Fatalf("handlerCalls = %d, want 0 (introspection must never execute a handler)", handlerCalls)
	}
	if middlewareCalls != 0 {
		t.Fatalf("middlewareCalls = %d, want 0 (introspection must never execute middleware)", middlewareCalls)
	}
}
