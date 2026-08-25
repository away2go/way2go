package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/param"
)

// Group is a validated set of Web Activity Definitions, exposed as a plain
// http.Handler. There is no global mux or registry: build one with All and
// keep the value it returns.
type Group struct {
	mux  *http.ServeMux
	defs []Definition
}

// ServeHTTP implements http.Handler by dispatching to the matching
// Definition's route, exactly as a *http.ServeMux would.
func (g Group) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mux.ServeHTTP(w, r)
}

// Definitions returns a copy of the Definitions g was built from, in the
// order passed to All.
func (g Group) Definitions() []Definition {
	out := make([]Definition, len(g.defs))
	copy(out, g.defs)
	return out
}

// All validates definitions and, if they are all consistent, returns the
// resulting Group. It rejects:
//
//   - an empty or duplicate Activity name;
//   - a duplicate method/path pair across Definitions;
//
// All is the final validation boundary for the effective descriptor:
// definitions fail deterministically here, before any Group ever serves a
// request.
func All(definitions ...Definition) (Group, error) {
	mux := http.NewServeMux()
	names := make(map[string]struct{}, len(definitions))
	routes := make(map[string]struct{}, len(definitions))

	for _, d := range definitions {
		if strings.TrimSpace(d.descriptor.Name) == "" {
			return Group{}, errors.New("web: activity name must not be empty")
		}
		if _, dup := names[d.descriptor.Name]; dup {
			return Group{}, fmt.Errorf("web: duplicate activity name %q", d.descriptor.Name)
		}
		names[d.descriptor.Name] = struct{}{}

		key := d.descriptor.Method + " " + d.descriptor.Path
		if _, dup := routes[key]; dup {
			return Group{}, fmt.Errorf("web: duplicate route %s", key)
		}
		routes[key] = struct{}{}

		mux.Handle(key, buildHandler(d))
	}

	return Group{mux: mux, defs: append([]Definition(nil), definitions...)}, nil
}

// buildHandler returns the http.HandlerFunc that serves d's route: resolve
// every declared Param's raw input, prepare (parse and validate) them all
// before any user middleware or handler runs, run the Activity's
// middleware-wrapped handler under a selective panic-recovery boundary, and
// commit the resulting Response.
func buildHandler(d Definition) http.HandlerFunc {
	bindings := d.descriptor.Params
	descriptors := make([]param.AnyDescriptor, len(bindings))
	for i, pb := range bindings {
		descriptors[i] = pb.Param
	}

	return func(w http.ResponseWriter, r *http.Request) {
		raw, err := resolveRaw(r, bindings)
		if err != nil {
			writeInputError(w, err)
			return
		}

		values, err := param.Prepare(descriptors, raw)
		if err != nil {
			// Missing/invalid declared input: param.MissingValueError,
			// param.ParseError or param.ValidationError. Ordinary input
			// error, mapped to HTTP 400, before any middleware or handler
			// runs.
			writeInputError(w, err)
			return
		}

		ctx := newContext(param.NewContext(r.Context(), values), r)
		resp := execute(d.handler, ctx)
		resp.writeTo(w)
	}
}

// resolveRaw resolves one param.RawValue per binding from r, according to
// its declared source ("query" or "form"). Presence and value stay
// distinct throughout: an absent query/form key yields param.RawValue{}
// (Present: false); an explicitly supplied empty value yields
// Present: true with an empty Value. A repeated query key or form field only
// ever contributes its first value — see FromQuery and FromForm's doc
// comments for the caller-facing statement of this limitation.
func resolveRaw(r *http.Request, bindings []activity.ParamBinding) (map[param.AnyDescriptor]param.RawValue, error) {
	raw := make(map[param.AnyDescriptor]param.RawValue, len(bindings))

	var query map[string][]string
	var formParsed bool

	for _, pb := range bindings {
		switch pb.Source {
		case "query":
			if query == nil {
				query = r.URL.Query()
			}
			if vals, ok := query[pb.Param.Name()]; ok {
				// Only the first value of a repeated key is used; see
				// FromQuery's doc comment.
				raw[pb.Param] = param.RawValue{Value: vals[0], Present: true}
			} else {
				raw[pb.Param] = param.RawValue{}
			}
		case "form":
			if !formParsed {
				if err := r.ParseForm(); err != nil {
					return nil, fmt.Errorf("web: parsing form: %w", err)
				}
				formParsed = true
			}
			if vals, ok := r.PostForm[pb.Param.Name()]; ok {
				// Only the first value of a repeated field is used; see
				// FromForm's doc comment.
				raw[pb.Param] = param.RawValue{Value: vals[0], Present: true}
			} else {
				raw[pb.Param] = param.RawValue{}
			}
		default:
			return nil, fmt.Errorf("web: param %q: unsupported binding source %q", pb.Param.Name(), pb.Source)
		}
	}

	return raw, nil
}

func writeInputError(w http.ResponseWriter, err error) {
	Render(http.StatusBadRequest, Text(err.Error())).writeTo(w)
}

// execute runs h(ctx) under a recovery boundary that recovers only a
// panic value satisfying the Way2Go activity.ProgrammerError contract
// (such as param.Read's undeclared-read panic), mapped to HTTP 500. Every
// other panic — a non-error value, or an error that does not implement
// activity.ProgrammerError, including the ordinary input errors Prepare
// itself returns — is re-panicked unchanged: it is never swallowed or
// mislabeled as a Param error.
func execute(h HandlerFunc, ctx Context) (resp Response) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		err, ok := r.(error)
		if !ok {
			panic(r)
		}
		var pe activity.ProgrammerError
		if !errors.As(err, &pe) {
			panic(r)
		}
		resp = Render(http.StatusInternalServerError, Text(pe.Error()))
	}()
	return h(ctx)
}
