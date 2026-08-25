package cli_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/cli"
	"github.com/away2go/way2go/input"
	"github.com/away2go/way2go/output"
	"github.com/away2go/way2go/param"
)

// TestFlagsResolveDefaultAndValidate proves API contract: FromFlag
// binds a Param as a long-form flag using the Param's own required/default
// and typed-validation semantics, all resolved before the handler runs.
func TestFlagsResolveDefaultAndValidate(t *testing.T) {
	limit := param.Int("limit", param.Default(10), param.Validate(func(v int) error {
		if v < 0 {
			return fmt.Errorf("limit must be >= 0")
		}
		return nil
	}))

	var got int
	act := cli.Activity("list", func(ctx context.Context) cli.Outcome {
		got = param.Read(ctx, limit)
		return cli.OK()
	}, cli.FromFlag(limit))
	app := cli.All(act)

	var out, errBuf bytes.Buffer
	if code := app.Execute(context.Background(), []string{"list", "--limit=5"}, nil, &out, &errBuf); code != 0 {
		t.Fatalf("Execute(--limit=5) code = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if got != 5 {
		t.Fatalf("got = %d, want 5", got)
	}

	out.Reset()
	errBuf.Reset()
	if code := app.Execute(context.Background(), []string{"list"}, nil, &out, &errBuf); code != 0 {
		t.Fatalf("Execute() (default) code = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if got != 10 {
		t.Fatalf("got = %d, want default 10", got)
	}

	out.Reset()
	errBuf.Reset()
	if code := app.Execute(context.Background(), []string{"list", "--limit=-1"}, nil, &out, &errBuf); code != 2 {
		t.Fatalf("Execute(--limit=-1) code = %d, want 2 (validation failure); stderr=%s", code, errBuf.String())
	}
}

// TestBoolFlagAcceptsBareForm proves a param.KindBool flag registers as a
// genuine pflag/Cobra bool flag: "--verbose" with no "=true" must set the
// flag true, exactly as every other pflag/Cobra-based CLI's bool flags
// already behave. Before this test's fix, every flag — regardless of Kind —
// was registered with cmd.Flags().String, which rejects the bare form with
// "flag needs an argument: --verbose" (exit code 2).
func TestBoolFlagAcceptsBareForm(t *testing.T) {
	verbose := param.Bool("verbose", param.Default(false))

	var got bool
	act := cli.Activity("run", func(ctx context.Context) cli.Outcome {
		got = param.Read(ctx, verbose)
		return cli.OK()
	}, cli.FromFlag(verbose))
	app := cli.All(act)

	var out, errBuf bytes.Buffer
	if code := app.Execute(context.Background(), []string{"run", "--verbose"}, nil, &out, &errBuf); code != 0 {
		t.Fatalf("Execute(--verbose) code = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if !got {
		t.Fatal("got = false, want true for bare --verbose")
	}

	out.Reset()
	errBuf.Reset()
	got = true
	if code := app.Execute(context.Background(), []string{"run"}, nil, &out, &errBuf); code != 0 {
		t.Fatalf("Execute() (default) code = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if got {
		t.Fatal("got = true, want default false when --verbose is omitted")
	}

	out.Reset()
	errBuf.Reset()
	if code := app.Execute(context.Background(), []string{"run", "--verbose=false"}, nil, &out, &errBuf); code != 0 {
		t.Fatalf("Execute(--verbose=false) code = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if got {
		t.Fatal("got = true, want false for --verbose=false")
	}
}

// TestPositionalArgsResolveInDeclarationOrder proves API contract
// #3's happy path: fixed positional args bind in declaration order.
func TestPositionalArgsResolveInDeclarationOrder(t *testing.T) {
	first := param.String("first")
	second := param.String("second", param.Default("fallback"))

	var gotFirst, gotSecond string
	act := cli.Activity("greet", func(ctx context.Context) cli.Outcome {
		gotFirst = param.Read(ctx, first)
		gotSecond = param.Read(ctx, second)
		return cli.OK()
	}, cli.FromArg(first, second))
	app := cli.All(act)

	var out, errBuf bytes.Buffer
	if code := app.Execute(context.Background(), []string{"greet", "alice", "bob"}, nil, &out, &errBuf); code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if gotFirst != "alice" || gotSecond != "bob" {
		t.Fatalf("got (%q, %q), want (%q, %q)", gotFirst, gotSecond, "alice", "bob")
	}

	out.Reset()
	errBuf.Reset()
	gotFirst, gotSecond = "", ""
	if code := app.Execute(context.Background(), []string{"greet", "alice"}, nil, &out, &errBuf); code != 0 {
		t.Fatalf("code = %d, want 0 (second optional); stderr=%s", code, errBuf.String())
	}
	if gotFirst != "alice" || gotSecond != "fallback" {
		t.Fatalf("got (%q, %q), want (%q, %q)", gotFirst, gotSecond, "alice", "fallback")
	}
}

// TestExtraPositionalArgumentFails proves API contract: supplying
// more positional arguments than declared is an input error (exit code 2),
// and v1 has no variadic positional mode to absorb the extra argument.
func TestExtraPositionalArgumentFails(t *testing.T) {
	one := param.String("one")
	act := cli.Activity("echo", func(ctx context.Context) cli.Outcome { return cli.OK() }, cli.FromArg(one))
	app := cli.All(act)

	var out, errBuf bytes.Buffer
	code := app.Execute(context.Background(), []string{"echo", "a", "b"}, nil, &out, &errBuf)
	if code != 2 {
		t.Fatalf("code = %d, want 2 (extra argument); stderr=%s", code, errBuf.String())
	}
}

// TestMissingRequiredFlagExitsTwo proves the missing-required-value case of
// API contract: a Param input error maps to exit code 2, before
// the handler ever runs.
func TestMissingRequiredFlagExitsTwo(t *testing.T) {
	name := param.String("name")
	executed := false
	act := cli.Activity("greet", func(ctx context.Context) cli.Outcome {
		executed = true
		return cli.OK()
	}, cli.FromFlag(name))
	app := cli.All(act)

	var out, errBuf bytes.Buffer
	code := app.Execute(context.Background(), []string{"greet"}, nil, &out, &errBuf)
	if code != 2 {
		t.Fatalf("code = %d, want 2; stderr=%s", code, errBuf.String())
	}
	if executed {
		t.Fatal("handler must not run when a required Param is missing")
	}
}

// TestOKAndNOKExitCodes proves the OK()/NOK() half of API contract
// #7.
func TestOKAndNOKExitCodes(t *testing.T) {
	ok := cli.Activity("ok", func(ctx context.Context) cli.Outcome { return cli.OK() })
	nok := cli.Activity("nok", func(ctx context.Context) cli.Outcome { return cli.NOK() })
	app := cli.All(ok, nok)

	var out, errBuf bytes.Buffer
	if code := app.Execute(context.Background(), []string{"ok"}, nil, &out, &errBuf); code != 0 {
		t.Fatalf("ok code = %d, want 0", code)
	}
	if code := app.Execute(context.Background(), []string{"nok"}, nil, &out, &errBuf); code != 1 {
		t.Fatalf("nok code = %d, want 1", code)
	}
}

// TestRecoveredProgrammerErrorExitsOne proves API contract: a
// Way2Go programmer error — here, reading a Param the Activity never
// declared — is selectively recovered around Activity execution and mapped
// to exit code 1, not 2 (it must not be mislabeled as a Param input error).
func TestRecoveredProgrammerErrorExitsOne(t *testing.T) {
	stray := param.String("stray") // deliberately never bound to this Activity
	act := cli.Activity("boom", func(ctx context.Context) cli.Outcome {
		param.Read(ctx, stray)
		return cli.OK()
	})
	app := cli.All(act)

	var out, errBuf bytes.Buffer
	code := app.Execute(context.Background(), []string{"boom"}, nil, &out, &errBuf)
	if code != 1 {
		t.Fatalf("code = %d, want 1 (recovered programmer error); stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "stray") {
		t.Fatalf("stderr = %q, want it to mention the undeclared param", errBuf.String())
	}
}

// TestUnrelatedPanicRepanics proves the other half of API contract
// #8: a panic that is not a Way2Go programmer error must propagate out of
// Execute rather than being recovered and mislabeled.
func TestUnrelatedPanicRepanics(t *testing.T) {
	act := cli.Activity("boom", func(ctx context.Context) cli.Outcome {
		panic("unrelated failure, not a Way2Go programmer error")
	})
	app := cli.All(act)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected the unrelated panic to propagate out of Execute")
		}
		if !strings.Contains(fmt.Sprint(r), "unrelated failure") {
			t.Fatalf("recovered value = %v, want it to be the original panic", r)
		}
	}()
	var out, errBuf bytes.Buffer
	app.Execute(context.Background(), []string{"boom"}, nil, &out, &errBuf)
}

// TestNestedGroupsDispatch proves API contract: cli.Group nests,
// with Activities only as leaves, and cli.All forms a working root.
func TestNestedGroupsDispatch(t *testing.T) {
	var ran bool
	leaf := cli.Activity("run", func(ctx context.Context) cli.Outcome {
		ran = true
		return cli.OK()
	})
	app := cli.All(cli.Group("job", cli.Group("sub", leaf)))

	var out, errBuf bytes.Buffer
	code := app.Execute(context.Background(), []string{"job", "sub", "run"}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if !ran {
		t.Fatal("expected the nested leaf Activity to run")
	}
}

// TestDuplicateActivityNamesUnderSameGroupPanics and
// TestDuplicateGroupNamesUnderSameParentPanics prove API contract
// #4's negative half plus the design's "duplicate CLI group/command paths"
// registration rule.
func TestDuplicateActivityNamesUnderSameGroupPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for duplicate Activity name under one group")
		}
	}()
	a := cli.Activity("dup", noop)
	b := cli.Activity("dup", noop)
	cli.All(a, b)
}

func TestDuplicateGroupNamesUnderSameParentPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for duplicate group name under one parent")
		}
	}()
	g1 := cli.Group("x", cli.Activity("a", noop))
	g2 := cli.Group("x", cli.Activity("b", noop))
	cli.All(g1, g2)
}

func TestEmptyGroupNamePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for empty group name")
		}
	}()
	cli.Group("   ")
}

// TestMiddlewareOrderAndPreMiddlewareParamResolution proves end to end that
// middleware runs in declaration order (first
// declared outermost), and every declared Param is already resolved and
// readable from within the first (outermost) middleware — i.e. before any
// user middleware or the handler runs.
func TestMiddlewareOrderAndPreMiddlewareParamResolution(t *testing.T) {
	var trace []string
	limit := param.Int("limit", param.Default(1))

	auth := activity.NewCLIMiddleware[cli.HandlerFunc]("auth", func(next cli.HandlerFunc) cli.HandlerFunc {
		return func(ctx context.Context) cli.Outcome {
			trace = append(trace, "enter:auth")
			v := param.Read(ctx, limit) // proves resolution happened before this, the outermost middleware
			trace = append(trace, fmt.Sprintf("auth-saw-limit:%d", v))
			r := next(ctx)
			trace = append(trace, "exit:auth")
			return r
		}
	})
	audit := activity.NewCLIMiddleware[cli.HandlerFunc]("audit", func(next cli.HandlerFunc) cli.HandlerFunc {
		return func(ctx context.Context) cli.Outcome {
			trace = append(trace, "enter:audit")
			r := next(ctx)
			trace = append(trace, "exit:audit")
			return r
		}
	})

	act := cli.Activity("run", func(ctx context.Context) cli.Outcome {
		trace = append(trace, "handler")
		return cli.OK()
	}, cli.FromFlag(limit), auth, audit)

	d := act.Descriptor()
	if len(d.Middleware) != 2 || d.Middleware[0].Name != "auth" || d.Middleware[1].Name != "audit" {
		t.Fatalf("Middleware = %+v, want [auth audit] in declaration order", d.Middleware)
	}

	app := cli.All(act)
	var out, errBuf bytes.Buffer
	if code := app.Execute(context.Background(), []string{"run", "--limit=42"}, nil, &out, &errBuf); code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, errBuf.String())
	}

	want := []string{"enter:auth", "auth-saw-limit:42", "enter:audit", "handler", "exit:audit", "exit:auth"}
	if len(trace) != len(want) {
		t.Fatalf("trace = %v, want %v", trace, want)
	}
	for i := range want {
		if trace[i] != want[i] {
			t.Fatalf("trace = %v, want %v", trace, want)
		}
	}
}

