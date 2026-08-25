package cli

import "context"

// HandlerFunc is the fixed CLI Activity handler signature. A handler reads
// its declared Params from ctx with param.Read, writes through the
// context-bound output package (output.Println(ctx, ...), output.Errorln(ctx,
// ...)) and reports its result as an Outcome — never a writable stream, an
// arbitrary error or an arbitrary exit code.
type HandlerFunc func(ctx context.Context) Outcome

// Outcome is the fixed result a CLI handler returns. Its zero value is never
// produced by handler code directly: the only way to obtain an Outcome is
// OK, NOK or Error, so a CLI Activity cannot select an arbitrary exit code —
// exit code mapping is entirely owned by the cli execution boundary (see
// App.Execute).
type Outcome struct {
	ok  bool
	err error
}

// OK reports a successful outcome. cli.Execute maps it to exit code 0.
func OK() Outcome { return Outcome{ok: true} }

// NOK reports an ordinary unsuccessful outcome — a handler-detected failure
// that is not a Way2Go programmer error. cli.Execute maps it to exit code 1.
// Unlike Error, NOK carries no message: the framework prints nothing on its
// account.
func NOK() Outcome { return Outcome{ok: false} }

// Error reports an unsuccessful outcome carrying err, a handler-detected
// failure the handler wants reported to the user. cli.Execute maps it to
// the same exit code as NOK (1) — Error is NOK plus a message, not a
// separate exit code — and prints err exactly once to stderr, prefixed with
// the Activity's name, instead of the handler printing it itself. err's
// wrapping is preserved: whatever %v/%w-compatible chain err carries prints
// in full.
func Error(err error) Outcome {
	if err == nil {
		panic("cli: Error requires a non-nil error")
	}
	return Outcome{ok: false, err: err}
}

// error returns o's carried error, if any. It is unexported: App.Execute is
// the only caller that needs it, to print the single Activity-prefixed
// error message; handler code reports failure only through
// OK/NOK/Error, never by inspecting an Outcome it did not just construct.
func (o Outcome) error() error { return o.err }
