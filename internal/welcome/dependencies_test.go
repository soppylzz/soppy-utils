package welcome

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectOptionalToolsWithMissingCommands(t *testing.T) {
	home := t.TempDir()
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	fonts := filepath.Join(home, ".figlet", "fonts")
	if err := os.MkdirAll(fonts, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fonts, "speed.flf"), []byte("font"), 0600); err != nil {
		t.Fatal(err)
	}
	d := detectOptionalTools(home)
	if d.figlet || d.fastfetch {
		t.Fatal("missing commands were reported as available")
	}
	if !d.speedFont || d.slantFont {
		t.Fatal("font availability was not detected independently")
	}
}
