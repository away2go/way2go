// Package param implements the target-neutral, declarative Param core: typed
// parameter descriptors, options (default, description, validators),
// preparation of raw external input into a validated typed value set, and
// param.Read access to that prepared set from within a handler.
//
// A Param is a documented, user-controlled parameterisation option. It is
// required by default; param.Default is the sole v1 mechanism that makes it
// optional. Presence and value are distinct: an explicitly supplied empty
// string is present and is validated like any other value, never silently
// replaced by a default.
//
// This package has no knowledge of Web or CLI. Target packages resolve raw
// values from query strings, form fields, CLI options or positional arguments,
// then call Prepare with the
// resulting param.RawValue set.
package param

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/away2go/way2go/file"
	"github.com/away2go/way2go/validation"
)

// Kind identifies a Param descriptor's declared type.
type Kind int

const (
	// KindString identifies a param.String descriptor.
	KindString Kind = iota + 1
	// KindInt identifies a param.Int descriptor.
	KindInt
	// KindBool identifies a param.Bool descriptor.
	KindBool
	// KindFile identifies a param.File descriptor. Its external and Go value
	// are both strings; the distinct kind lets documentation render it as a
	// file path rather than a generic string.
	KindFile
	// KindDirectory identifies an existing directory path.
	KindDirectory
	// KindInputFile identifies an existing regular file path.
	KindInputFile
	// KindOutputFile identifies a new file path whose parent exists.
	KindOutputFile
	// KindCustom identifies a descriptor constructed with Of.
	KindCustom
)

// String returns a human-readable name for k, e.g. "string".
func (k Kind) String() string {
	switch k {
	case KindString:
		return "string"
	case KindInt:
		return "int"
	case KindBool:
		return "bool"
	case KindFile:
		return "file path"
	case KindDirectory:
		return "directory"
	case KindInputFile:
		return "input file"
	case KindOutputFile:
		return "output file"
	case KindCustom:
		return "custom"
	default:
		return "unknown"
	}
}

// core holds a descriptor's shared state. It is unexported so AnyDescriptor
// can only be implemented by types defined in this package: identity is the
// pointer to a core, which is what makes two Descriptor values (of any T)
// comparable as the "same Param" or "different Params" regardless of shared
// name or type.
type core struct {
	name        string
	kind        Kind
	typeName    string
	description string
	hasDefault  bool
	defaultVal  any
	parseRaw    func(string) (any, error)
	validateAny func(any) error
}

// AnyDescriptor is the type-erased view of a Param descriptor (as returned by
// String, Int, Bool, ...), used wherever a Param must be stored, compared or
// introspected without its type parameter — most notably by package
// activity, which records ordered Param declarations on an Activity.
//
// Only param's own descriptor types implement AnyDescriptor: the unexported
// core method seals the interface to this package, so descriptor identity
// can be trusted by callers such as activity's dedup/conflict logic.
type AnyDescriptor interface {
	// Name returns the Param's declared name. In v1 this is always the
	// external binding name too: there is no alias or rename API.
	Name() string
	// Kind reports the Param's declared type.
	Kind() Kind
	// TypeName returns the generic, documentable name of the value type.
	// It never contains this descriptor's binding name.
	TypeName() string
	// Description returns the Param's optional human-readable description.
	Description() string
	// HasDefault reports whether the Param has a default value.
	HasDefault() bool
	// Default returns the Param's default value, or nil if HasDefault is
	// false.
	Default() any

	core() *core
}

// Descriptor is an identity-bearing, typed Param descriptor. Two Descriptor
// values are the same Param if and only if they were produced by the same
// call to String, Int or Bool (or a copy thereof); calling String("q") twice
// produces two distinct identities even though their name and options match.
//
// Construct one with String, Int, Bool or File.
type Descriptor[T any] struct {
	c *core
}

// Name returns the Param's declared name.
func (d Descriptor[T]) Name() string { return d.c.name }

// Kind reports the Param's declared type.
func (d Descriptor[T]) Kind() Kind { return d.c.kind }

// TypeName returns the generic, documentable name of d's value type.
func (d Descriptor[T]) TypeName() string { return d.c.typeName }

// Description returns the Param's optional human-readable description.
func (d Descriptor[T]) Description() string { return d.c.description }

// HasDefault reports whether the Param has a default value.
func (d Descriptor[T]) HasDefault() bool { return d.c.hasDefault }

// Default returns the Param's default value, or nil if HasDefault is false.
func (d Descriptor[T]) Default() any {
	if !d.c.hasDefault {
		return nil
	}
	return d.c.defaultVal
}

