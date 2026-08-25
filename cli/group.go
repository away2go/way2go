// group.go implements Node (the sealed CLI command-tree element interface),
// Group and App: nested command groups, the App root, and its Execute/Run
// execution entry points. See option.go's package doc comment for the
// package's overall contract; Way2Go owns the complete command tree, and
// Cobra is an internal implementation detail no exported declaration here
// ever leaks.

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/away2go/way2go/input"
	"github.com/away2go/way2go/output"
)

// Node is a member of a CLI command tree: either a leaf Definition (see
// Activity) or a group returned by Group. Its methods are unexported, so —
// exactly like activity.WebOption/CLIOption — the only values that satisfy
// it are the ones this package itself produces; no external package can
// implement Node.
type Node interface {
	name() string
	buildCommand(result *execResult) *cobra.Command
}

// groupNode is the Node implementation Group returns: a named, non-leaf
// command with its own children, which may themselves be further groups or
// Activity leaves.
type groupNode struct {
	group    string
	children []Node
}

func (g groupNode) name() string { return g.group }

func (g groupNode) buildCommand(result *execResult) *cobra.Command {
	cmd := &cobra.Command{
		Use:           g.group,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	for _, child := range g.children {
		cmd.AddCommand(child.buildCommand(result))
	}
	return cmd
}

// Group declares a named, nested command group. Its children may be further
// Groups or Activity leaves (see Activity) in any combination — only
// Activities are ever leaves. Group panics if name is empty, or if two
// children share a name: both are registration-time conflicts, detected
// deterministically when the tree is declared rather than when it is later
// executed.
func Group(name string, children ...Node) Node {
	name = trimmedNonEmpty("cli: group name must not be empty", name)
	rejectDuplicateNames(name, children)
	return groupNode{group: name, children: children}
}

// App is the root of a Way2Go CLI command tree, built by All. It exposes no
// Cobra configuration surface: the only operations are Execute, for
// programmatic/test-driven execution with explicit args and output sinks,
// and Run, the real-process convenience entry point.
type App struct {
	nodes []Node
}

// All declares the root command group of a CLI command tree. As with Group,
// two top-level children sharing a name is a registration-time panic.
func All(children ...Node) App {
	rejectDuplicateNames("root", children)
	return App{nodes: children}
}

func rejectDuplicateNames(parent string, children []Node) {
	seen := make(map[string]bool, len(children))
	for _, c := range children {
		n := c.name()
		if seen[n] {
			panic(fmt.Sprintf("cli: duplicate command or group name %q under %q", n, parent))
		}
		seen[n] = true
	}
}

func trimmedNonEmpty(panicMsg string, s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		panic(panicMsg)
	}
	return trimmed
}

// execResult carries the single Outcome produced by whichever leaf command a
// App.Execute call ends up dispatching to, along with the name of the
// Activity that produced it (needed to prefix a cli.Error(err) message, see
// Execute). A fresh execResult, and a fresh *cobra.Command tree built with
// it, is created for every Execute call, so concurrent Execute calls on the
// same App never share mutable state.
type execResult struct {
	outcome      Outcome
	hasOutcome   bool
	activityName string
}

