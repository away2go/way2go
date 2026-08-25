package way2go_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestNoCobraOrPflagInExportedSurface checks that Cobra remains an internal
// CLI implementation detail: no Cobra or
// pflag type may appear in any exported Way2Go signature, alias, embedded
// field or return value.
//
// It shells out to `go doc -all` for cli and output — the same view of the
// exported surface a consumer of these packages sees — rather than
// re-implementing an AST/type walk: `go doc -all` prints every exported
// declaration's signature as an unindented "func .../type .../const .../var
// ..." line, with its doc-comment prose indented underneath. Doc comments
// are allowed, and expected, to mention Cobra/pflag by name (they do, to
// explain why this package exists); only the unindented signature lines are
// checked for a "cobra"/"pflag" reference, since those are what would mean a
// third-party type actually leaked into the public API.
func TestNoCobraOrPflagInExportedSurface(t *testing.T) {
	for _, pkg := range []string{
		"github.com/away2go/way2go/cli",
		"github.com/away2go/way2go/output",
	} {
		out, err := exec.Command("go", "doc", "-all", pkg).CombinedOutput()
		if err != nil {
			t.Fatalf("go doc -all %s: %v\n%s", pkg, err, out)
		}
		for _, line := range strings.Split(string(out), "\n") {
			// Declaration signature lines are the ones go doc -all prints
			// starting with the Go keyword itself (func/type/const/var);
			// everything else — package doc, indented per-symbol doc
			// prose, blank lines, "TYPES"/"FUNCS" section headers — is
			// commentary, not part of the exported surface's shape, and is
			// allowed (expected, even) to mention Cobra/pflag by name.
			isSignature := false
			for _, kw := range []string{"func ", "type ", "const ", "var "} {
				if strings.HasPrefix(line, kw) {
					isSignature = true
					break
				}
			}
			if !isSignature {
				continue
			}
			lower := strings.ToLower(line)
			if strings.Contains(lower, "cobra") || strings.Contains(lower, "pflag") {
				t.Errorf("package %s: exported signature references cobra/pflag: %q", pkg, line)
			}
		}
	}
}