// TestErrorOutcomePrintsOnceWithActivityPrefixAndExitsOne proves that
// cli.Error(err) maps to the same exit code as cli.NOK() (1), but
// — unlike NOK, which is silent — the framework itself prints err to
// stderr exactly once, prefixed with the dispatched Activity's name, and
// preserves err's wrapped chain rather than flattening it.
func TestErrorOutcomePrintsOnceWithActivityPrefixAndExitsOne(t *testing.T) {
	inner := fmt.Errorf("underlying cause")
	wrapped := fmt.Errorf("failed to seal Batch: %w", inner)
	act := cli.Activity("generate", func(ctx context.Context) cli.Outcome {
		return cli.Error(wrapped)
	})
	app := cli.All(act)

	var out, errBuf bytes.Buffer
	code := app.Execute(context.Background(), []string{"generate"}, nil, &out, &errBuf)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	got := errBuf.String()
	want := "generate: failed to seal Batch: underlying cause\n"
	if got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if !errors.Is(wrapped, inner) {
		t.Fatal("sanity: wrapped should still Is-match inner")
	}
}

// TestNOKPrintsNothing proves NOK's silence is unaffected by the addition
// of cli.Error: a handler returning plain NOK() must still produce no
// framework-authored stderr output.
func TestNOKPrintsNothing(t *testing.T) {
	act := cli.Activity("nok", func(ctx context.Context) cli.Outcome { return cli.NOK() })
	app := cli.All(act)

	var out, errBuf bytes.Buffer
	code := app.Execute(context.Background(), []string{"nok"}, nil, &out, &errBuf)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if errBuf.String() != "" {
		t.Fatalf("stderr = %q, want empty", errBuf.String())
	}
}

