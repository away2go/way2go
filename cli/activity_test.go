package cli_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/cli"
	"github.com/away2go/way2go/param"
)

func noop(ctx context.Context) cli.Outcome { return cli.OK() }

// TestDescriptorIntrospectableWithoutExecution proves API contract
// #1: cli.Activity produces a Definition whose Descriptor is a complete,
// immutable snapshot — name, description, target, Param bindings and
// middleware, all inspectable without ever calling the handler.
func TestDescriptorIntrospectableWithoutExecution(t *testing.T) {
	name := param.String("name", param.Describe("who to greet"))
	executed := false
	def := cli.Activity("greet", func(ctx context.Context) cli.Outcome {
		executed = true
		return cli.OK()
	},
		activity.Describe("Greets someone."),
		cli.FromArgs(name),
	)

	d := def.Descriptor()
	if d.Name != "greet" {
		t.Fatalf("Name = %q, want %q", d.Name, "greet")
	}
	if d.Target != "cli" {
		t.Fatalf("Target = %q, want %q", d.Target, "cli")
	}
	if d.Description != "Greets someone." {
		t.Fatalf("Description = %q, want %q", d.Description, "Greets someone.")
	}
	if len(d.Params) != 1 || d.Params[0].Param.Name() != "name" || d.Params[0].Source != "arg" {
		t.Fatalf("Params = %+v, want one name/arg binding", d.Params)
	}
	if executed {
		t.Fatalf("Descriptor() must not execute the handler")
	}
}

// TestActivityPanicsOnNilHandler proves cli.Activity rejects a nil handler
// deterministically at construction time.
func TestActivityPanicsOnNilHandler(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for nil handler")
		}
	}()
	cli.Activity("x", nil)
}

// TestFromArgsRequiredAfterOptionalPanics proves API contract:
// required positional Params must precede optional ones, validated at
// registration (cli.Activity construction), even across two separate
// FromArgs calls.
func TestFromArgsRequiredAfterOptionalPanics(t *testing.T) {
	optionalFirst := param.String("first", param.Default("x"))
	requiredSecond := param.String("second")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for required-after-optional positional arg")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "second") {
			t.Fatalf("panic message = %q, want it to mention %q", msg, "second")
		}
	}()
	cli.Activity("x", noop, cli.FromArgs(optionalFirst), cli.FromArgs(requiredSecond))
}

// TestFromArgsRequiredBeforeOptionalIsAccepted is the mirror-image proof:
// required-then-optional across multiple FromArgs calls is a valid ordering.
func TestFromArgsRequiredBeforeOptionalIsAccepted(t *testing.T) {
	required := param.String("first")
	optional := param.String("second", param.Default("x"))

	def := cli.Activity("x", noop, cli.FromArgs(required), cli.FromArgs(optional))
	d := def.Descriptor()
	if len(d.Params) != 2 {
		t.Fatalf("Params = %+v, want 2 entries", d.Params)
	}
}

// TestDuplicateExternalNameFromDistinctDescriptorsConflicts proves that two
// distinct param descriptors sharing an external name are always rejected,
// even bound to the same source, matching activity.Builder.DeclareParam's
// documented conflict rule.
func TestDuplicateExternalNameFromDistinctDescriptorsConflicts(t *testing.T) {
	a := param.String("limit", param.Default("a"))
	b := param.String("limit", param.Default("b"))

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for conflicting param names")
		}
	}()
	cli.Activity("x", noop, cli.FromOptions(a), cli.FromOptions(b))
}

// TestSameDescriptorSameBindingIsDeduplicated proves re-declaring the same
// Param identity with the same source (e.g. reusing a shared package-level
// descriptor in more than one FromOptions call) does not duplicate it.
func TestSameDescriptorSameBindingIsDeduplicated(t *testing.T) {
	limit := param.Int("limit", param.Default(1))
	def := cli.Activity("x", noop, cli.FromOptions(limit), cli.FromOptions(limit))
	d := def.Descriptor()
	if len(d.Params) != 1 {
		t.Fatalf("Params = %+v, want exactly one entry after dedup", d.Params)
	}
}

// TestSameDescriptorConflictingBindingIsRejected proves the same Param
// identity cannot be bound both as a flag and an arg.
func TestSameDescriptorConflictingBindingIsRejected(t *testing.T) {
	shared := param.String("name")
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for conflicting source binding")
		}
	}()
	cli.Activity("x", noop, cli.FromOptions(shared), cli.FromArgs(shared))
}
