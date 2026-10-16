package license

import (
	"context"
	"github.com/soppylzz/soppy-utils/internal/core/coretest"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderKnownLegalText(t *testing.T) {
	for _, kind := range []string{"mit", "isc"} {
		b, err := Render(kind, "Alice", 2026)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		if !strings.HasPrefix(s, "Copyright (c) 2026 Alice\n") || !strings.HasSuffix(s, "\n") {
			t.Fatalf("bad %s output", kind)
		}
	}
}

func TestExplicitAnswersRepairVersionConflict(t *testing.T) {
	home := t.TempDir()
	s := Store(home)
	data := map[string]any{"type": "isc", "author": "Alice", "output": "LICENSE"}
	if err := s.Save(data); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.VersionPath(), []byte("9\n"), 0600); err != nil {
		t.Fatal(err)
	}
	answers := filepath.Join(home, "answers.json")
	raw := []byte(`{"util":"license","schema_version":1,"data":{"type":"isc","author":"Alice","output":"LICENSE"}}`)
	if err := os.WriteFile(answers, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), sessionFor(home),
		[]string{"migrate", "--answers", answers}, io.Discard, &coretest.Recorder{}); err != nil {
		t.Fatal(err)
	}
	v := s.Read()
	if !v.Writable || v.StoredVersion != 1 || v.Warning != "" {
		t.Fatalf("repair failed: %+v", v)
	}
}
