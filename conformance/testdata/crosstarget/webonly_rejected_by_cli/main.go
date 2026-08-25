// Command webonly_rejected_by_cli is a negative compile fixture driven by
// conformance/crosstarget_fixture_test.go). It must fail to compile: it
// proves that a Web-only middleware — genuinely accepted by its own
// target's Activity constructor, web.Activity — is rejected by the Go
// compiler, not by a runtime registration check, when passed to
// cli.Activity.
package main

import (
	"context"
	"net/http"

	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/cli"
	"github.com/away2go/way2go/web"
)

func main() {
	csrf := activity.NewWebMiddleware[web.HandlerFunc]("csrf", func(next web.HandlerFunc) web.HandlerFunc {
		return next
	})

	// Sanity check: csrf is a genuinely valid web.Option, accepted by its own
	// target's Activity constructor. If this line ever fails to compile, the
	// fixture below is no longer isolating the cross-target rejection it
	// claims to.
	_ = web.Activity("secure", func(web.Context) web.Response {
		return web.Render(http.StatusOK, web.Text("ok"))
	}, csrf)

	// This is the line under test: a Web-only middleware Option must not
	// satisfy cli.Option, so passing it to cli.Activity must fail to
	// compile.
	_ = cli.Activity("secure-cli", func(context.Context) cli.Outcome {
		return cli.OK()
	}, csrf)
}
