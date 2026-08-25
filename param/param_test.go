package param_test

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/away2go/way2go/file"
	"github.com/away2go/way2go/param"
	"github.com/away2go/way2go/validation"
)

// -- construction / options -------------------------------------------------

func TestStringIntBoolConstructorsRequireNonEmptyName(t *testing.T) {
	for _, fn := range []func(){
		func() { param.String("") },
		func() { param.Int("  ") },
		func() { param.Bool("") },
	} {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("expected panic for empty name")
				}
			}()
			fn()
		}()
	}
}

func TestDescriptorIntrospection(t *testing.T) {
	d := param.String("q", param.Describe("query text"), param.Default("hi"))
	if d.Name() != "q" {
		t.Fatalf("Name() = %q, want %q", d.Name(), "q")
	}
	if d.Kind() != param.KindString {
		t.Fatalf("Kind() = %v, want %v", d.Kind(), param.KindString)
	}
	if d.Description() != "query text" {
		t.Fatalf("Description() = %q", d.Description())
	}
	if !d.HasDefault() {
		t.Fatalf("HasDefault() = false, want true")
	}
	if d.Default() != "hi" {
		t.Fatalf("Default() = %v, want %q", d.Default(), "hi")
	}
}

func TestFileDescriptorIntrospectionDefaultsAndValidation(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/new.data"
	d := param.File("output", param.Describe("new output file"), param.Default(path), param.Validate(file.MustNotExist(), file.ParentExists(), file.Extension(".data")))

	if d.Kind() != param.KindFile {
		t.Fatalf("Kind() = %v, want %v", d.Kind(), param.KindFile)
	}
	if d.Kind().String() != "file path" {
		t.Fatalf("Kind().String() = %q, want file path", d.Kind().String())
	}
	if d.Description() != "new output file" || d.Default() != path || !d.HasDefault() {
		t.Fatalf("descriptor metadata = description %q, default %v, has default %v", d.Description(), d.Default(), d.HasDefault())
	}

	values, err := param.Prepare([]param.AnyDescriptor{d}, map[param.AnyDescriptor]param.RawValue{})
	if err != nil {
		t.Fatalf("Prepare default: %v", err)
	}
	if got := param.Read(param.NewContext(context.Background(), values), d); got != path {
		t.Fatalf("Read(default) = %q, want %q", got, path)
	}

	_, err = param.Prepare([]param.AnyDescriptor{d}, map[param.AnyDescriptor]param.RawValue{
		d: {Value: dir + "/wrong.txt", Present: true},
	})
	var validationErr *param.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Prepare invalid supplied file = %v (%T), want ValidationError", err, err)
	}
}

func TestRequiredParamHasNoDefault(t *testing.T) {
	d := param.String("q")
	if d.HasDefault() {
		t.Fatalf("HasDefault() = true, want false for a required param")
	}
	if d.Default() != nil {
		t.Fatalf("Default() = %v, want nil", d.Default())
	}
}

func TestDefaultForWrongTypePanicsAtConstruction(t *testing.T) {
	// param.Default(v) is generic in v's own type, inferred from v; Go
	// cannot tie that inference to whichever constructor (String/Int/Bool)
	// the Option ends up passed to (Describe has the same limitation, which
	// is why Option is not parameterised by T at all — see its doc comment).
	// Applying an Option built for the wrong type must still fail
	// deterministically, at construction time.
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic constructing an Int param with a string default")
		}
	}()
	param.Int("age", param.Default("not-an-int"))
}

func TestValidatorForWrongTypePanicsWhenApplied(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic applying an int validator to a string param")
		}
	}()
	intValidator := param.Validate(func(v int) error { return nil })
	q := param.String("q", intValidator)
	descriptors := []param.AnyDescriptor{q}
	_, _ = param.Prepare(descriptors, map[param.AnyDescriptor]param.RawValue{
		q: {Value: "hi", Present: true},
	})
}