// Execute parses args against a's command tree and dispatches to the
// resolved Activity, exactly as a real CLI invocation would, but under full
// caller control: ctx seeds the execution context (nil is treated as
// context.Background()), in is the stdin source and out and err are the
// stdout/stderr sinks handlers observe through the input and output
// packages respectively, and the returned int is the process exit code
// this execution maps to. Execute never calls os.Exit; that is Run's job.
//
// A nil out or err is normalized to io.Discard before Execute uses it for
// anything — including Execute's own direct writes, not just the
// output.NewContext-mediated writes a handler makes — so a caller passing a
// literal nil for either never causes a nil-writer panic.
//
// Every declared Param is resolved and validated — via param.Prepare — and
// found valid before any user middleware or the handler runs. Exit code
// mapping is fixed:
//
//   - cli.OK() maps to 0;
//   - cli.NOK() maps to 1, silently — the framework prints nothing;
//   - cli.Error(err) also maps to 1, but first prints err to the normalized
//     err writer exactly once, prefixed with the dispatched Activity's name
//     (e.g. "generate: failed to seal Batch: ...\n");
//   - a flag, argument, Param or marked interactive input error (missing,
//     unparsable or validator-rejected value; an unmatched command; too many
//     positional arguments) maps to 2;
//   - a recovered Way2Go programmer error (see activity.ProgrammerError)
//     maps to 1. An unrelated panic is re-panicked out of Execute, not
//     recovered.
//
// Execute builds a brand new *cobra.Command tree from a's declarative Node
// tree on every call: no Cobra command or pflag.FlagSet, and therefore no
// flag "Changed" state, is ever reused or shared across calls, which is what
// keeps concurrent Execute calls — and concurrent test executions using
// independently injected out/err buffers — from cross-contaminating each
// other.
func (a App) Execute(ctx context.Context, args []string, in io.Reader, out, err io.Writer) int {
	if ctx == nil {
		ctx = context.Background()
	}
	if out == nil {
		out = io.Discard
	}
	if err == nil {
		err = io.Discard
	}
	result := &execResult{}
	root := &cobra.Command{
		Use:           programName(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	for _, n := range a.nodes {
		root.AddCommand(n.buildCommand(result))
	}
	root.SetOut(out)
	root.SetErr(err)
	root.SetArgs(args)

	execCtx := output.NewContext(ctx, out, err)
	execCtx = input.NewContext(execCtx, in)
	execErr := root.ExecuteContext(execCtx)
	if execErr != nil {
		var pe *programmerError
		if errors.As(execErr, &pe) {
			fmt.Fprintln(err, execErr)
			return 1
		}
		fmt.Fprintln(err, execErr)
		return 2
	}

	if result.hasOutcome && !result.outcome.ok {
		if oerr := result.outcome.error(); oerr != nil {
			if isInputFailure(oerr) {
				fmt.Fprintf(err, "%s: %v\n", result.activityName, oerr)
				return 2
			}
			fmt.Fprintf(err, "%s: %v\n", result.activityName, oerr)
		}
		return 1
	}
	return 0
}

// Run is the real-process convenience entry point: it executes a against
// os.Args[1:] with os.Stdin/os.Stdout/os.Stderr and terminates the process
// with the resulting exit code via os.Exit. Production mains call Run;
// tests call Execute directly to keep control of args, input, output and
// the process.
//
// Run watches for os.Interrupt (SIGINT, e.g. Ctrl-C) and SIGTERM with
// signal.Notify on an explicit channel — rather than signal.NotifyContext,
// which does not expose which signal fired — so it can tell the two apart.
// It runs Execute in its own goroutine and races that goroutine's
// completion against the signal channel:
//
//   - if Execute finishes first, Run exits with Execute's own returned code
//     (0, 1 or 2), unchanged;
//   - if a signal arrives first, Run cancels Execute's context and closes its
//     active input relay. That interrupts the handler's ctx-bound terminal or
//     pipe read, then Run waits for Execute to return before exiting with 130
//     for SIGINT or 143 for SIGTERM.
//
// Execute itself deliberately accepts arbitrary injected readers. A reader
// that cannot be closed cannot in general be interrupted while blocked; it
// still observes cancellation before and after each read. Run avoids that
// limitation for process stdin with an internal, demand-driven input relay
// (see stdinRelay): its read end is always closeable even where closing an
// inherited os.Stdin does not reliably interrupt an in-flight operating-
// system read. A Read call already in flight against the real os.Stdin when
// Close happens leaks until it returns on its own (new input, EOF, or a read
// error) or the process exits, which happens immediately after Execute
// returns on a signal, so it cannot outlive Run.
//
// Run also marks ctx as interactive (input.MarkInteractive) before starting
// Execute: this is the one real-process entry point where os.Stdin genuinely
// is the process's own standard input, which is what lets way2go/prompt's
// ReadSecret decide it is safe to attempt a direct, non-echoing read against
// the real terminal file descriptor instead of ctx's relayed source. See
// stdinRelay's doc comment for why that direct read never races the relay.
func (a App) Run() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = input.MarkInteractive(ctx)
	in := newStdinRelay()
	defer in.Close()

	done := make(chan int, 1)
	go func() {
		done <- a.Execute(ctx, os.Args[1:], in, os.Stdout, os.Stderr)
	}()

	select {
	case code := <-done:
		os.Exit(code)
	case sig := <-sigCh:
		cancel()
		_ = in.Close()
		<-done
		switch sig {
		case syscall.SIGTERM:
			os.Exit(143)
		default: // os.Interrupt (SIGINT)
			os.Exit(130)
		}
	}
}

// stdinRelay is Run's demand-driven adapter over process stdin. Unlike a
// free-running io.Copy, it issues a Read against the real os.Stdin only the
// instant its own Read method is called, and never reads ahead
// speculatively. This is what lets way2go/prompt's non-echoing secret-input
// path (ReadSecret) safely read os.Stdin directly — via golang.org/x/term,
// which needs the real terminal file descriptor, not ctx's relayed/buffered
// source — in between ordinary ReadLine calls, without racing this relay for
// the same bytes: stdinRelay is guaranteed to be idle, parked on a channel
// receive rather than blocked inside a live os.Stdin.Read call, whenever no
// ReadLine call is in flight, because a CLI handler only ever has one read
// of any kind outstanding at a time. An eagerly free-running relay (the
// previous io.Copy-based implementation) cannot offer that guarantee: it
// always tries to stay one Read ahead of its consumer, so it is typically
// already blocked inside os.Stdin.Read — with no portable way to hand that
// read off — exactly when a secret prompt would need the fd.
//
// Close makes any Read call currently or later blocked on the relay return
// io.ErrClosedPipe immediately, mirroring io.PipeReader's close behavior; it
// does not and cannot interrupt an os.Stdin.Read call already in flight
// inside loop — the same limitation package input's ReadLine documents for
// any arbitrary reader. A Read call already in flight when Close happens
// leaks until it returns on its own (new input, EOF, or a read error) or the
// process exits, exactly as the previous io.Copy-based relay's goroutine
// could leak after a signal.
type stdinRelay struct {
	pull      chan []byte
	result    chan stdinReadResult
	closed    chan struct{}
	closeOnce sync.Once
}

// stdinReadResult carries one os.Stdin.Read call's outcome back to the
// stdinRelay.Read call that requested it.
type stdinReadResult struct {
	n   int
	err error
}

// newStdinRelay starts a stdinRelay's background loop and returns it.
func newStdinRelay() *stdinRelay {
	r := &stdinRelay{
		pull:   make(chan []byte),
		result: make(chan stdinReadResult),
		closed: make(chan struct{}),
	}
	go r.loop()
	return r
}

// loop services pull requests one at a time, each with its own single
// os.Stdin.Read call, and exits once closed is closed — except that a Read
// call already in flight when that happens cannot itself be interrupted;
// see the type doc comment.
func (r *stdinRelay) loop() {
	for {
		select {
		case buf := <-r.pull:
			n, err := os.Stdin.Read(buf)
			select {
			case r.result <- stdinReadResult{n, err}:
			case <-r.closed:
				return
			}
		case <-r.closed:
			return
		}
	}
}

// Read implements io.Reader by forwarding p to the relay loop and returning
// exactly what its one corresponding os.Stdin.Read(p) call produced.
func (r *stdinRelay) Read(p []byte) (int, error) {
	select {
	case r.pull <- p:
	case <-r.closed:
		return 0, io.ErrClosedPipe
	}
	select {
	case res := <-r.result:
		return res.n, res.err
	case <-r.closed:
		return 0, io.ErrClosedPipe
	}
}

// Close makes every blocked or future Read return io.ErrClosedPipe. It is
// safe to call more than once.
func (r *stdinRelay) Close() error {
	r.closeOnce.Do(func() { close(r.closed) })
	return nil
}

// programName derives the root command's displayed program name from the
// executing process, so a's internal Cobra root command needs no public
// configuration surface for it.
func programName() string {
	if len(os.Args) == 0 {
		return "cli"
	}
	return filepath.Base(os.Args[0])
}
