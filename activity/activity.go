// Package activity implements the target-neutral declarative core of the
// Way2Go Activity model: option composition, ordered Param and middleware
// descriptor bookkeeping, and the immutable Activity descriptor snapshot.
//
// An Activity has a non-empty name, a target-specific handler, an optional
// description, an ordered set of declared Params and target bindings, and an
// ordered set of middleware descriptors. Everything after the handler is an
// option: there is no mandatory Use(...) or Params(...) wrapper at the
// Activity declaration site.
//
// This package also owns the closed Web/CLI target option contract
// (WebOption, CLIOption, Portable — see option.go) and the generic
// middleware wrapping primitive (Wrapper, Chain, NewMiddleware,
// NewWebMiddleware, NewCLIMiddleware — see middleware.go). Those types must
// live here rather than in web/cli: WebOption and CLIOption are sealed with
// unexported methods so the contract stays closed to Way2Go-owned values,
// and a target-neutral value such as Describe can only have a static type
// satisfying both sealed contracts if that type — and the seal itself — is
// declared in one package that both web and cli merely alias into (web.
// Option = activity.WebOption, cli.Option = activity.CLIOption). This
// package does not implement middleware *execution*, or Web/CLI resolution
// and registration — those belong to the web and cli packages.
// It provides the Builder and Descriptor types those packages construct
// their target-specific Activity constructors on top of.
package activity

import (
	"fmt"
	"strings"

	"github.com/away2go/way2go/param"
)

// ParamBinding is the immutable, introspectable record of one Param declared
// on an Activity together with the external source it is bound to. In v1
// there are no aliases: a Param's external name is always its own declared
// name (param.AnyDescriptor.Name), and Source is an opaque, target-defined
// tag (e.g. "query", "path", "form", "flag", "arg") supplied by the target
// package that declared the binding.
type ParamBinding struct {
	// Param is the bound Param's type-erased descriptor.
	Param param.AnyDescriptor
	// Source is the opaque, target-defined binding tag (e.g. "query",
	// "path", "form", "flag", "arg").
	Source string
}

// Middleware is the declaration-order identity of one middleware contributed
// to an Activity. Target packages attach the concrete wrapping behaviour;
// activity only records identity and order here. The first declared
// middleware is the outermost wrapper.
type Middleware struct {
	Name string
}

// Descriptor is the immutable, defensively-copied snapshot of an Activity's
// declarative configuration. It is introspectable without executing the
// Activity's handler.
type Descriptor struct {
	// Name is the Activity's declared, non-empty name.
	Name string
	// Description is the Activity's optional human-readable description,
	// set with Describe. It is empty if Describe was never applied.
	Description string
	// Target is the opaque, target-defined tag the Activity was built
	// with (e.g. "web" or "cli"), as passed to New.
	Target string
	// Params is every declared Param and its target binding, in
	// declaration order.
	Params []ParamBinding
	// Middleware is every declared middleware's identity, in declaration
	// order — the first entry is the outermost wrapper.
	Middleware []Middleware
}

// Builder accumulates an Activity's declarative configuration as Options are
// applied to it. A target package constructs a Builder with New, applies the
// caller-supplied Options (and its own target-specific declarations, such as
// route or binding options) in declaration order, then calls Snapshot once
// the target-specific handler is known to obtain the public, immutable
// Descriptor. Builder itself is a construction-time type, not part of the
// introspectable public contract.
type Builder struct {
	name        string
	target      string
	description string
	params      []ParamBinding
	middleware  []Middleware
}

// New starts a Builder for an Activity named name, bound to target (an
// opaque, target-defined tag such as "web" or "cli"). Both must be
// non-empty; New panics otherwise — an Activity always has a non-empty name.
func New(name, target string) *Builder {
	name = strings.TrimSpace(name)
	target = strings.TrimSpace(target)
	if name == "" {
		panic("activity: name must not be empty")
	}
	if target == "" {
		panic("activity: target must not be empty")
	}
	return &Builder{name: name, target: target}
}

// ApplyWeb applies each Web-capable opt to b in order and returns b for
// chaining. A Web Activity constructor (see web.Activity) calls
// this once it has constructed b with New(name, "web") and recorded any of
// its own target-specific declarations.
func (b *Builder) ApplyWeb(opts ...WebOption) *Builder {
	for _, opt := range opts {
		opt.applyWeb(b)
	}
	return b
}

// ApplyCLI is the CLI-side counterpart of ApplyWeb.
func (b *Builder) ApplyCLI(opts ...CLIOption) *Builder {
	for _, opt := range opts {
		opt.applyCLI(b)
	}
	return b
}

// DeclareParam records d as bound to source.
//
// Re-declaring the same Param identity (see param.AnyDescriptor) with the
// same source is a no-op: it is deduplicated. Re-declaring the same Param
// identity with a different source is a conflicting binding. Declaring a
// distinct Param identity that shares another declared Param's external
// name is always a conflict too, even if their type and options happen to
// match. Both conflicts return a descriptive error; DeclareParam never
// panics.
func (b *Builder) DeclareParam(d param.AnyDescriptor, source string) error {
	name := d.Name()
	for _, existing := range b.params {
		if existing.Param == d {
			if existing.Source == source {
				return nil
			}
			return fmt.Errorf("activity %q: param %q is already bound to source %q, cannot also bind to %q",
				b.name, name, existing.Source, source)
		}
		if existing.Param.Name() == name {
			return fmt.Errorf("activity %q: external param name %q is already declared by a different param",
				b.name, name)
		}
	}
	b.params = append(b.params, ParamBinding{Param: d, Source: source})
	return nil
}

// DeclareMiddleware appends m to the Activity's ordered middleware
// descriptors.
func (b *Builder) DeclareMiddleware(m Middleware) {
	b.middleware = append(b.middleware, m)
}

// Snapshot returns the immutable, defensively-copied Descriptor for b's
// current configuration. Calling Snapshot does not execute a handler and can
// be repeated safely.
func (b *Builder) Snapshot() Descriptor {
	params := make([]ParamBinding, len(b.params))
	copy(params, b.params)
	middleware := make([]Middleware, len(b.middleware))
	copy(middleware, b.middleware)
	return Descriptor{
		Name:        b.name,
		Description: b.description,
		Target:      b.target,
		Params:      params,
		Middleware:  middleware,
	}
}
