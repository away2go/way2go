// Package cli implements Way2Go's Cobra-backed CLI Activity target: named
// Activities (Activity) with long-form option (FromOptions) and fixed positional
// argument (FromArgs) Param bindings, nested command groups (Group) rooted in
// an App (All), middleware execution, a fixed HandlerFunc/Outcome handler
// contract, a selective panic-recovery boundary and a deterministic exit
// code mapping.
//
// A cli.Activity's handler reads its declared Params from ctx with
// param.Read, writes through the context-bound output/input packages, and
// reports its result as an Outcome — OK() or NOK() — never an arbitrary
// error or exit code. App.Execute resolves and validates every declared
// Param (param.Prepare) before any middleware or handler runs, then maps the
// outcome to a process exit code: 0 for OK, 1 for NOK or a recovered Way2Go
// programmer error (activity.ProgrammerError), 2 for any flag, argument or
// Param input error. App.Run is the real-process entry point: it runs
// App.Execute against os.Args/os.Stdin/os.Stdout/os.Stderr under a context
// cancelled on SIGINT/SIGTERM, and terminates via os.Exit.
//
// Cobra and pflag are used internally for parsing, dispatch and standard
// help, but are an implementation detail: Node's methods are unexported so
// only this package can implement it (see group.go), and no Cobra or pflag
// type appears in any exported declaration here.
package cli

import "github.com/away2go/way2go/activity"

// Option is every value valid to pass as a CLI Activity's option: an
// activity.Describe(...) call, a CLI-only middleware built with
// activity.NewCLIMiddleware, or the CLI side of a portable middleware built
// with activity.NewMiddleware. It is a direct alias for activity.CLIOption
// — see web.Option and activity.WebOption for the closed-contract rationale
// this mirrors.
//
// A Web-only Option (web.Option / activity.WebOption) does not satisfy this
// type: passing one where cli.Option is expected is a compile error.
type Option = activity.CLIOption
