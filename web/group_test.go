package web_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/away2go/way2go/param"
	"github.com/away2go/way2go/web"
)

func TestAllRejectsDuplicateActivityName(t *testing.T) {
	a := web.Activity("dup", noopHandler, web.Get("/a"))
	b := web.Activity("dup", noopHandler, web.Get("/b"))

	_, err := web.All(a, b)
	if err == nil {
		t.Fatalf("All: expected an error for a duplicate Activity name")
	}
	if !strings.Contains(err.Error(), "dup") {
		t.Fatalf("error %q does not mention the duplicate name", err)
	}
}

func TestAllRejectsDuplicateMethodPathPair(t *testing.T) {
	a := web.Activity("a", noopHandler, web.Get("/items"))
	b := web.Activity("b", noopHandler, web.Get("/items"))

	_, err := web.All(a, b)
	if err == nil {
		t.Fatalf("All: expected an error for a duplicate GET /items registration")
	}
}

func TestAllAllowsSamePathDifferentMethod(t *testing.T) {
	a := web.Activity("list", noopHandler, web.Get("/items"))
	b := web.Activity("create", noopHandler, web.Post("/items"))

	if _, err := web.All(a, b); err != nil {
		t.Fatalf("All: unexpected error for distinct method/path pairs: %v", err)
	}
}

func TestAllRejectsMissingRoute(t *testing.T) {
	a := web.Activity("no-route", noopHandler)
	if _, err := web.All(a); err == nil {
		t.Fatalf("All: expected an error for a missing route")
	}
}

func TestAllRejectsPlaceholderWithoutBinding(t *testing.T) {
	a := web.Activity("get-item", noopHandler, web.Get("/items/{id}"))
	if _, err := web.All(a); err == nil {
		t.Fatalf("All: expected an error for a path placeholder with no FromPath binding")
	}
}

func TestAllRejectsBindingWithoutPlaceholder(t *testing.T) {
	id := param.String("id")
	a := web.Activity("get-item", noopHandler, web.Get("/items"), web.FromPath(id))
	if _, err := web.All(a); err == nil {
		t.Fatalf("All: expected an error for a FromPath binding with no matching placeholder")
	}
}

func TestAllAcceptsMatchingPlaceholderAndBinding(t *testing.T) {
	id := param.String("id")
	a := web.Activity("get-item", noopHandler, web.Get("/items/{id}"), web.FromPath(id))
	if _, err := web.All(a); err != nil {
		t.Fatalf("All: unexpected error: %v", err)
	}
}

func TestGroupIsAnHTTPHandler(t *testing.T) {
	var _ http.Handler = web.Group{}
}

func TestGroupDefinitionsReturnsRegisteredDefinitions(t *testing.T) {
	a := web.Activity("a", noopHandler, web.Get("/a"))
	group, err := web.All(a)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	defs := group.Definitions()
	if len(defs) != 1 || defs[0].Name() != "a" {
		t.Fatalf("Definitions() = %+v, want one entry named a", defs)
	}
}