func TestBoolDefaultFalseIsOptional(t *testing.T) {
	d := param.Bool("flag", param.Default(false))
	if !d.HasDefault() {
		t.Fatalf("HasDefault() = false, want true for explicit Default(false)")
	}
	if d.Default() != false {
		t.Fatalf("Default() = %v, want false", d.Default())
	}
}

// -- parsing ------------------------------------------------------------

func TestPrepareParsesStringIntBool(t *testing.T) {
	name := param.String("name")
	age := param.Int("age")
	active := param.Bool("active")
	descriptors := []param.AnyDescriptor{name, age, active}
	raw := map[param.AnyDescriptor]param.RawValue{
		name:   {Value: "ada", Present: true},
		age:    {Value: "36", Present: true},
		active: {Value: "true", Present: true},
	}

	values, err := param.Prepare(descriptors, raw)
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}

	ctx := param.NewContext(context.Background(), values)
	if got := param.Read(ctx, name); got != "ada" {
		t.Fatalf("Read(name) = %q, want %q", got, "ada")
	}
	if got := param.Read(ctx, age); got != 36 {
		t.Fatalf("Read(age) = %d, want 36", got)
	}
	if got := param.Read(ctx, active); got != true {
		t.Fatalf("Read(active) = %v, want true", got)
	}
}

func TestPrepareRejectsUnparsableIntAsValidationError(t *testing.T) {
	age := param.Int("age")
	descriptors := []param.AnyDescriptor{age}
	raw := map[param.AnyDescriptor]param.RawValue{
		age: {Value: "not-a-number", Present: true},
	}

	_, err := param.Prepare(descriptors, raw)
	if err == nil {
		t.Fatalf("expected an error for unparsable int")
	}
	var validationErr *param.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v (%T), want *param.ValidationError", err, err)
	}
	if validationErr.Name != "age" {
		t.Fatalf("ValidationError.Name = %q, want %q", validationErr.Name, "age")
	}
	var parseErr *param.ParseError
	if !errors.As(err, &parseErr) || parseErr.Raw != "not-a-number" {
		t.Fatalf("ValidationError should retain ParseError cause, got %v", err)
	}
}

func TestPrepareRejectsUnparsableBool(t *testing.T) {
	active := param.Bool("active")
	descriptors := []param.AnyDescriptor{active}
	raw := map[param.AnyDescriptor]param.RawValue{
		active: {Value: "not-a-bool", Present: true},
	}

	_, err := param.Prepare(descriptors, raw)
	var validationErr *param.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v (%T), want *param.ValidationError", err, err)
	}
}

func TestDefineTypeAndOfParseThenValidate(t *testing.T) {
	parseErr := errors.New("not an even integer")
	evenInt := param.DefineType("even integer", func(raw string) (int, error) {
		v, err := strconv.Atoi(raw)
		if err != nil || v%2 != 0 {
			return 0, parseErr
		}
		return v, nil
	})
	d := param.Of(evenInt, "count", param.Validate(func(v int) error {
		if v < 2 {
			return errors.New("must be at least 2")
		}
		return nil
	}))
	if d.Kind() != param.KindCustom || d.TypeName() != "even integer" {
		t.Fatalf("descriptor metadata = kind %v, type %q", d.Kind(), d.TypeName())
	}

	values, err := param.Prepare([]param.AnyDescriptor{d}, map[param.AnyDescriptor]param.RawValue{
		d: {Value: "4", Present: true},
	})
	if err != nil {
		t.Fatalf("Prepare(valid) = %v", err)
	}
	if got := param.Read(param.NewContext(context.Background(), values), d); got != 4 {
		t.Fatalf("Read() = %d, want 4", got)
	}

	_, err = param.Prepare([]param.AnyDescriptor{d}, map[param.AnyDescriptor]param.RawValue{
		d: {Value: "3", Present: true},
	})
	var validationErr *param.ValidationError
	if !errors.As(err, &validationErr) || !errors.Is(err, parseErr) {
		t.Fatalf("Prepare(parse failure) = %v, want ValidationError wrapping parse cause", err)
	}

	_, err = param.Prepare([]param.AnyDescriptor{d}, map[param.AnyDescriptor]param.RawValue{
		d: {Value: "0", Present: true},
	})
	if !errors.As(err, &validationErr) || validationErr.Err.Error() != "must be at least 2" {
		t.Fatalf("Prepare(validator failure) = %v, want validator ValidationError", err)
	}
}

