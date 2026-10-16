package registry

import "testing"

func TestListKeepsRegistrationOrder(t *testing.T) {
	r := New()
	for _, id := range []string{"welcome", "license", "git", "vscode"} {
		if err := r.Register(Capability{ID: id, Title: id}); err != nil {
			t.Fatalf("Register(%s): %v", id, err)
		}
	}

	got := r.List()
	want := []string{"welcome", "license", "git", "vscode"}
	if len(got) != len(want) {
		t.Fatalf("List returned %d capabilities, want %d", len(got), len(want))
	}
	// Order is part of the behaviour: help output that shuffles between runs is a
	// documentation bug that only shows up on somebody else's machine.
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("List()[%d].ID = %q, want %q", i, got[i].ID, id)
		}
	}
}

func TestRegisterRejectsADuplicateID(t *testing.T) {
	r := New()
	if err := r.Register(Capability{ID: "welcome"}); err != nil {
		t.Fatalf("first Register: %v", err)
	}

	// Two packages claiming one name means one of them is silently unreachable, and
	// which one wins would depend on assembly order.
	if err := r.Register(Capability{ID: "welcome"}); err == nil {
		t.Fatal("Register accepted a duplicate id, want an error")
	}
}

func TestARegistryIsNotGlobal(t *testing.T) {
	// The reason registration is explicit rather than done in init(). Capability
	// evaluation is what the tests need to drive with a controlled environment, and a
	// package-level registry makes tests order-dependent and impossible to isolate.
	first := New()
	second := New()
	if err := first.Register(Capability{ID: "welcome"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if len(second.List()) != 0 {
		t.Errorf("a second registry saw %d capabilities from the first", len(second.List()))
	}
}

func TestLookupFindsARegisteredCapability(t *testing.T) {
	r := New()
	if err := r.Register(Capability{ID: "welcome", Title: "Welcome banner"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, ok := r.Lookup("welcome")
	if !ok {
		t.Fatal("Lookup did not find a registered capability")
	}
	if got.Title != "Welcome banner" {
		t.Errorf("Title = %q, want Welcome banner", got.Title)
	}
}

func TestLookupReportsAnUnknownID(t *testing.T) {
	r := New()
	if _, ok := r.Lookup("nonesuch"); ok {
		t.Fatal("Lookup found a capability that was never registered")
	}
}
