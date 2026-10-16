package client

import (
	"strings"
	"testing"

	"github.com/soppylzz/soppy-utils/internal/form"
)

func TestControlsMapEachFieldKind(t *testing.T) {
	controls, err := Controls(form.Spec{Fields: []form.Field{
		{Key: "name", Type: form.String, Title: "Name"},
		{Key: "banner_in_tmux", Type: form.Boolean, Title: "Show in tmux"},
		{Key: "rc", Type: form.Enum, Title: "Shell hooks",
			Options: []form.Option{{Value: "none", Label: "None"}, {Value: "both", Label: "Bash and Zsh"}}},
	}})
	if err != nil {
		t.Fatalf("Controls: %v", err)
	}
	if len(controls) != 3 {
		t.Fatalf("got %d controls, want 3", len(controls))
	}

	want := []Kind{Input, Confirm, Select}
	for i, k := range want {
		if controls[i].Kind != k {
			t.Errorf("controls[%d].Kind = %q, want %q", i, controls[i].Kind, k)
		}
	}
	if controls[2].Options[1].Label != "Bash and Zsh" {
		t.Errorf("select options lost their labels: %+v", controls[2].Options)
	}
}

func TestControlsCarryTheTitleAndHelp(t *testing.T) {
	controls, err := Controls(form.Spec{Fields: []form.Field{
		{Key: "name", Type: form.String, Title: "Your name", Help: "Shown in the banner"},
	}})
	if err != nil {
		t.Fatalf("Controls: %v", err)
	}
	if controls[0].Title != "Your name" {
		t.Errorf("title = %q, want the title core sent", controls[0].Title)
	}
	if controls[0].Help != "Shown in the banner" {
		t.Errorf("help = %q, want the help core sent", controls[0].Help)
	}
}

func TestControlsCarryTheCurrentValues(t *testing.T) {
	controls, err := Controls(form.Spec{Fields: []form.Field{
		{Key: "name", Type: form.String, Value: "Alice"},
		{Key: "banner_in_tmux", Type: form.Boolean, Value: true},
		{Key: "fresh", Type: form.String},
	}})
	if err != nil {
		t.Fatalf("Controls: %v", err)
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
	_, err := Controls(form.Spec{Fields: []form.Field{
		{Key: "aliases", Type: form.Type("keyvalue"), Title: "Aliases"},
	}})
	if err == nil {
		t.Fatal("Controls accepted a kind it cannot draw, want an error")
	}
	if !strings.Contains(err.Error(), "aliases") {
		t.Errorf("error %q does not name the field", err)
	}
}

func TestABooleanWithNoSavedValueStartsOff(t *testing.T) {
	// A question whose answer was never set has to start somewhere, and "no" is the
	// safe reading of "nobody said".
	controls, err := Controls(form.Spec{Fields: []form.Field{
		{Key: "banner_in_tmux", Type: form.Boolean},
	}})
	if err != nil {
		t.Fatalf("Controls: %v", err)
	}
	if controls[0].Initial != false {
		t.Errorf("initial = %v, want false", controls[0].Initial)
	}
}
