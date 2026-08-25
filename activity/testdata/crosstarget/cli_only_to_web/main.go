// Command cli_only_to_web is a negative compile fixture (driven by
// activity/crosstarget_fixture_test.go). It must fail to compile: it proves
// that a CLI-only middleware Option is rejected by the compiler — not
// merely at runtime registration — when passed where a Web-capable Option
// is required.
package main

import (
	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/cli"
	"github.com/away2go/way2go/web"
)

type handler func(int) int

func acceptWeb(opts ...web.Option) {}

func main() {
	auditCLIOnly := activity.NewCLIMiddleware[handler]("audit", func(next handler) handler { return next })

	// Sanity check: auditCLIOnly is a genuinely valid cli.Option. If this
	// line ever fails to compile, the fixture below is no longer isolating
	// the cross-target rejection it claims to.
	var _ cli.Option = auditCLIOnly

	// This is the line under test: a CLI-only middleware Option must not
	// satisfy web.Option, so this call must fail to compile.
	acceptWeb(auditCLIOnly)
}