func (d Descriptor[T]) core() *core { return d.c }

var _ AnyDescriptor = Descriptor[string]{}

// settings accumulates the Options applied to a Param under construction.
// Unlike Descriptor, settings is not parameterised by T: Describe has no
// argument that mentions a Param's value type, so Go cannot infer a type
// parameter for it (there is nothing for type inference to work from — see
// the Option doc comment). Keeping settings type-erased lets one Describe
// implementation serve String, Int and Bool alike; Default and Validate stay
// individually type-checked where it matters, at the point you call them,
// and newDescriptor asserts the erased value back to T when it applies them.
type settings struct {
	description string
	hasDefault  bool
	def         any
	validators  []func(any) error
}

// Option configures a Param descriptor (String, Int, Bool, File, ...) at
// construction time.
//
// Option is intentionally not parameterised by the Param's value type T.
// Default[T] and Validate[T] are themselves generic and type-check their own
// argument (the default value, or each validator's parameter type) against
// whatever T you call them with; Describe's argument is a plain string that
// never mentions T, so — because Go type inference cannot derive a type
// parameter from an argument that doesn't reference it, nor from the
// variadic parameter's element type at the call site — Option cannot be
// parameterised by T without forcing every Describe("...") call to spell out
// an explicit type argument. newDescriptor asserts each Default/Validate
// option's erased value back to the constructor's own T when applying it;
// passing an Option built for the wrong type (e.g. param.Default(5) to
// param.String) panics deterministically at construction time, the same
// failure class as an empty name or an invalid default.
type Option func(*settings)

// Describe sets the Param's optional human-readable description. It is
// accepted by String, Int and Bool alike.
func Describe(text string) Option {
	return func(s *settings) { s.description = text }
}

// Default makes the Param optional: v is used whenever the Param's value is
// absent. Default is the sole v1 mechanism for making a Param optional,
// including an explicit zero value such as Default(false). Defaults are
// validated by the same validators as supplied values; an invalid default
// panics at construction time, no later than registration.
func Default[T any](v T) Option {
	return func(s *settings) {
		s.hasDefault = true
		s.def = v
	}
}

// Validate adds one or more validators, applied in declaration order to
// every supplied value and to the default (if any). The first validator to
// return a non-nil error wins; later validators do not run — the same
// ordered, first-error-wins contract validation.Apply implements, which is
// what actually runs them (see newDescriptor). Multiple calls to Validate
// accumulate in the order they appear among a Param's options.
//
// fns is typed as validation.Validator[T] rather than a bare func(T) error
// so a validator can be declared once, as a validation.Validator[T], and
// passed here or to package prompt's equivalent option without conversion;
// an untyped func literal or a plain func(T) error-typed variable remains
// assignable here too, so existing call sites need no change.
func Validate[T any](fns ...validation.Validator[T]) Option {
	return func(s *settings) {
		for _, fn := range fns {
			fn := fn
			s.validators = append(s.validators, func(v any) error {
				typed, ok := v.(T)
				if !ok {
					panic(fmt.Sprintf("param: validator declared for %T applied to a value of type %T", typed, v))
				}
				return fn(typed)
			})
		}
	}
}

// String declares a required-by-default string Param. Use Default to make it
// optional.
func String(name string, opts ...Option) Descriptor[string] {
	return newDescriptor(name, KindString, "string", parseString, opts...)
}

// Int declares a required-by-default int Param, parsed with strconv.Atoi.
func Int(name string, opts ...Option) Descriptor[int] {
	return newDescriptor(name, KindInt, "int", parseInt, opts...)
}

// Bool declares a required-by-default bool Param, parsed with
// strconv.ParseBool. A Bool with Default(false) is optional, matching the
// design's explicit example of an optional Boolean.
func Bool(name string, opts ...Option) Descriptor[bool] {
	return newDescriptor(name, KindBool, "bool", parseBool, opts...)
}

// File declares a required-by-default file-path Param. Its value is a path
// string; File neither reads nor writes that path while constructing or
// preparing the Param. Attach validators such as file.Exists or
// file.MustNotExist with Validate when the command needs preflight feedback.
//
// File has String's binding and default semantics, but its Kind renders as
// "file path" for descriptor consumers and CLI documentation.
func File(name string, opts ...Option) Descriptor[string] {
	return newDescriptor(name, KindFile, "file path", parseString, opts...)
}

// Directory declares a required-by-default path to an existing directory.
func Directory(name string, opts ...Option) Descriptor[string] {
	return newDescriptor(name, KindDirectory, "directory", parseString,
		withRequiredValidators([]validation.Validator[string]{file.Directory()}, opts)...)
}

