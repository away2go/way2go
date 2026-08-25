package web_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/away2go/way2go/web"
)

func TestAllRejectsDuplicateActivityName(t *testing.T) {
	a := web.Activity("dup", noopHandler)
	b := web.Activity("dup", noopHandler)

	_, err := web.All(a, b)
	if err == nil {
		t.Fatalf("All: expected an error for a duplicate Activity name")
	}
	if !strings.Contains(err.Error(), "dup") {
		t.Fatalf("error %q does not mention the duplicate name", err)
	}
}

func TestGroupIsAnHTTPHandler(t *testing.T) {
	var _ http.Handler = web.Group{}
}

func TestGroupDefinitionsReturnsRegisteredDefinitions(t *testing.T) {
	a := web.Activity("a", noopHandler)
	group, err := web.All(a)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	defs := group.Definitions()
	if len(defs) != 1 || defs[0].Name() != "a" {
		t.Fatalf("Definitions() = %+v, want one entry named a", defs)
	}
}
