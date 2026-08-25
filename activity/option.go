package activity

import (
	"fmt"

	"github.com/away2go/way2go/param"
)

// WebOption is satisfied by every Option value that is valid to pass to a
// Web Activity constructor (see web.Activity). Its single method,
// applyWeb, is unexported and declared only in this package, which makes
// WebOption a closed contract: the only values that satisfy it are the ones
// produced by this package's own constructors — Describe, NewWebMiddleware
// and NewMiddleware. No caller-defined func or third-party type can conform
// to it, however similar its shape.
//
// web.Option is a direct alias for WebOption. This package has no
// dependency on web: the alias is how web exposes the contract under its
// own name without this package needing to know web exists.
type WebOption interface {
	applyWeb(*Builder)
}

// CLIOption is the CLI-side counterpart of WebOption: the closed contract
// satisfied by Describe, NewCLIMiddleware and NewMiddleware, and aliased as
// cli.Option.
type CLIOption interface {
	applyCLI(*Builder)
}

// Portable is satisfied by every Option that is valid on both a Web and a
// CLI Activity: activity.Describe, and any middleware built with
// NewMiddleware. Because Portable embeds both WebOption and CLIOption, a
// Portable value's static type already carries both sealed methods, so the
// very same expression — e.g. activity.Describe("...") — compiles directly
// as an argument to a Web Activity constructor (...web.Option) and to a CLI
// Activity constructor (...cli.Option), with no conversion at the call
// site.
//
// A middleware that intentionally supports only one target returns a bare
// WebOption or CLIOption instead (see NewWebMiddleware, NewCLIMiddleware):
// its static type then lacks the other target's method entirely, so passing
// it to the other target's constructor is a compile error, not a runtime
// registration failure.
type Portable interface {
	WebOption
	CLIOption
}

// Describe sets the Activity's optional human-readable description. It is
// the only target-neutral core Option in v1: its return type, Portable,
// compiles on both a Web and a CLI Activity.
func Describe(text string) Portable { return descOption{text: text} }

type descOption struct{ text string }

func (d descOption) applyWeb(b *Builder) { b.description = d.text }
func (d descOption) applyCLI(b *Builder) { b.description = d.text }

// NewCLIBinding builds a CLI-only Option that declares each of ds as a Param
// bound to source (e.g. "option", "arg") on whichever CLI Builder it is
// applied to. Unlike NewCLIMiddleware, it contributes no Middleware
// descriptor entry and carries no execution wrapper: it exists for CLI
// target-binding options (see cli.FromOptions and cli.FromArgs) that
// only need to attach ParamBindings, the same way NewWebMiddleware/
// NewCLIMiddleware/NewMiddleware are the only way outside this package to
// attach a Middleware descriptor entry — CLIOption's applyCLI method is
// unexported and sealed to this package (see the WebOption/CLIOption doc
// comments), so cli cannot define its own CLIOption-satisfying type and
// needs this constructor for the non-middleware, binding-only case.
//
// Each ds entry goes through the same DeclareParam identity, dedup and
// conflict rules as a directly bound Param or a middleware-contributed one:
// re-declaring the same identity with the same source is a no-op, and a
// conflicting redeclaration panics — the same failure class as an invalid
// Option applied through ApplyCLI at Activity construction time (see
// declareMiddleware in middleware.go), since Options apply with no error
// return available to the caller.
func NewCLIBinding(source string, ds ...param.AnyDescriptor) CLIOption {
	return cliBinding{source: source, params: ds}
}

type cliBinding struct {
	source string
	params []param.AnyDescriptor
}

func (b cliBinding) applyCLI(builder *Builder) {
	for _, d := range b.params {
		if err := builder.DeclareParam(d, b.source); err != nil {
			panic(fmt.Sprintf("activity: %v", err))
		}
	}
}
