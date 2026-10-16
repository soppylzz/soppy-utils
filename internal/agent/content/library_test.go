package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rootOf is the configuration root a session with this home resolves to. The tests set
// the home and the library resolves the rest, which is the path the command takes too.
func rootOf(home string) string { return filepath.Join(home, ".config", "soppy-utils") }

// writeItem puts a template on disk the way somebody would.
func writeItem(t *testing.T, dir, id, body string, files map[string]string) {
	t.Helper()
	itemDir := filepath.Join(dir, id)
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "item.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	for name, content := range files {
		full := filepath.Join(itemDir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
}

const customItem = `{
  "id": "my-conventions",
  "version": 1,
  "title": "Mine",
  "policy": "reuse",
  "outputs": [{"root": "agents", "path": "prompts/mine.md",
    "content": [{"file": "header.md"}]}]
}`

func TestALibraryWithNothingAddedIsTheShippedItems(t *testing.T) {
	shipped, err := Items()
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	lib, err := Library(t.TempDir())
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	if len(lib) != len(shipped) {
		t.Errorf("Library has %d items, want the %d shipped ones", len(lib), len(shipped))
	}
	for _, src := range lib {
		if src.Files == nil {
			t.Errorf("%s has no files to read its content from", src.Item.ID)
		}
	}
}

func TestImportAddsATemplateOfYourOwn(t *testing.T) {
	home := t.TempDir()
	from := t.TempDir()
	writeItem(t, from, "my-conventions", customItem, map[string]string{"header.md": "# Mine\n"})

	got, replaced, err := Store(rootOf(home), filepath.Join(from, "my-conventions"))
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if replaced {
		t.Error("a first import reported replacing something")
	}
	if got.Item.ID != "my-conventions" {
		t.Errorf("imported %q", got.Item.ID)
	}

	lib, err := Library(rootOf(home))
	if err != nil {
		t.Fatalf("Library: %v", err)
	}
	found := false
	for _, src := range lib {
		if src.Item.ID == "my-conventions" {
			found = true
			body, err := src.Files.Open("header.md")
			if err != nil {
				t.Fatalf("the imported item cannot read its own content: %v", err)
			}
			body.Close()
		}
	}
	if !found {
		t.Error("the imported item is not in the library")
	}
}

func TestImportingTheSameNameAgainReplacesIt(t *testing.T) {
	// Templates improve, and re-importing the better one is the ordinary flow.
	home := t.TempDir()
	from := t.TempDir()
	writeItem(t, from, "my-conventions", customItem, map[string]string{"header.md": "first\n"})
	if _, _, err := Store(rootOf(home), filepath.Join(from, "my-conventions")); err != nil {
		t.Fatalf("first: %v", err)
	}

	writeItem(t, from, "my-conventions", customItem, map[string]string{"header.md": "second\n"})
	_, replaced, err := Store(rootOf(home), filepath.Join(from, "my-conventions"))
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if !replaced {
		t.Error("re-importing reported nothing replaced")
	}
	body, err := os.ReadFile(filepath.Join(LibraryDir(rootOf(home)), "my-conventions", "header.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(body) != "second\n" {
		t.Errorf("the old content survived: %q", body)
	}
}

func TestATemplateThatDoesNotParseIsRefusedBeforeAnythingIsCopied(t *testing.T) {
	home := t.TempDir()
	from := t.TempDir()
	writeItem(t, from, "broken", `{"id": "broken"}`, nil)

	if _, _, err := Store(rootOf(home), filepath.Join(from, "broken")); err == nil {
		t.Fatal("a definition that does not parse was imported")
	}
	if _, err := os.Stat(LibraryDir(rootOf(home))); err == nil {
		t.Error("something was copied before the definition was read")
	}
}

func TestATemplateCannotTakeTheNameOfOneSpuShips(t *testing.T) {
	// The ledger records ids. Two items with one name would make `--items` mean whichever
	// happened to load, and what was generated unreproducible.
	home := t.TempDir()
	from := t.TempDir()
	writeItem(t, from, "conduct-of-go", `{
	  "id": "conduct-of-go", "version": 1, "policy": "reuse",
	  "outputs": [{"root": "agents", "path": "x.md", "content": [{"text": "hi"}]}]
	}`, nil)

	_, _, err := Store(rootOf(home), filepath.Join(from, "conduct-of-go"))
	if err == nil {
		t.Fatal("a template took the name of a shipped item")
	}
	if !strings.Contains(err.Error(), "conduct-of-go") {
		t.Errorf("error %q does not name the clash", err)
	}
}

func TestAnItemWhoseDirectoryAndIdDisagreeIsRefused(t *testing.T) {
	// spu finds one by the other, so a mismatch would be found by whoever ran it next.
	home := t.TempDir()
	from := t.TempDir()
	writeItem(t, from, "somewhere-else", customItem, nil)

	if _, _, err := Store(rootOf(home), filepath.Join(from, "somewhere-else")); err == nil {
		t.Fatal("a directory whose name disagrees with its id was imported")
	}
}

func TestAnItemOfYourOwnCannotShadowAShippedOne(t *testing.T) {
	// Written by hand into the library rather than imported, which is the case the check
	// in Store cannot see.
	home := t.TempDir()
	writeItem(t, LibraryDir(rootOf(home)), "conduct-of-go", `{
	  "id": "conduct-of-go", "version": 1, "policy": "reuse",
	  "outputs": [{"root": "agents", "path": "x.md", "content": [{"text": "hi"}]}]
	}`, nil)

	if _, err := Library(rootOf(home)); err == nil {
		t.Fatal("a library holding a shipped id was accepted")
	}
}

func TestRemoveTakesItBackOut(t *testing.T) {
	home := t.TempDir()
	from := t.TempDir()
	writeItem(t, from, "my-conventions", customItem, map[string]string{"header.md": "x\n"})
	if _, _, err := Store(rootOf(home), filepath.Join(from, "my-conventions")); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if err := Remove(rootOf(home), "my-conventions"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := Remove(rootOf(home), "my-conventions"); err == nil {
		t.Error("removing something twice was accepted")
	}
}
