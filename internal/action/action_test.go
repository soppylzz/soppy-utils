package action

import (
	"encoding/json"
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