func TestErrorRejectsNil(t *testing.T) {
	defer func() {
		if got := recover(); got == nil || !strings.Contains(fmt.Sprint(got), "non-nil") {
			t.Fatalf("panic = %v, want non-nil error diagnostic", got)
		}
	}()
	_ = cli.Error(nil)
}

func TestMarkedInteractiveInputFailureExitsTwo(t *testing.T) {
	act := cli.Activity("ask", func(context.Context) cli.Outcome {
		return cli.Error(markedInputError{err: errors.New("must contain a word")})
	})
	app := cli.All(act)

	var out, errBuf bytes.Buffer
	if code := app.Execute(context.Background(), []string{"ask"}, nil, &out, &errBuf); code != 2 {
		t.Fatalf("code = %d, want 2; stderr=%q", code, errBuf.String())
	}
	if got, want := errBuf.String(), "ask: must contain a word\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

type markedInputError struct{ err error }

func (e markedInputError) Error() string  { return e.err.Error() }
func (e markedInputError) Unwrap() error  { return e.err }
func (markedInputError) InputError() bool { return true }

// TestExecuteWithNilWritersDoesNotPanic proves that Execute must
// normalize nil out/err writers itself, not just rely on output.NewContext
// doing so internally for handler-side writes — Execute's own direct
// writes (the input-error path and the cli.Error single-print path) must
// not panic on a literal nil writer either.
func TestExecuteWithNilWritersDoesNotPanic(t *testing.T) {
	t.Run("input error with nil err writer", func(t *testing.T) {
		name := param.String("name")
		act := cli.Activity("greet", func(ctx context.Context) cli.Outcome { return cli.OK() }, cli.FromFlag(name))
		app := cli.All(act)

		var out bytes.Buffer
		code := app.Execute(context.Background(), []string{"greet"}, nil, &out, nil)
		if code != 2 {
			t.Fatalf("code = %d, want 2", code)
		}
	})

	t.Run("cli.Error with nil out and err writers", func(t *testing.T) {
		act := cli.Activity("boom", func(ctx context.Context) cli.Outcome {
			return cli.Error(fmt.Errorf("kaboom"))
		})
		app := cli.All(act)

		code := app.Execute(context.Background(), []string{"boom"}, nil, nil, nil)
		if code != 1 {
			t.Fatalf("code = %d, want 1", code)
		}
	})
}

// TestConcurrentExecutionsDoNotCrossContaminateOutput proves that each
// App.Execute call's handler writes through the
// independently injected out buffer for that call, with no cross-talk
// between concurrent executions of the same App/Definition.
func TestConcurrentExecutionsDoNotCrossContaminateOutput(t *testing.T) {
	message := param.String("message")
	act := cli.Activity("say", func(ctx context.Context) cli.Outcome {
		output.Println(ctx, param.Read(ctx, message))
		return cli.OK()
	}, cli.FromFlag(message))
	app := cli.All(act)

	const n = 25
	var wg sync.WaitGroup
	results := make([]string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var out, errBuf bytes.Buffer
			msg := fmt.Sprintf("hello-%d", i)
			code := app.Execute(context.Background(), []string{"say", "--message=" + msg}, nil, &out, &errBuf)
			if code != 0 {
				t.Errorf("goroutine %d: code = %d, want 0; stderr=%s", i, code, errBuf.String())
			}
			results[i] = strings.TrimSpace(out.String())
		}(i)
	}
	wg.Wait()

	for i, got := range results {
		want := fmt.Sprintf("hello-%d", i)
		if got != want {
			t.Errorf("goroutine %d: stdout = %q, want %q", i, got, want)
		}
	}
}

