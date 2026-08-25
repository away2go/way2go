package cli

import (
	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/param"
)

// optionSource and argSource are the opaque ParamBinding.Source tags this
// package uses. They are also the strings buildCommand and Activity use
// internally to tell option bindings from positional-argument bindings apart
// within an effective activity.Descriptor's Params.
const (
	optionSource = "option"
	argSource    = "arg"
)

// FromOptions declares and binds each of ds as a long-form CLI option, named
// after the Param's own name (param.AnyDescriptor.Name). v1 has no aliases,
// short options or prompts. An option's required/default/typed validation
// semantics are exactly the ones already implemented by param: FromOptions
// itself performs no parsing or validation, it only records the
// binding. The same descriptor bound as an option more than once is
// deduplicated; the same descriptor bound to conflicting sources, or a
// distinct descriptor sharing another declared Param's name, is rejected —
// see activity.Builder.DeclareParam.
func FromOptions(ds ...param.AnyDescriptor) Option {
	return activity.NewCLIBinding(optionSource, ds...)
}

// FromArgs binds each of ds, in the order given, as a fixed positional
// argument. Positional Params are matched to command-line arguments strictly
// by declaration order across every FromArgs call on the same Activity — a
// second FromArgs call's descriptors come after the first's. A required
// (no-Default) positional Param may not follow an optional (Default) one:
// cli.Activity rejects that ordering at registration. v1 has no variadic
// positional argument; a caller supplying more positional arguments than
// declared fails with an input error.
func FromArgs(ds ...param.AnyDescriptor) Option {
	return activity.NewCLIBinding(argSource, ds...)
}
