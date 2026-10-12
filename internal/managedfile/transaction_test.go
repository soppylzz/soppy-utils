package managedfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/soppylzz/soppy-utils/internal/configstore"
)

func TestRollbackRestoresExistingAndRemovesNewFile(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing")
	fresh := filepath.Join(dir, "fresh")
	if err := os.WriteFile(existing, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	err := WithRollback([]string{existing, fresh}, func() error {
		if e := configstore.AtomicWrite(existing, []byte("changed"), 0600); e != nil {
			return e
		}
		if e := configstore.AtomicWrite(fresh, []byte("new"), 0600); e != nil {
			return e
		}
		return errors.New("later step failed")
	})
	if err == nil {
		t.Fatal("expected failure")
	}
	b, e := os.ReadFile(existing)
	if e != nil || string(b) != "original" {
		t.Fatalf("existing file: %q, %v", b, e)
	}
	if _, e := os.Stat(fresh); !os.IsNotExist(e) {
		t.Fatal("new file remains")
	}
}
