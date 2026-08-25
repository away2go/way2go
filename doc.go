// Package way2go is the module root of the transport-agnostic Way2Go
// application framework. It establishes one declarative Activity model for
// user interactions across Web and CLI while retaining target-specific
// handlers, contexts, responses and runtime implementations: same model, not
// same implementation.
//
// This module contains:
//
//   - activity: the target-neutral declarative core — Activity descriptors,
//     Option composition, the closed Web/CLI target-option contract
//     (WebOption, CLIOption, Portable), the generic middleware wrapping
//     primitive (Wrapper, Chain, NewMiddleware, NewWebMiddleware,
//     NewCLIMiddleware) and the Way2Go programmer-error contract
//     (ProgrammerError).
//   - param: typed Param descriptors (String, Int, Bool), defaults,
//     validators, preparation of raw external input into a validated typed
//     value set, and Read access to that prepared set from a handler.
//   - web: Web Activities — explicit net/http routes, query/path/form Param
//     bindings, direct middleware Options, a Way2Go-owned Context and
//     Response, a selective panic-recovery boundary, and registration into
//     a validated, dependency-free http.Handler Group.
//   - cli: CLI Activities — nested command Groups, flag/argument Param
//     bindings, direct middleware Options, Outcome-based results, recovery
//     and Cobra-backed execution; Cobra is an internal implementation
//     detail and never appears in this package's exported API.
//   - output: context-bound stdout/stderr sinks for CLI handlers, with no
//     package-global output state.
//
// See the repository README for a guided introduction, runnable Web and CLI
// examples, and the module's non-goals.
package way2go
