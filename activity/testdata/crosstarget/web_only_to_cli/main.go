// Command web_only_to_cli is a negative compile fixture (driven by
// activity/crosstarget_fixture_test.go). It must fail to compile: it proves
// the symmetric rejection to cli_only_to_web — a Web-only middleware Option
// is rejected by the compiler when passed where a CLI-capable Option is
// required.
package main

import (
	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/cli"
	"github.com/away2go/way2go/web"
)

type handler func(int) int

func acceptCLI(opts ...cli.Option) {}

func main() {
	csrfWebOnly := activity.NewWebMiddleware[handler]("csrf", func(next handler) handler { return next })

	// Sanity check: csrfWebOnly is a genuinely valid web.Option.
	var _ web.Option = csrfWebOnly

	// This is the line under test: a Web-only middleware Option must not
	// satisfy cli.Option, so this call must fail to compile.
	acceptCLI(csrfWebOnly)
}
