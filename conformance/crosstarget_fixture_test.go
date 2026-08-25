package conformance_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestTargetSpecificFixturesAreRejectedAtCompileTimeByTheOtherTarget drives
// the negative compile fixtures under testdata/crosstarget. Each fixture
// package is deliberately excluded from this module's own build ("testdata"
// directories are always ignored by "./..." expansion), so it never breaks
// `go build ./...` / `go test ./...`, but can still be built directly by
// path — which is exactly what this test does, following the same technique
// as activity/crosstarget_fixture_test.go's negative compile fixtures.
//
// A target-specific fixture is
// genuinely accepted by its own target's Activity constructor (each
// fixture's "sanity check" line, which must keep compiling), and rejected —
// by the Go compiler, not a runtime registration failure — when passed to
// the other target's Activity constructor.
func TestTargetSpecificFixturesAreRejectedAtCompileTimeByTheOtherTarget(t *testing.T) {
	cases := []struct {
		name       string
		dir        string
		wantInDiag []string
	}{
		{
			name: "Web-only middleware accepted by web.Activity, rejected by cli.Activity",
			dir:  "testdata/crosstarget/webonly_rejected_by_cli",
			wantInDiag: []string{
				"cannot use csrf",
				"activity.WebOption does not implement activity.CLIOption",
				"missing method applyCLI",
			},
		},
		{
			name: "CLI-only middleware accepted by cli.Activity, rejected by web.Activity",
			dir:  "testdata/crosstarget/clionly_rejected_by_web",
			wantInDiag: []string{
				"cannot use audit",
				"activity.CLIOption does not implement activity.WebOption",
				"missing method applyWeb",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// go test's working directory is already this package's own
			// directory (conformance/), and the fixtures live under
			// conformance/testdata/..., so no directory change is needed —
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
			// unrelated reason: exactly the one cross-target call must be
			// rejected, and the fixture's own sanity-check assignment
			// (proving the value is genuinely valid for its own target)
			// must still compile fine.
			if strings.Count(diag, "cannot use") != 1 {
				t.Errorf("expected exactly one compile error (the cross-target call), got:\n%s", diag)
			}
		})
	}
}
