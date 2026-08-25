package activity

import "fmt"

// Wrapper is the shape every target's own handler-wrapping function
// follows: given the next handler in the chain, it returns a new handler
// that runs around it. Target packages (web and cli) define their
// own concrete handler type H (a Web handler, a CLI handler); Wrapper[H]
// and Chain are the shared, generic composition primitive both targets
// reuse so middleware declaration order stays deterministic no matter which
// target is doing the wrapping.
type Wrapper[H any] func(next H) H

// Chain composes wrappers around handler in declaration order: wrappers[0]
// — the first declared middleware — ends up outermost. Chain(h, a, b)
// behaves as a(b(h)): when the result is finally invoked, a runs first,
// then b, then h.
func Chain[H any](handler H, wrappers ...Wrapper[H]) H {
	for i := len(wrappers) - 1; i >= 0; i-- {
		handler = wrappers[i](handler)
	}
	return handler
}

// wrapperCarrier is implemented by every Option value produced by
// NewMiddleware, NewWebMiddleware or NewCLIMiddleware. It hands back a
// target's Wrapper[H] as an opaque any because neither Web's handler type
// nor CLI's handler type exists in this package; WebWrapper and CLIWrapper
// recover the concrete type for the caller's own H — exactly the H the
// target package itself defines and will call with. Options that carry no
// wrapper (such as Describe) do not implement wrapperCarrier at all, so the
// type assertion in WebWrapper/CLIWrapper simply reports ok == false for
// them.
type wrapperCarrier interface {
	webWrapper() any
	cliWrapper() any
}

// WebWrapper extracts the Web-target Wrapper[H] carried by opt, if any. It
// reports ok == false for an Option that carries no Web wrapper (Describe,
// or a middleware built with NewCLIMiddleware) or whose stored wrapper was
// built for a different handler type than H.
func WebWrapper[H any](opt WebOption) (w Wrapper[H], ok bool) {
	carrier, isCarrier := opt.(wrapperCarrier)
	if !isCarrier {
		return nil, false
	}
	raw := carrier.webWrapper()
	if raw == nil {
		return nil, false
	}
	w, ok = raw.(Wrapper[H])
	return w, ok
}

// CLIWrapper is the CLI-side counterpart of WebWrapper.
func CLIWrapper[H any](opt CLIOption) (w Wrapper[H], ok bool) {
	carrier, isCarrier := opt.(wrapperCarrier)
	if !isCarrier {
		return nil, false
	}
	raw := carrier.cliWrapper()
	if raw == nil {
		return nil, false
	}
	w, ok = raw.(Wrapper[H])
	return w, ok
}

// declareMiddleware records mw and each of params' bindings on b, reusing
// DeclareParam/DeclareMiddleware so a middleware-contributed Param goes
// through exactly the same identity, dedup and conflict rules as a Param
// bound directly at the Activity call site.
//
// Options are applied at Activity construction time (ApplyWeb/ApplyCLI),
// typically from a package-level var initializer, where there is no error
// return available to the caller. Failing fast with a panic here gives the
// same "fails deterministically before serving a Web request or executing a
// CLI Activity" guarantee registration validation gives explicit bindings,
// just earlier — at process init instead of at first traffic.
func declareMiddleware(b *Builder, mw Middleware, params []ParamBinding) {
	b.DeclareMiddleware(mw)
	for _, p := range params {
		if err := b.DeclareParam(p.Param, p.Source); err != nil {
			panic(fmt.Sprintf("activity: middleware %q: %v", mw.Name, err))
		}
	}
}

// NewWebMiddleware builds a Web-only middleware Option: it contributes the
// Middleware{Name: name} descriptor and params to whichever Web Builder it
// is applied to, and carries wrap for later retrieval with WebWrapper.
//
// Its return type, WebOption, has no applyCLI method, so passing the result
// where a CLIOption (cli.Option) is required is a compile error, not a
// runtime registration failure — see the negative compile fixtures in
// testdata/crosstarget.
func NewWebMiddleware[H any](name string, wrap Wrapper[H], params ...ParamBinding) WebOption {
	return webMiddleware[H]{mw: Middleware{Name: name}, wrap: wrap, params: params}
}

