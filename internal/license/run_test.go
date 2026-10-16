package license

import (
	"context"
	"encoding/json"
	"github.com/soppylzz/soppy-utils/internal/core/coretest"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/soppylzz/soppy-utils/internal/core"
	"github.com/soppylzz/soppy-utils/internal/form"
	"github.com/soppylzz/soppy-utils/internal/registry"
)

func TestTheCreateFormIsTheSchemaPlusWhatIsNotConfiguration(t *testing.T) {
	// type, author, and output are stored preferences, so they are projected from the
	// schema. The year is resolved per invocation and whether to keep the answers is a
	// decision about this run, so neither is invented in a schema that describes what is
	// stored.
	spec, err := createForm(Store(t.TempDir()), map[string]any{
		"type": "mit", "author": "Alice", "output": "LICENSE",
		"year": "2026", "save_defaults": false,
	})
	if err != nil {
		t.Fatalf("createForm: %v", err)
	}

	want := []string{"type", "author", "year", "output", "save_defaults"}
	if len(spec.Fields) != len(want) {
		t.Fatalf("got %d fields, want %d: %v", len(spec.Fields), len(want), keysOf(spec))
	}
	for i, key := range want {
		if spec.Fields[i].Key != key {
			t.Errorf("field %d = %q, want %q", i, spec.Fields[i].Key, key)
		}
	}

	// A year is four digits, and a client can say so before the answer is sent.
	year := spec.Fields[2]
	if year.MinLength != 4 || year.MaxLength != 4 {
		t.Errorf("year constraints = %d..%d, want exactly four characters",
			year.MinLength, year.MaxLength)
	}
	if spec.Fields[0].Type != form.Enum || len(spec.Fields[0].Options) != 2 {
		t.Errorf("type = %+v, want a choice of two", spec.Fields[0])
	}
}

func keysOf(spec form.Spec) []string {
	out := make([]string, 0, len(spec.Fields))
	for _, f := range spec.Fields {
		out = append(out, f.Key)
	}
	return out
}

func TestCoreValidatesTheYearItIsGiven(t *testing.T) {
	// A client can only check that the answer is four characters wide. Whether it is a
	// year is core's to decide, and the answer is not taken on trust.
	for _, year := range []string{"abcd", "999", "20000", ""} {
		home := t.TempDir()
		r := &coretest.Recorder{Askable: true, Confirms: []bool{true}, Answers: map[string]any{
			"type": "mit", "author": "Alice", "output": "LICENSE",
			"year": year, "save_defaults": false,
		}}
		err := Run(context.Background(), sessionFor(home), []string{}, io.Discard, r)
		if err == nil {
			t.Errorf("year %q was accepted", year)
		}
	}
}

func TestGeneratingIsConfirmedBeforeAnythingIsWritten(t *testing.T) {
	home := t.TempDir()
	out := &strings.Builder{}
	r := &coretest.Recorder{Askable: true, Confirms: []bool{false}, Answers: map[string]any{
		"type": "mit", "author": "Alice", "output": "LICENSE",
		"year": "2026", "save_defaults": false,
	}}

	if err := Run(context.Background(), sessionFor(home), []string{}, out, r); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(r.Questions) == 0 {
		t.Fatal("nothing was confirmed before writing")
	}
	if !strings.Contains(r.Questions[0], "MIT") || !strings.Contains(r.Questions[0], "Alice") {
		t.Errorf("question = %q, want the summary in it", r.Questions[0])
	}
	// Saying no leaves nothing behind and nothing written.
	if _, err := os.Stat(filepath.Join(home, "LICENSE")); !os.IsNotExist(err) {
		t.Error("a file was written although the answer was no")
	}
	if out.String() != "" {
		t.Errorf("output = %q, want nothing", out.String())
	}
}

