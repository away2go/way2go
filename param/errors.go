package param

import "fmt"

// MissingValueError reports that a required Param (one without a Default)
// was not supplied. It is an ordinary input error: targets map it to their
// standard input-error representation (Web HTTP 400, CLI exit code 2), never
// to the Way2Go programmer-error recovery path.
type MissingValueError struct {
	Name string
}

func (e *MissingValueError) Error() string {
	return fmt.Sprintf("param %q: value is required", e.Name)
}

// ParseError records that a supplied raw value could not be parsed as the
// Param's declared type. It is retained as the wrapped cause of a
// ValidationError so callers can inspect the raw value and parse cause while
// targets consistently classify all rejected input as ValidationError.
type ParseError struct {
	Name string
	Raw  string
	Err  error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("param %q: %v", e.Name, e.Err)
}

func (e *ParseError) Unwrap() error { return e.Err }

// ValidationError reports that a supplied value was rejected by a declared
// validator. It is an ordinary input error.
type ValidationError struct {
	Name string
	Err  error
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("param %q: %v", e.Name, e.Err)
}

func (e *ValidationError) Unwrap() error { return e.Err }

// UndeclaredReadError reports that param.Read was called with a descriptor
// that is not part of the prepared value set reachable from ctx — i.e. a
// Param the effective Activity never declared, or a call made outside any
// prepared Activity context at all. This is a Way2Go programmer error, not
// user input: it can only happen because handler or middleware code reads a
// Param it never declared. It implements the Way2Go programmer-error
// contract (see activity.ProgrammerError) via Way2GoProgrammerError, so
// target recovery boundaries can recognise it through errors.As without
// param importing activity.
type UndeclaredReadError struct {
	Name string
}

func (e *UndeclaredReadError) Error() string {
	return fmt.Sprintf("param: read of undeclared param %q", e.Name)
}

// Way2GoProgrammerError marks UndeclaredReadError as a Way2Go programmer
// error. See activity.ProgrammerError for the contract this satisfies.
func (e *UndeclaredReadError) Way2GoProgrammerError() {}
