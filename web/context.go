package web

import (
	"context"
	"net/http"
)

// Context is a Web Activity handler's runtime view of one request. It is a
// Way2Go-owned type — not a type alias for *http.Request or
// context.Context — and deliberately exposes no writable
// http.ResponseWriter: a handler communicates its result solely through
// its returned Response.
//
// All declared Params are resolved, parsed and validated before Context is
// constructed (see buildHandler in group.go), so param.Read(ctx.Context(),
// descriptor) against the context.Context it exposes always observes the
// complete, validated Param view — for every declared Param, from the
// first user middleware onward.
type Context struct {
	ctx context.Context
	req *http.Request
}

// newContext builds a Context around req, carrying ctx (already prepared
// with the Activity's resolved param.Values via param.NewContext).
func newContext(ctx context.Context, req *http.Request) Context {
	return Context{ctx: ctx, req: req}
}

// Request returns the underlying *http.Request, for target-specific
// runtime information (headers, cookies, remote address, TLS state, the
// request's own context, ...) that is not a declared Param.
func (c Context) Request() *http.Request { return c.req }

// Context returns the request's context.Context, carrying the resolved
// param.Values readable with param.Read, derived from the original
// *http.Request's context (so deadlines and cancellation propagate).
func (c Context) Context() context.Context { return c.ctx }
