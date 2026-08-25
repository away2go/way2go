package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/away2go/way2go/activity"
)

// programmerError wraps a recovered Way2Go programmer error (see
// activity.ProgrammerError) so App.Execute can tell it apart from an
// ordinary input error returned by Cobra's own parsing/argument-count
// validation or by param.Prepare. It is the sole error type App.Execute maps
// to exit code 1; every other non-nil error it sees maps to exit code 2.
type programmerError struct {
	err error
}

func (e *programmerError) Error() string { return e.err.Error() }
func (e *programmerError) Unwrap() error { return e.err }

// invoke runs handler with ctx inside a recovery boundary that recovers only
// a panic whose value implements activity.ProgrammerError (the same
// selective-recovery discipline used by every Way2Go target execution
// boundary): such a panic is turned into a *programmerError and returned as
// err rather than propagated. Every other panic — including an ordinary
// input error type such as *param.MissingValueError, which is never raised
// as a panic by this package's own code in the first place, and any
// unrelated application panic — is re-panicked unchanged, so it is never
// silently swallowed or mislabeled as a Param error.
func invoke(ctx context.Context, handler HandlerFunc) (outcome Outcome, err error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		if asErr, ok := r.(error); ok {
			var pe activity.ProgrammerError
			if errors.As(asErr, &pe) {
				err = &programmerError{err: asErr}
				return
			}
		}
		panic(r)
	}()
	outcome = handler(ctx)
	return outcome, nil
}

// inputError wraps an ordinary flag/argument/Param input error (a
// param.Prepare failure, or this package's own "too many arguments" check)
// so its message is easy to recognise as an input error in logs and tests.
// App.Execute does not need to type-switch on it specially: any error that
// is not a *programmerError already maps to exit code 2.
type inputError struct {
	err error
}

func (e *inputError) Error() string { return fmt.Sprintf("cli: %v", e.err) }
func (e *inputError) Unwrap() error { return e.err }

// inputFailure is the small marker interface interactive packages use to
// identify an ordinary invalid user entry. It deliberately lives here rather
// than importing prompt: prompt remains independent from CLI, and another
// interactive source can opt into the same fixed status-2 convention.
type inputFailure interface {
	InputError() bool
}

func isInputFailure(err error) bool {
	var failure inputFailure
	return errors.As(err, &failure) && failure.InputError()
}
