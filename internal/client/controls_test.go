package client

import (
	"strings"
	"testing"

	"github.com/soppylzz/soppy-utils/internal/form"
)

func TestControlsMapEachFieldKind(t *testing.T) {
	controls, err := controls(form.Spec{Fields: []form.Field{
		{Key: "name", Type: form.String, Title: "Name"},
		{Key: "banner_in_tmux", Type: form.Boolean, Title: "Show in tmux"},
		{Key: "rc", Type: form.Enum, Title: "Shell hooks",
			Options: []form.Option{{Value: "none", Label: "None"}, {Value: "both", Label: "Bash and Zsh"}}},
	}})
	if err != nil {
		t.Fatalf("controls: %v", err)
	}
	if len(controls) != 3 {
		t.Fatalf("got %d controls, want 3", len(controls))
	}

	want := []kind{inputKind, confirmKind, selectKind}
	for i, k := range want {
		if controls[i].kind != k {
			t.Errorf("controls[%d].kind = %q, want %q", i, controls[i].kind, k)
		}
	}
	if controls[2].Options[1].Label != "Bash and Zsh" {
		t.Errorf("select options lost their labels: %+v", controls[2].Options)
	}
}

func TestControlsCarryTheTitleAndHelp(t *testing.T) {
	controls, err := controls(form.Spec{Fields: []form.Field{
		{Key: "name", Type: form.String, Title: "Your name", Help: "Shown in the banner"},
	}})
	if err != nil {
		t.Fatalf("controls: %v", err)
	}
	if controls[0].Title != "Your name" {
		t.Errorf("title = %q, want the title core sent", controls[0].Title)
	}
	if controls[0].Help != "Shown in the banner" {
		t.Errorf("help = %q, want the help core sent", controls[0].Help)
	}
}

func TestControlsCarryTheCurrentValues(t *testing.T) {
	controls, err := controls(form.Spec{Fields: []form.Field{
		{Key: "name", Type: form.String, Value: "Alice"},
		{Key: "banner_in_tmux", Type: form.Boolean, Value: true},
		{Key: "fresh", Type: form.String},
	}})
	if err != nil {
		t.Fatalf("controls: %v", err)
	}
	if controls[0].Initial != "Alice" {
		t.Errorf("initial = %v, want Alice", controls[0].Initial)
	}
	if controls[1].Initial != true {
		t.Errorf("initial = %v, want true", controls[1].Initial)
	}
	if controls[2].Initial != nil {
		t.Errorf("initial = %v, want nil for a field with no saved value", controls[2].Initial)
	}
}

func TestAFieldKindThisClientCannotDrawIsRejectedByName(t *testing.T) {
	// A window client and a terminal client draw different controls, but neither may
	// silently drop a field: the user would submit a form that was missing a question.
	//
	// The example here used to be a map of strings, which this client now draws. The
	// list grows when a util needs a kind, so the example has to be one nothing draws.
	_, err := controls(form.Spec{Fields: []form.Field{
		{Key: "tags", Type: form.Type("array"), Title: "Tags"},
	}})
	if err == nil {
		t.Fatal("controls accepted a kind it cannot draw, want an error")
	}
	if !strings.Contains(err.Error(), "tags") {
		t.Errorf("error %q does not name the field", err)
	}
}

func TestABooleanWithNoSavedValueStartsOff(t *testing.T) {
	// A question whose answer was never set has to start somewhere, and "no" is the
	// safe reading of "nobody said".
	controls, err := controls(form.Spec{Fields: []form.Field{
		{Key: "banner_in_tmux", Type: form.Boolean},
	}})
	if err != nil {
		t.Fatalf("controls: %v", err)
	}
	if controls[0].Initial != false {
		t.Errorf("initial = %v, want false", controls[0].Initial)
	}
}

func TestControlsMapAKeyValueField(t *testing.T) {
	controls, err := controls(form.Spec{Fields: []form.Field{
		{Key: "aliases", Type: form.KeyValue, Title: "Aliases",
			Value: map[string]any{"st": "status"}},
	}})
	if err != nil {
		t.Fatalf("controls: %v", err)
	}
	if controls[0].kind != keyValueKind {
		t.Errorf("kind = %q, want %q", controls[0].kind, keyValueKind)
	}
}

func TestKeyValueTextRoundTrips(t *testing.T) {
	// The terminal renders a map as the text a person edits, and reads it back. Both
	// halves are here because a form that renders something it cannot read is a form
	// that loses the user's work.
	original := map[string]any{"st": "status", "lg": "log --oneline"}
	back, err := parseKeyValue(keyValueText(original))
	if err != nil {
		t.Fatalf("parseKeyValue: %v", err)
	}
	if len(back) != 2 || back["lg"] != "log --oneline" {
		t.Errorf("round trip gave %v, want %v", back, original)
	}
}

func TestKeyValueTextIsReadable(t *testing.T) {
	// Sorted, so the text does not shuffle between openings, and indented, because a
	// person is going to edit it.
	text := keyValueText(map[string]any{"zeta": "z", "alpha": "a"})
	want := "{\n  \"alpha\": \"a\",\n  \"zeta\": \"z\"\n}"
	if text != want {
		t.Errorf("text =\n%s\nwant\n%s", text, want)
	}
}

func TestParseKeyValueRejectsWhatIsNotAnObject(t *testing.T) {
	for _, text := range []string{"", "not json", "[1,2]", `{"a":1}`} {
		if _, err := parseKeyValue(text); err == nil {
			t.Errorf("parseKeyValue(%q) succeeded, want an error", text)
		}
	}
}
