package activity

// ProgrammerError is the contract satisfied by Way2Go errors that represent
// a programmer mistake — such as reading a Param an Activity never declared
// (param.UndeclaredReadError) — rather than ordinary user input. Target
// execution boundaries (Web and CLI) establish a recovery
// boundary around Activity execution and recover a panic only when
// errors.As(recovered, &pe) succeeds against this interface; every other
// panic, including ordinary input errors such as param.MissingValueError,
// param.ParseError and param.ValidationError, is re-panicked unchanged and
// must never be mislabeled as a Param error.
//
// Concrete error types satisfy ProgrammerError structurally, by providing a
// Way2GoProgrammerError method, without importing this package — this keeps
// param (and any future package that needs to raise a programmer error) free
// of a dependency on activity.
type ProgrammerError interface {
	error
	Way2GoProgrammerError()
}