func TestReplacingAnExistingFileIsConfirmedSeparately(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(home, "LICENSE")
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	out := &strings.Builder{}
	r := &coretest.Recorder{Askable: true, Confirms: []bool{true, false}, Answers: map[string]any{
		"type": "mit", "author": "Alice", "output": target,
		"year": "2026", "save_defaults": false,
	}}

	if err := Run(context.Background(), sessionFor(home), []string{}, out, r); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(r.Questions) != 2 {
		t.Fatalf("asked %d questions, want one to generate and one to replace: %v",
			len(r.Questions), r.Questions)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "existing" {
		t.Error("the existing file was replaced although the answer was no")
	}
}

func TestAOneShotLicenseAsksNothing(t *testing.T) {
	// spu license --mit is the common case in a script, and it must not need a terminal.
	home := t.TempDir()
	out := &strings.Builder{}
	r := &coretest.Recorder{Askable: false}

	if err := Run(context.Background(), sessionFor(home),
		[]string{"--mit", "--name", "Alice", "--year", "2026"}, out, r); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(r.Asked) != 0 || len(r.Questions) != 0 {
		t.Errorf("asked %d forms and %d questions, want none", len(r.Asked), len(r.Questions))
	}
	body, err := render("mit", "Alice", 2026)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if out.String() != string(body) {
		t.Errorf("output is not the rendered licence byte for byte:\n got %q\nwant %q",
			out.String(), body)
	}
}

func TestTheHandlerHandsBackTheLicenceByteForByte(t *testing.T) {
	// The bytes are the file the user saves, so nothing may be added at the edges.
	r := &coretest.Recorder{}
	cmd := core.Command{Util: "license", Args: []string{"--mit", "--name", "Alice", "--year", "2026"},
		Session: sessionFor(t.TempDir())}

	if err := (Handler{}).Run(context.Background(), cmd, r); err != nil {
		t.Fatalf("Handler.Run: %v", err)
	}

	var got core.Result
	if err := json.Unmarshal(r.Finished, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	body, err := render("mit", "Alice", 2026)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got.Output != string(body) {
		t.Errorf("the output is not the rendered licence byte for byte:\n got %q\nwant %q",
			got.Output, body)
	}
	if got.OutputTo != "" {
		t.Errorf("OutputTo = %q, want the default, which is stdout", got.OutputTo)
	}
}

func TestABadFlagIsReportedWithoutWritingToTheOutput(t *testing.T) {
	// A flag error is a diagnostic. It has to reach the user as an error rather than
	// appearing in the middle of whatever they were piping.
	out := &strings.Builder{}
	err := Run(context.Background(), sessionFor(t.TempDir()), []string{"--nonesuch"}, out, &coretest.Recorder{})
	if err == nil {
		t.Fatal("an unknown flag was accepted")
	}
	if out.String() != "" {
		t.Errorf("output = %q, want nothing on it", out.String())
	}
}

func TestTheCapabilityCarriesItsCommandSurface(t *testing.T) {
	r := registry.New()
	if err := Register(r); err != nil {
		t.Fatalf("Register: %v", err)
	}
	c, ok := r.Lookup("license")
	if !ok {
		t.Fatal("license did not register a capability")
	}
	if c.Usage == "" {
		t.Error("no usage line")
	}

	want := map[string]bool{"edit": false, "migrate": false, "purge": false}
	for _, cmd := range c.Commands {
		if _, ok := want[cmd.Name]; ok {
			want[cmd.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("the command surface does not declare %s", name)
		}
	}

	// It depends on nothing beyond the filesystem: the author falls back to the user when
	// git is absent, so a missing git must not make it look unavailable. Judged through a
	// snapshot, which is how a client sees it and which handles a capability that
	// declares no judge at all.
	snap := r.Snapshot(registry.Host{GOOS: "linux"})
	if len(snap.Capabilities) != 1 {
		t.Fatalf("snapshot has %d capabilities, want 1", len(snap.Capabilities))
	}
	if got := snap.Capabilities[0].State; got != registry.Available {
		t.Errorf("state = %q (%s), want available", got, snap.Capabilities[0].Reason)
	}
}

// sessionFor is a session on the machine running the test.
func sessionFor(home string) core.Session {
	return coretest.Session(home, runtime.GOOS)
}
