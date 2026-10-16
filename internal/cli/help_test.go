package cli

import (
	"strings"
	"testing"

	"github.com/soppylzz/soppy-utils/internal/registry"
)

// synthetic is a registry nothing in this package knows about, so a test can prove the
// help is read from it rather than from a list baked in beside it.
func synthetic(t *testing.T) *registry.Registry {
	t.Helper()
	r := registry.New()
	if err := r.Register(registry.Capability{
		ID: "widget", Title: "A thing that is not real",
		Usage: "spu widget [spin]",
		Commands: []registry.Command{
			{Name: "spin", Summary: "Turn it", Flags: []registry.Flag{
				{Name: "--fast", Arg: "bool", Summary: "Turn it quickly"},
				{Name: "--mode", Arg: "enum", Summary: "How", Options: []string{"a", "b"}},
				{Name: "--label", Arg: "string", Summary: "What to call it"},
			}},
			{Name: "stop", Summary: "Stop it"},
		},
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return r
}

func TestTheOverviewComesFromTheRegistry(t *testing.T) {
	var out strings.Builder
	if err := helpFor(synthetic(t), &out); err != nil {
		t.Fatalf("helpFor: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "widget") || !strings.Contains(got, "A thing that is not real") {
		t.Errorf("the overview does not mention the capability it was given:\n%s", got)
	}
	// And nothing from the real list leaked in, which would mean it was not reading the
	// registry at all.
	if strings.Contains(got, "welcome") {
		t.Errorf("the overview lists a util that is not in the registry it was given:\n%s", got)
	}
}

func TestAUtilsHelpComesFromItsCapability(t *testing.T) {
	var out strings.Builder
	if err := utilHelpFor(synthetic(t), &out, "widget"); err != nil {
		t.Fatalf("utilHelpFor: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"spu widget [spin]",
		"spin", "Turn it",
		"stop", "Stop it",
		"--fast", "Turn it quickly",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("help does not contain %q:\n%s", want, got)
		}
	}
	// An enum shows the values it accepts, and a boolean shows none: there is nothing to
	// supply for it.
	if !strings.Contains(got, "--mode <a|b>") {
		t.Errorf("the enum flag does not show its values:\n%s", got)
	}
	if strings.Contains(got, "--fast <") {
		t.Errorf("the boolean flag asks for a value:\n%s", got)
	}
	if !strings.Contains(got, "--label <string>") {
		t.Errorf("the string flag does not say what it takes:\n%s", got)
	}
}

func TestAHelpForSomethingUnknownIsAnError(t *testing.T) {
	var out strings.Builder
	if err := utilHelpFor(synthetic(t), &out, "nonesuch"); err == nil {
		t.Fatal("help succeeded for a util that does not exist, want an error")
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want nothing", out.String())
	}
}

func TestTheRealRegistryBuilds(t *testing.T) {
	r, err := buildRegistry()
	if err != nil {
		t.Fatalf("buildRegistry: %v", err)
	}
	for _, id := range []string{"welcome", "license", "git"} {
		c, ok := r.Lookup(id)
		if !ok {
			t.Errorf("%s is not registered", id)
			continue
		}
		if c.Usage == "" {
			t.Errorf("%s has no usage line, so its help has nothing to say", id)
		}
		if len(c.Commands) == 0 {
			t.Errorf("%s declares no commands, so its help would be empty", id)
		}
	}
}

func TestBriefHelpIsOneLine(t *testing.T) {
	// The project has always answered -h with a line and --help with the options.
	var out strings.Builder
	if err := briefFor(synthetic(t), &out, "widget"); err != nil {
		t.Fatalf("briefFor: %v", err)
	}
	if got := out.String(); got != "widget: A thing that is not real\n" {
		t.Errorf("brief help = %q", got)
	}
}