func TestDefineTypeRejectsInvalidDefinitions(t *testing.T) {
	for _, fn := range []func(){
		func() { param.DefineType[int]("", func(string) (int, error) { return 0, nil }) },
		func() { param.DefineType[int]("integer", nil) },
		func() { param.Of(param.Type[int]{}, "value") },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("expected panic")
				}
			}()
			fn()
		}()
	}
}

func TestPathTypeConstructorsEnforceTheirSemantics(t *testing.T) {
	dir := t.TempDir()
	regular := dir + "/input.txt"
	if err := os.WriteFile(regular, []byte("input"), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		d    param.Descriptor[string]
		ok   string
		bad  string
		kind param.Kind
	}{
		{"directory", param.Directory("dir"), dir, regular, param.KindDirectory},
		{"input file", param.InputFile("input"), regular, dir, param.KindInputFile},
		{"output file", param.OutputFile("output"), dir + "/new.txt", regular, param.KindOutputFile},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.d.Kind() != tt.kind {
				t.Fatalf("Kind() = %v, want %v", tt.d.Kind(), tt.kind)
			}
			if _, err := param.Prepare([]param.AnyDescriptor{tt.d}, map[param.AnyDescriptor]param.RawValue{tt.d: {Value: tt.ok, Present: true}}); err != nil {
				t.Fatalf("Prepare(valid) = %v", err)
			}
			_, err := param.Prepare([]param.AnyDescriptor{tt.d}, map[param.AnyDescriptor]param.RawValue{tt.d: {Value: tt.bad, Present: true}})
			var validationErr *param.ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("Prepare(invalid) = %v, want ValidationError", err)
			}
		})
	}
}

func TestPathTypeConstructorsValidateDefaults(t *testing.T) {
	dir := t.TempDir()
	regular := dir + "/input.txt"
	if err := os.WriteFile(regular, []byte("input"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, fn := range []func(){
		func() { param.Directory("dir", param.Default(regular)) },
		func() { param.InputFile("input", param.Default(dir)) },
		func() { param.OutputFile("output", param.Default(regular)) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("expected invalid path default to panic")
				}
			}()
			fn()
		}()
	}
}

// -- required / default behaviour ----------------------------------------

func TestPrepareRequiredMissingFails(t *testing.T) {
	q := param.String("q")
	descriptors := []param.AnyDescriptor{q}

	_, err := param.Prepare(descriptors, map[param.AnyDescriptor]param.RawValue{})
	var missingErr *param.MissingValueError
	if !errors.As(err, &missingErr) {
		t.Fatalf("error = %v (%T), want *param.MissingValueError", err, err)
	}
	if missingErr.Name != "q" {
		t.Fatalf("MissingValueError.Name = %q, want %q", missingErr.Name, "q")
	}
}

func TestPrepareUsesDefaultWhenAbsent(t *testing.T) {
	limit := param.Int("limit", param.Default(10))
	descriptors := []param.AnyDescriptor{limit}

	values, err := param.Prepare(descriptors, map[param.AnyDescriptor]param.RawValue{})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	ctx := param.NewContext(context.Background(), values)
	if got := param.Read(ctx, limit); got != 10 {
		t.Fatalf("Read(limit) = %d, want 10 (default)", got)
	}
}

func TestPrepareSuppliedValueOverridesDefault(t *testing.T) {
	limit := param.Int("limit", param.Default(10))
	descriptors := []param.AnyDescriptor{limit}
	raw := map[param.AnyDescriptor]param.RawValue{
		limit: {Value: "25", Present: true},
	}

	values, err := param.Prepare(descriptors, raw)
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	ctx := param.NewContext(context.Background(), values)
	if got := param.Read(ctx, limit); got != 25 {
		t.Fatalf("Read(limit) = %d, want 25", got)
	}
}

