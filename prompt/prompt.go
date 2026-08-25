// Package prompt implements Way2Go's imperative, generic helper for
// interactively prompting a user over the context-bound input/output sinks:
// Read[T] writes a prompt label, reads one line, parses it into a T, and
// optionally validates and retries on invalid entry.
//
// prompt is deliberately thin and standalone. It shares exactly one
// dependency with package param — package validation, for the ordered,
// first-error-wins Validator[T]/Apply contract both packages need to run
// "parse-then-validate" — and nothing else. The intended dependency graph is
// validation <- param and validation <- prompt, with no edge between param
// and prompt. This is a deliberate, previously-litigated scope boundary, not
// an oversight:
//
//   - prompt does not import param, and never will. It has no descriptors,
//     no defaults, no introspection, none of param's declarative machinery —
//     a prompt is just a parse-then-validate read, called inline.
//   - prompt exposes no descriptor or introspection concept whatsoever: no
//     Descriptor, no AnyDescriptor-equivalent, no way to enumerate declared
//     prompts, nothing registered anywhere. There is nothing to enumerate,
//     because nothing is declared ahead of time.
//   - prompt has no FromPrompt binding function and no integration point
//     with param.Values or any equivalent store. A prompt value lives only
//     in the local variable Read returns to its caller; it is never a Param
//     source for a CLI or web target.
//   - prompt has no description/help-text option and is not "documentable"
//     in any sense param descriptors are: there is no declarative surface to
//     document.
//   - Prompts are called imperatively, inline, in ordinary handler code —
//     potentially conditionally, potentially more than once in sequence
//     (input.ReadLine's context-bound buffering supports that directly).
//     There is no declarative registration step and no builder. This is the
//     entire point of the package: a small, direct wrapper around
//     input.ReadLine and output.Errorf/Errorln, not a second Param system.
//
// A future reader tempted to add any of the above "for consistency with
// param" or "in case it's useful later" should not: it was considered and
// rejected when this package was designed.
package prompt

import (
	"context"

	"github.com/away2go/way2go/input"
	"github.com/away2go/way2go/output"
	"github.com/away2go/way2go/validation"
)

// settings accumulates the Options applied to a single Read[T] call.
// Unlike param.settings, prompt's settings can stay genuinely generic: every
// one of prompt's options is naturally parameterised by T (there is no
// Describe-shaped option whose signature fails to mention T), so there is no
// reason to reproduce param.settings' type-erasure machinery here.
type settings[T any] struct {
	validators   []validation.Validator[T]
	retryInvalid bool
}

// Option configures a single Read[T] call, via Validate and RetryInvalid.
type Option[T any] func(*settings[T])

// InvalidInputError reports that a line was read successfully but could not
// be parsed or validated. It is distinct from input-source failures such as
// EOF, I/O errors and context cancellation, which Read returns unchanged.
// Callers such as cli can use errors.As to classify this as ordinary user
// input failure while errors.Is/errors.As still reach the parser or validator
// error through Unwrap.
type InvalidInputError struct {
	Err error
}

func (e *InvalidInputError) Error() string {
	if e == nil || e.Err == nil {
		return "invalid prompt input"
	}
	return e.Err.Error()
}

func (e *InvalidInputError) Unwrap() error { return e.Err }

// InputError marks InvalidInputError as ordinary invalid user input. It lets
// target packages classify the error through a small local interface instead
// of importing prompt, preserving prompt's independence from CLI targets.
func (e *InvalidInputError) InputError() bool { return true }

// Validate adds one or more validators, run in declaration order against a
// successfully parsed value; the first validator to return a non-nil error
// wins, via validation.Apply. Multiple Validate calls among a single Read
// call's options accumulate, in the order supplied — mirroring
// param.Validate's own accumulation behaviour, so a validator written for
// one package reads the same way in the other.
func Validate[T any](validators ...validation.Validator[T]) Option[T] {
	return func(s *settings[T]) {
		s.validators = append(s.validators, validators...)
	}
}

// RetryInvalid makes Read re-prompt on an invalid entry (a parse error or a
// validation failure) instead of returning it immediately. Without
// RetryInvalid, Read returns the first invalid-entry error it encounters,
// leaving the caller to decide how to fail. RetryInvalid never applies to
// input.ReadLine errors (EOF, other I/O failures, context cancellation) —
// those are unrecoverable read conditions, not invalid entries, and always
// return immediately regardless of this option.
func RetryInvalid[T any]() Option[T] {
	return func(s *settings[T]) {
		s.retryInvalid = true
	}
}

// Read writes label to ctx's bound stderr sink (via output.Errorln, never
// stdout — so a command's actual result output stays uncontaminated by
// interactive prompt chatter), reads one line via input.ReadLine(ctx),
// parses it with parse, and — if parsing succeeds — validates it against
// every validator accumulated via Validate, in order, using
// validation.Apply.
//
// label is written verbatim as one line: Read appends nothing beyond the
// trailing newline output.Errorln itself adds, so label is expected to
// already carry whatever punctuation the caller wants (e.g. "Enter your
// name:" rather than "Enter your name" with punctuation bolted on
// elsewhere).
//
// Read distinguishes two failure classes:
//
//   - An input.ReadLine error — io.EOF (source exhausted), any other I/O
//     error, or a context-cancellation error — is an unrecoverable read
//     condition. Read returns it immediately as (zero T, err), never
//     retrying, regardless of RetryInvalid.
//   - A parse error, or a validation error from a successfully parsed
//     value, is an invalid user entry. Without RetryInvalid, Read returns it
//     immediately as (zero T, err). With RetryInvalid, Read writes the error
//     to ctx's stderr sink and re-prompts (re-writing label and reading
//     another line), until either a valid entry arrives or an
//     input.ReadLine error ends the loop.
//
// Before writing label on each attempt (including the first), Read performs
// the same cheap, cooperative pre-read cancellation check input.ReadLine
// itself performs: if ctx is already done, Read returns (zero T, ctx.Err())
// without writing a prompt line at all. This is a deliberate
// belt-and-suspenders check, not a replacement for input.ReadLine's own — it
// exists so an already-cancelled context never produces a dangling prompt
// line on ctx's stderr sink for a read that was never going to happen; a
// context that becomes cancelled after this check but before or during the
// ReadLine call is still caught by ReadLine's own check.
func Read[T any](
	ctx context.Context,
	label string,
	parse func(string) (T, error),
	options ...Option[T],
) (T, error) {
	var s settings[T]
	for _, opt := range options {
		if opt == nil {
			continue
		}
		opt(&s)
	}

	var zero T
	for {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		output.Errorln(ctx, label)

		line, err := input.ReadLine(ctx)
		if err != nil {
			return zero, err
		}

		value, err := parse(line)
		if err == nil {
			err = validation.Apply(value, s.validators...)
		}
		if err != nil {
			if s.retryInvalid {
				output.Errorln(ctx, err)
				continue
			}
			return zero, &InvalidInputError{Err: err}
		}

		return value, nil
	}
}
