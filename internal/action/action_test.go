package action

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestActionWireFormat(t *testing.T) {
	// This is what a client receives and acts on, so it is pinned rather than described.
	// argv is an array and never a string: a shell would re-split it, and the difference
	// between an argument and an injection is exactly who does the splitting.
	got, err := json.Marshal(Action{
		Kind:           Run,
		Argv:           []string{"brew", "install", "figlet", "fastfetch"},
		NeedsElevation: true,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"kind":"run","argv":["brew","install","figlet","fastfetch"],"needs_elevation":true}`
	if string(got) != want {
		t.Errorf("wire format\n got %s\nwant %s", got, want)
	}
}

func TestOutcomeIsCompleteOnlyWhenEverythingWorked(t *testing.T) {
	cases := []struct {
		name string
		out  Outcome
		want bool
	}{
		{"nothing to do", Outcome{}, true},
		{"everything performed", Outcome{Performed: 2}, true},
		{"declined", Outcome{Declined: true}, false},
		{"a failure", Outcome{Failures: []string{"apt-get: not found"}}, false},
		{"declined and failed", Outcome{Declined: true, Failures: []string{"x"}}, false},
	}
	for _, tc := range cases {
		if got := tc.out.Complete(); got != tc.want {
			t.Errorf("%s: Complete() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestAnActionSaysWhetherItChangesTheMachine(t *testing.T) {
	// A client asks before installing a package and does not ask before opening a file.
	// Which of the two it is has to travel with the action: core knows what the command
	// is doing, and the client knows what its own users expect to be asked about.
	installing := Action{Kind: Run, Argv: []string{"apt-get", "install", "-y", "figlet"},
		NeedsElevation: true, ChangesMachine: true}
	opening := Action{Kind: Run, Argv: []string{"code", "/home/u/.config/Code/User/settings.json"}}

	body, err := json.Marshal(installing)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(body), `"changes_machine":true`) {
		t.Errorf("an installing action does not say it changes the machine: %s", body)
	}

	body, err = json.Marshal(opening)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(body), "changes_machine") {
		t.Errorf("an action that only opens something claims to change the machine: %s", body)
	}
}