// InputFile declares a required-by-default path to an existing regular file.
func InputFile(name string, opts ...Option) Descriptor[string] {
	return newDescriptor(name, KindInputFile, "input file", parseString,
		withRequiredValidators([]validation.Validator[string]{file.RegularFile()}, opts)...)
}

// OutputFile declares a required-by-default path for a file that does not
// yet exist and whose parent directory exists. WriteNew remains the
// race-safe authority for the subsequent creation.
func OutputFile(name string, opts ...Option) Descriptor[string] {
	return newDescriptor(name, KindOutputFile, "output file", parseString,
		withRequiredValidators([]validation.Validator[string]{file.MustNotExist(), file.ParentExists()}, opts)...)
}

// Parser converts one raw external value into T.
//
// A parser error is reported by Prepare as a ValidationError for the
// concrete descriptor, because both parser rejection and validator rejection
// are invalid user input at the target boundary. The original error remains
// available through errors.Is/errors.As on ValidationError.
type Parser[T any] func(string) (T, error)

// Type is a reusable, documentable parameter type. Construct it with
// DefineType and bind it to a concrete external parameter with Of.
type Type[T any] struct {
	typeName string
	parse    Parser[T]
}

// DefineType defines a reusable parameter type with a generic name and raw
// input parser. typeName names the type itself (for example "BIP-39 batch
// mnemonic"), not an individual parameter binding.
func DefineType[T any](typeName string, parse Parser[T]) Type[T] {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		panic("param: type name must not be empty")
	}
	if parse == nil {
		panic("param: type parser must not be nil")
	}
	return Type[T]{typeName: typeName, parse: parse}
}

// Of binds typ to the external parameter name. Validators supplied in opts
// run only after typ has parsed the raw value successfully.
func Of[T any](typ Type[T], name string, opts ...Option) Descriptor[T] {
	if typ.typeName == "" || typ.parse == nil {
		panic("param: Type must be created with DefineType")
	}
	return newDescriptor(name, KindCustom, typ.typeName, typ.parse, opts...)
}

func parseString(s string) (string, error) { return s, nil }

// withRequiredValidators adds a type's invariant validators before caller
// options. The validators remain an implementation detail of the type: users
// select Directory/InputFile/OutputFile rather than spelling them manually.
// Keeping them in the descriptor's validator chain also applies the same
// invariants to defaults at construction time.
func withRequiredValidators[T any](validators []validation.Validator[T], opts []Option) []Option {
	all := make([]Option, 0, len(opts)+1)
	all = append(all, Validate(validators...))
	return append(all, opts...)
}

func parseInt(s string) (int, error) {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid int value %q: %w", s, err)
	}
	return v, nil
}

func parseBool(s string) (bool, error) {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return false, fmt.Errorf("invalid bool value %q: %w", s, err)
	}
	return v, nil
}

func newDescriptor[T any](name string, kind Kind, typeName string, parse Parser[T], opts ...Option) Descriptor[T] {
	name = strings.TrimSpace(name)
	if name == "" {
		panic("param: name must not be empty")
	}

	var s settings
	for _, opt := range opts {
		opt(&s)
	}

	var def T
	if s.hasDefault {
		typed, ok := s.def.(T)
		if !ok {
			panic(fmt.Sprintf("param %q: default has type %T, want a %s value", name, s.def, kind))
		}
		def = typed
	}

	// settings.validators is []func(any) error — type-erased, because
	// settings itself cannot be parameterised by T (see the Option doc
	// comment). validation.Apply needs []validation.Validator[any]; the two
	// element types have identical underlying types but are not identical,
	// so Go requires converting element-by-element rather than the whole
	// slice at once. validation.Apply itself supplies the ordered,
	// first-error-wins loop and the nil-entry tolerance this used to
	// implement inline.
	erased := make([]validation.Validator[any], len(s.validators))
	for i, fn := range s.validators {
		erased[i] = fn
	}
	validate := func(v any) error {
		return validation.Apply(v, erased...)
	}

	if s.hasDefault {
		if err := validate(def); err != nil {
			panic(fmt.Sprintf("param %q: invalid default: %v", name, err))
		}
	}

	c := &core{
		name:        name,
		kind:        kind,
		typeName:    typeName,
		description: s.description,
		hasDefault:  s.hasDefault,
		defaultVal:  def,
		parseRaw: func(raw string) (any, error) {
			v, err := parse(raw)
			if err != nil {
				return nil, err
			}
			return v, nil
		},
		validateAny: validate,
	}
	return Descriptor[T]{c: c}
}