type webMiddleware[H any] struct {
	mw     Middleware
	wrap   Wrapper[H]
	params []ParamBinding
}

func (m webMiddleware[H]) applyWeb(b *Builder) { declareMiddleware(b, m.mw, m.params) }
func (m webMiddleware[H]) webWrapper() any     { return Wrapper[H](m.wrap) }
func (m webMiddleware[H]) cliWrapper() any     { return nil }

// NewCLIMiddleware is the CLI-side counterpart of NewWebMiddleware.
func NewCLIMiddleware[H any](name string, wrap Wrapper[H], params ...ParamBinding) CLIOption {
	return cliMiddleware[H]{mw: Middleware{Name: name}, wrap: wrap, params: params}
}

type cliMiddleware[H any] struct {
	mw     Middleware
	wrap   Wrapper[H]
	params []ParamBinding
}

func (m cliMiddleware[H]) applyCLI(b *Builder) { declareMiddleware(b, m.mw, m.params) }
func (m cliMiddleware[H]) webWrapper() any     { return nil }
func (m cliMiddleware[H]) cliWrapper() any     { return Wrapper[H](m.wrap) }

// WebSpec bundles what a portable middleware built with NewMiddleware
// contributes specifically to the Web target: the Wrapper it carries there
// and the ParamBindings (with their Web-side source, e.g. "query") it
// declares when applied to a Web Activity's Builder.
type WebSpec[H any] struct {
	Wrap   Wrapper[H]
	Params []ParamBinding
}

// CLISpec is the CLI-side counterpart of WebSpec: the Wrapper and
// ParamBindings (with their CLI-side source, e.g. "flag") a portable
// middleware contributes when applied to a CLI Activity's Builder.
type CLISpec[H any] struct {
	Wrap   Wrapper[H]
	Params []ParamBinding
}

// NewMiddleware builds one middleware Option usable on both Web and CLI
// Activities. It contributes a single logical Middleware{Name: name}
// descriptor on whichever target Builder it is applied to, together with
// that target's own ParamBindings from web/cli — which is what lets one
// Param identity (e.g. a package-level Limit descriptor) be bound to a Web
// query parameter under the same middleware that binds it to a CLI flag —
// and carries that target's own wrapper for later retrieval with
// WebWrapper/CLIWrapper.
//
// The two handler type parameters WH and CH are independent: WH is the
// target's own Web handler type, CH its own CLI handler type. A middleware
// author who only wants to support one target uses NewWebMiddleware or
// NewCLIMiddleware instead, whose return type structurally cannot satisfy
// the other target's option contract.
func NewMiddleware[WH, CH any](name string, web WebSpec[WH], cli CLISpec[CH]) Portable {
	mw := Middleware{Name: name}
	return portableMiddleware[WH, CH]{
		mw:      mw,
		webWrap: web.Wrap,
		webPar:  web.Params,
		cliWrap: cli.Wrap,
		cliPar:  cli.Params,
	}
}

type portableMiddleware[WH, CH any] struct {
	mw      Middleware
	webWrap Wrapper[WH]
	webPar  []ParamBinding
	cliWrap Wrapper[CH]
	cliPar  []ParamBinding
}

func (m portableMiddleware[WH, CH]) applyWeb(b *Builder) { declareMiddleware(b, m.mw, m.webPar) }
func (m portableMiddleware[WH, CH]) applyCLI(b *Builder) { declareMiddleware(b, m.mw, m.cliPar) }
func (m portableMiddleware[WH, CH]) webWrapper() any     { return Wrapper[WH](m.webWrap) }
func (m portableMiddleware[WH, CH]) cliWrapper() any     { return Wrapper[CH](m.cliWrap) }
