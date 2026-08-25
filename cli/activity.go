package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/param"
)

// Definition is the immutable result of cli.Activity: a CLI Activity ready
// to be placed as a leaf under cli.Group/cli.All. Its Descriptor method
// exposes the effective, immutable activity.Descriptor snapshot —
// introspectable without executing handler — that API contract
// requires.
type Definition struct {
	descriptor activity.Descriptor
	flagParams []param.AnyDescriptor // Source == optionSource, declaration order
	argParams  []param.AnyDescriptor // Source == argSource, declaration order
	allParams  []param.AnyDescriptor // full declaration order, for param.Prepare
	handler    HandlerFunc           // already wrapped with declared middleware, outermost first
}

// Descriptor returns d's immutable effective activity.Descriptor: name,
// description, target ("cli"), every declared Param binding and every
// declared middleware, in declaration order. Descriptor never executes d's
// handler.
func (d Definition) Descriptor() activity.Descriptor { return d.descriptor }

func (d Definition) name() string { return d.descriptor.Name }

// Activity declares a CLI Activity named name with handler and the given
// CLI-capable options (activity.Describe, cli.FromOptions, cli.FromArgs, CLI or
// portable middleware). It builds directly on activity.New(name, "cli") and
// Builder.ApplyCLI, exactly as web.Activity builds on
// activity.New(name, "web") and Builder.ApplyWeb.
//
// Activity panics — deterministically, at Activity construction time rather
// than at first execution — if handler is nil, if any option conflicts (see
// activity.Builder.DeclareParam and NewCLIBinding/NewCLIMiddleware), or if
// the Activity's positional (FromArgs) Params place a required argument after
// an optional one. This mirrors the panic discipline package activity
// already uses for the same class of registration failure (see
// declareMiddleware in activity/middleware.go): Options apply with no error
// return available to the call site, typically a package-level var
// initializer.
func Activity(name string, handler HandlerFunc, options ...Option) Definition {
	if handler == nil {
		panic(fmt.Sprintf("cli: activity %q: handler must not be nil", name))
	}

	b := activity.New(name, "cli")
	b.ApplyCLI(options...)
	descriptor := b.Snapshot()

	var flagParams, argParams, allParams []param.AnyDescriptor
	for _, pb := range descriptor.Params {
		allParams = append(allParams, pb.Param)
		switch pb.Source {
		case optionSource:
			flagParams = append(flagParams, pb.Param)
		case argSource:
			argParams = append(argParams, pb.Param)
		}
	}
	validateArgOrder(descriptor.Name, argParams)

	var wrappers []activity.Wrapper[HandlerFunc]
	for _, opt := range options {
		if w, ok := activity.CLIWrapper[HandlerFunc](opt); ok {
			wrappers = append(wrappers, w)
		}
	}
	wrapped := activity.Chain(handler, wrappers...)

	return Definition{
		descriptor: descriptor,
		flagParams: flagParams,
		argParams:  argParams,
		allParams:  allParams,
		handler:    wrapped,
	}
}

// validateArgOrder enforces that among an Activity's positional (FromArgs)
// Params, every required (no-Default) Param precedes every optional
// (Default) Param, regardless of how many FromArgs calls contributed them.
func validateArgOrder(activityName string, argParams []param.AnyDescriptor) {
	seenOptional := ""
	for _, p := range argParams {
		if p.HasDefault() {
			seenOptional = p.Name()
			continue
		}
		if seenOptional != "" {
			panic(fmt.Sprintf(
				"cli: activity %q: required argument %q must be declared before optional argument %q",
				activityName, p.Name(), seenOptional,
			))
		}
	}
}

// buildCommand constructs a fresh, unexported *cobra.Command for d each time
// it is called — Node.buildCommand is invoked once per App.Execute call (see
// group.go) precisely so that no *cobra.Command, and no pflag.FlagSet state
// such as a flag's Changed bit, is ever shared or reused across executions.
// That is what keeps concurrent App.Execute calls (and concurrent test
// executions using independently injected output buffers) from
// cross-contaminating each other, without cli needing any locking of its
// own.
func (d Definition) buildCommand(result *execResult) *cobra.Command {
	cmd := &cobra.Command{
		Use:           d.descriptor.Name,
		Short:         d.descriptor.Description,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MaximumNArgs(len(d.argParams)),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw := make(map[param.AnyDescriptor]param.RawValue, len(d.allParams))
			for _, p := range d.flagParams {
				v, err := flagValue(cmd, p)
				if err != nil {
					return &inputError{err: err}
				}
				raw[p] = param.RawValue{Value: v, Present: cmd.Flags().Changed(p.Name())}
			}
			for i, p := range d.argParams {
				if i < len(args) {
					raw[p] = param.RawValue{Value: args[i], Present: true}
				} else {
					raw[p] = param.RawValue{}
				}
			}

			values, err := param.Prepare(d.allParams, raw)
			if err != nil {
				return &inputError{err: err}
			}

			ctx := param.NewContext(cmd.Context(), values)
			outcome, err := invoke(ctx, d.handler)
			if err != nil {
				return err
			}
			result.outcome = outcome
			result.hasOutcome = true
			result.activityName = d.descriptor.Name
			return nil
		},
	}
	for _, p := range d.flagParams {
		registerFlag(cmd, p)
	}
	return cmd
}

// registerFlag registers p as a long-form flag on cmd, choosing pflag's flag
// type by p.Kind(). A param.KindBool Param must be registered via
// cmd.Flags().Bool rather than cmd.Flags().String: only pflag's native bool
// flag type gives that flag pflag/Cobra's usual "bare flag means true"
// shorthand (--verbose, with no "=true"), which is the behavior every other
// pflag/Cobra-based CLI already gives its bool flags. Every other Kind is
// registered as a string flag: param.Prepare (see param.Descriptor.parseRaw)
// parses and validates the raw string itself, so no other Kind needs a
// typed pflag flag. The registered default is always the flag type's zero
// value — never the Param's own param.Default, if any — because
// buildCommand's RunE loop applies a Param's default via param.Prepare,
// keyed off cmd.Flags().Changed, not off whatever pflag itself reports as
// unset.
func registerFlag(cmd *cobra.Command, p param.AnyDescriptor) {
	switch p.Kind() {
	case param.KindBool:
		cmd.Flags().Bool(p.Name(), false, p.Description())
	default:
		cmd.Flags().String(p.Name(), "", p.Description())
	}
}

// flagValue reads p's flag value back off cmd, in whatever pflag type
// registerFlag registered it as, and renders it back to the raw string
// param.Prepare expects (param.Bool parses a string with strconv.ParseBool,
// so a bool flag's value round-trips through strconv.FormatBool). This is
// the read-side counterpart to registerFlag: the two must stay in lockstep
// per Kind, since GetString fails on a flag that was not registered as a
// string flag.
func flagValue(cmd *cobra.Command, p param.AnyDescriptor) (string, error) {
	switch p.Kind() {
	case param.KindBool:
		v, err := cmd.Flags().GetBool(p.Name())
		if err != nil {
			return "", err
		}
		return strconv.FormatBool(v), nil
	default:
		return cmd.Flags().GetString(p.Name())
	}
}
