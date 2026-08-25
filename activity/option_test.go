package activity_test

import (
	"testing"

	"github.com/away2go/way2go/activity"
	"github.com/away2go/way2go/cli"
	"github.com/away2go/way2go/web"
)

// TestDescribeIsUsableOnBothTargetContracts is the API contract
// proof: the very same activity.Describe(...) expression assigns directly,
// with no conversion, to both web.Option and cli.Option — because its
// static return type, activity.Portable, embeds both sealed contracts.
func TestDescribeIsUsableOnBothTargetContracts(t *testing.T) {
	var _ web.Option = activity.Describe("x")
	var _ cli.Option = activity.Describe("x")

	wb := activity.New("search", "web")
	wb.ApplyWeb(activity.Describe("Searches for things."))
	if got := wb.Snapshot().Description; got != "Searches for things." {
		t.Fatalf("web Description = %q, want %q", got, "Searches for things.")
	}

	cb := activity.New("search", "cli")
	cb.ApplyCLI(activity.Describe("Searches for things."))
	if got := cb.Snapshot().Description; got != "Searches for things." {
		t.Fatalf("cli Description = %q, want %q", got, "Searches for things.")
	}
}
