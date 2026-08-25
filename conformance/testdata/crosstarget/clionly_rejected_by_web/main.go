// Command clionly_rejected_by_web is a negative compile fixture driven by
// conformance/crosstarget_fixture_test.go). It must fail to compile: the
// symmetric rejection to webonly_rejected_by_cli. A CLI-only middleware —
// genuinely accepted by its own target's Activity constructor, cli.Activity
// — is rejected by the Go compiler when passed to web.Activity.
package main

import (
	"context"
	"net/http"

	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/cli"
	"github.com/away2go/way2go/web"
)

func main() {
	audit := activity.NewCLIMiddleware[cli.HandlerFunc]("audit", func(next cli.HandlerFunc) cli.HandlerFunc {
		return next
	})

	// Sanity check: audit is a genuinely valid cli.Option, accepted by its
	// own target's Activity constructor.
	_ = cli.Activity("audited", func(context.Context) cli.Outcome {
		return cli.OK()
	}, audit)

	// This is the line under test: a CLI-only middleware Option must not
	// satisfy web.Option, so passing it to web.Activity must fail to
	// compile.
	_ = web.Activity("audited-web", func(web.Context) web.Response {
		return web.Render(http.StatusOK, web.Text("ok"))
	}, audit)
}