// TestConcurrentExecutionsDoNotCrossContaminateInput proves that each
// App.Execute call's handler reads through the
// independently injected in reader for that call, via input.ReadLine, with
// no cross-talk between concurrent executions of the same App/Definition —
// the same guarantee TestConcurrentExecutionsDoNotCrossContaminateOutput
// proves for out/err, now proven for in.
func TestConcurrentExecutionsDoNotCrossContaminateInput(t *testing.T) {
	act := cli.Activity("echo", func(ctx context.Context) cli.Outcome {
		line, err := input.ReadLine(ctx)
		if err != nil {
			output.Errorln(ctx, err)
			return cli.NOK()
		}
		output.Println(ctx, line)
		return cli.OK()
	})
	app := cli.All(act)

	const n = 25
	var wg sync.WaitGroup
	results := make([]string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			in := strings.NewReader(fmt.Sprintf("input-%d\n", i))
			var out, errBuf bytes.Buffer
			code := app.Execute(context.Background(), []string{"echo"}, in, &out, &errBuf)
			if code != 0 {
				t.Errorf("goroutine %d: code = %d, want 0; stderr=%s", i, code, errBuf.String())
			}
			results[i] = strings.TrimSpace(out.String())
		}(i)
	}
	wg.Wait()

	for i, got := range results {
		want := fmt.Sprintf("input-%d", i)
		if got != want {
			t.Errorf("goroutine %d: stdout = %q, want %q", i, got, want)
		}
	}
}