// -- empty-string presence semantics --------------------------------------

func TestPresentEmptyStringIsDistinctFromAbsent(t *testing.T) {
	q := param.String("q", param.Default("fallback"))
	descriptors := []param.AnyDescriptor{q}

	// Explicitly supplied empty string: present, so default must not apply.
	raw := map[param.AnyDescriptor]param.RawValue{
		q: {Value: "", Present: true},
	}
	values, err := param.Prepare(descriptors, raw)
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	ctx := param.NewContext(context.Background(), values)
	if got := param.Read(ctx, q); got != "" {
		t.Fatalf("Read(q) = %q, want empty string (present value), not the default", got)
	}

	// Absent: default applies.
	values2, err := param.Prepare(descriptors, map[param.AnyDescriptor]param.RawValue{})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	ctx2 := param.NewContext(context.Background(), values2)
	if got := param.Read(ctx2, q); got != "fallback" {
		t.Fatalf("Read(q) = %q, want %q (default applied when absent)", got, "fallback")
	}
}

// -- validators -----------------------------------------------------------

func TestValidatorsRunInDeclarationOrderAndFirstFailureWins(t *testing.T) {
	var order []string
	first := func(s string) error {
		order = append(order, "first")
		return errors.New("first failed")
	}
	second := func(s string) error {
		order = append(order, "second")
		return errors.New("second failed")
	}

	q := param.String("q", param.Validate(first, second))
	descriptors := []param.AnyDescriptor{q}
	raw := map[param.AnyDescriptor]param.RawValue{
		q: {Value: "x", Present: true},
	}

	_, err := param.Prepare(descriptors, raw)
	var valErr *param.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("error = %v (%T), want *param.ValidationError", err, err)
	}
	if valErr.Err.Error() != "first failed" {
		t.Fatalf("first validator error = %v, want %q", valErr.Err, "first failed")
	}
	if len(order) != 1 || order[0] != "first" {
		t.Fatalf("order = %v, want only [first] to have run", order)
	}
}

func TestValidatorsRunInDeclarationOrderSecondFailureAfterFirstPasses(t *testing.T) {
	var order []string
	first := func(s string) error {
		order = append(order, "first")
		return nil
	}
	second := func(s string) error {
		order = append(order, "second")
		return errors.New("second failed")
	}

	q := param.String("q", param.Validate(first, second))
	descriptors := []param.AnyDescriptor{q}
	raw := map[param.AnyDescriptor]param.RawValue{
		q: {Value: "x", Present: true},
	}

	_, err := param.Prepare(descriptors, raw)
	var valErr *param.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("error = %v (%T), want *param.ValidationError", err, err)
	}
	if !strings.Contains(valErr.Error(), "second failed") {
		t.Fatalf("error = %v, want it to mention %q", valErr, "second failed")
	}
	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Fatalf("order = %v, want [first second]", order)
	}
}

func TestValidatorAppliesToDefaultToo(t *testing.T) {
	// A default that itself violates a validator must be rejected no later
	// than construction (well before registration).
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for invalid default")
		}
	}()
	param.Int("limit", param.Default(-1), param.Validate(func(v int) error {
		if v < 0 {
			return errors.New("must be >= 0")
		}
		return nil
	}))
}

func TestValidateAcceptsValidationValidatorTypedVariable(t *testing.T) {
	// param.Validate's parameter type is validation.Validator[T], not a bare
	// func(T) error; a variable declared with that exact shared type must be
	// accepted with no conversion, proving param and a sibling package such
	// as prompt can hand the same validator value to both.
	var isPositive validation.Validator[int] = func(v int) error {
		if v <= 0 {
			return errors.New("must be positive")
		}
		return nil
	}
	d := param.Int("n", param.Validate(isPositive))
	descriptors := []param.AnyDescriptor{d}

	_, err := param.Prepare(descriptors, map[param.AnyDescriptor]param.RawValue{
		d: {Value: "-5", Present: true},
	})
	var valErr *param.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("error = %v (%T), want *param.ValidationError", err, err)
	}
}

