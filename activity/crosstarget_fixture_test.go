package activity_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestCrossTargetMiddlewareIsRejectedAtCompileTime drives the negative
// compile fixtures under testdata/crosstarget. Each fixture package is
// deliberately excluded from the module's own build (Go's tooling ignores
// any directory named "testdata" when expanding "./..."), so it never
// breaks `go build ./...` / `go test ./...`, but can still be built
// directly by path — which is exactly what this test does, asserting the
// build fails and fails for the right reason: a static-type mismatch
// between activity.WebOption and activity.CLIOption, not a runtime panic.
//
// This is the API contract proof: "passing [a target-specific
// middleware] to the other target fails compilation rather than
// registration or execution."
func TestCrossTargetMiddlewareIsRejectedAtCompileTime(t *testing.T) {
	cases := []struct {
		name       string
		dir        string
		wantInDiag []string
	}{
		{
			name: "CLI-only middleware rejected by a Web-shaped option acceptor",
			dir:  "testdata/crosstarget/cli_only_to_web",
			wantInDiag: []string{
				"cannot use auditCLIOnly",
				"activity.CLIOption does not implement activity.WebOption",
				"missing method applyWeb",
			},
		},
		{
			name: "Web-only middleware rejected by a CLI-shaped option acceptor",
			dir:  "testdata/crosstarget/web_only_to_cli",
			wantInDiag: []string{
				"cannot use csrfWebOnly",
				"activity.WebOption does not implement activity.CLIOption",
				"missing method applyCLI",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// go test's working directory is already this package's
			// directory (activity/), and the fixtures live under
			// activity/testdata/..., so no directory change is needed —
			// just build the fixture by its relative package path.
			cmd := exec.Command("go", "build", "./"+c.dir)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("expected %s to fail to compile, but `go build` succeeded:\n%s", c.dir, out)
			}
			diag := string(out)
			for _, want := range c.wantInDiag {
				if !strings.Contains(diag, want) {
					t.Errorf("compiler diagnostic missing %q; got:\n%s", want, diag)
				}
			}
			// Guard against the fixture accidentally failing for an
			// unrelated reason (e.g. a typo breaking every line): the
			// sanity-check assignments in each fixture must still be fine,
			// so the diagnostic must be scoped to the one rejected call.
			if strings.Count(diag, "cannot use") != 1 {
				t.Errorf("expected exactly one compile error (the cross-target call), got:\n%s", diag)
			}
		})
	}
}