// TestValidationApplySemanticsBackParamValidationFailure is a light
// integration check that Prepare's validation failure is genuinely backed by
// validation.Apply's ordered, first-error-wins contract, not a separate
// reimplementation: applying the same validators directly through
// validation.Apply produces the same first error as Prepare does.
func TestValidationApplySemanticsBackParamValidationFailure(t *testing.T) {
	first := func(v int) error { return errors.New("first failed") }
	second := func(v int) error { return errors.New("second failed") }

	direct := validation.Apply(7, first, second)

	d := param.Int("n", param.Validate(first, second))
	descriptors := []param.AnyDescriptor{d}
	_, err := param.Prepare(descriptors, map[param.AnyDescriptor]param.RawValue{
		d: {Value: "7", Present: true},
	})
	var valErr *param.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("error = %v (%T), want *param.ValidationError", err, err)
	}
	if valErr.Err.Error() != direct.Error() {
		t.Fatalf("Prepare's validation error = %q, want it to match validation.Apply's direct result %q", valErr.Err, direct)
	}
}

func TestValidatorAcceptsValidDefault(t *testing.T) {
	d := param.Int("limit", param.Default(5), param.Validate(func(v int) error {
		if v < 0 {
			return errors.New("must be >= 0")
		}
		return nil
	}))
	if d.Default() != 5 {
		t.Fatalf("Default() = %v, want 5", d.Default())
	}
}

// -- undeclared reads / programmer error contract --------------------------

// progErr mirrors the (error + Way2GoProgrammerError) shape without
// importing package activity, since param must not depend on activity.
type progErr interface {
	error
	Way2GoProgrammerError()
}

func TestReadOfUndeclaredParamPanicsWithProgrammerError(t *testing.T) {
	declared := param.String("q")
	other := param.String("q") // distinct identity, same name
	descriptors := []param.AnyDescriptor{declared}
	values, err := param.Prepare(descriptors, map[param.AnyDescriptor]param.RawValue{
		declared: {Value: "hi", Present: true},
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	ctx := param.NewContext(context.Background(), values)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic reading an undeclared param")
		}
		err, ok := r.(error)
		if !ok {
			t.Fatalf("recovered value %v is not an error", r)
		}
		var pe progErr
		if !errors.As(err, &pe) {
			t.Fatalf("recovered error %v (%T) does not satisfy the programmer-error contract", err, err)
		}
		var ure *param.UndeclaredReadError
		if !errors.As(err, &ure) {
			t.Fatalf("recovered error %v (%T) is not a *param.UndeclaredReadError", err, err)
		}
	}()
	param.Read(ctx, other)
}

func TestReadWithoutAnyPreparedContextPanics(t *testing.T) {
	q := param.String("q")
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic reading from a context with no prepared Values")
		}
	}()
	param.Read(context.Background(), q)
}

func TestOrdinaryInputErrorsDoNotSatisfyProgrammerErrorContract(t *testing.T) {
	cases := []error{
		&param.MissingValueError{Name: "q"},
		&param.ParseError{Name: "n", Raw: "x", Err: errors.New("bad")},
		&param.ValidationError{Name: "n", Err: errors.New("bad")},
	}
	for _, err := range cases {
		var pe progErr
		if errors.As(err, &pe) {
			t.Fatalf("%T unexpectedly satisfies the programmer-error contract", err)
		}
	}
}

// -- descriptor identity ---------------------------------------------------

func TestDistinctConstructorCallsHaveDistinctIdentity(t *testing.T) {
	a := param.String("q")
	b := param.String("q")
	var anyA, anyB param.AnyDescriptor = a, b
	if anyA == anyB {
		t.Fatalf("two distinct param.String(\"q\") calls must not be the same identity")
	}
}

func TestCopyOfDescriptorSharesIdentity(t *testing.T) {
	a := param.String("q")
	b := a
	var anyA, anyB param.AnyDescriptor = a, b
	if anyA != anyB {
		t.Fatalf("copying a Descriptor value must preserve identity")
	}
}
