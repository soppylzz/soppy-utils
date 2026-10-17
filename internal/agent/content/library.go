package content

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/soppylzz/soppy-utils/internal/agent/item"
)

// LibraryDir is where a template somebody wrote lives, under a configuration root the
// caller has already resolved for this platform.
//
// A global library rather than one per project, because a convention somebody holds is
// theirs and does not stop being true when they change repository. What was generated
// stays in the project; what can be generated lives here.
func LibraryDir(configRoot string) string {
	return filepath.Join(configRoot, "agent", "templates")
}

// Library is every item that can be generated: the ones spu ships, then the ones somebody
// added.
//
// Sorted by id, and an id that collides is refused rather than resolved. Two items with
// one name would make `--items` ambiguous, and the ledger records names.
func Library(configRoot string) ([]item.Source, error) {
	embedded, err := Items()
	if err != nil {
		return nil, err
	}

	out := make([]item.Source, 0, len(embedded))
	seen := make(map[string]bool, len(embedded))
	for _, it := range embedded {
		out = append(out, item.Source{Item: it, Files: Files()})
		seen[it.ID] = true
	}

	custom, err := Custom(configRoot)
	if err != nil {
		return nil, err
	}
	for _, src := range custom {
		if seen[src.Item.ID] {
			return nil, fmt.Errorf("an item of your own is called %q, which is also one of "+
				"spu's; rename it in %s", src.Item.ID, LibraryDir(configRoot))
		}
		seen[src.Item.ID] = true
		out = append(out, src)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Item.ID < out[j].Item.ID })
	return out, nil
}

// Custom is the items in the library, in a stable order.
//
// A library that is not there is an empty one: most people never add anything, and that
// is not an error.
func Custom(configRoot string) ([]item.Source, error) {
	dir := LibraryDir(configRoot)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var out []item.Source
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		src, err := Load(filepath.Join(dir, e.Name()))
		if err != nil {
			// One broken item must not take the rest of the library with it, but it must
			// not be silent either: an item that stopped parsing is a document that
			// stopped being generated.
			return nil, err
		}
		out = append(out, src)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Item.ID < out[j].Item.ID })
	return out, nil
}

// Load reads one item directory: an item.json and whatever it names beside it.
func Load(dir string) (item.Source, error) {
	body, err := os.ReadFile(filepath.Join(dir, "item.json"))
	if err != nil {
		return item.Source{}, fmt.Errorf("%s: %w", dir, err)
	}
	it, err := item.Parse(body)
	if err != nil {
		return item.Source{}, fmt.Errorf("%s: %w", dir, err)
	}
	if filepath.Base(dir) != it.ID {
		return item.Source{}, fmt.Errorf("%s holds an item called %q; the directory and the "+
			"id have to agree, because spu finds one by the other", dir, it.ID)
	}
	return item.Source{Item: it, Files: os.DirFS(dir)}, nil
}

// Store copies an item into the library.
//
// The definition is parsed before anything is copied, so a directory that is not an item
// is refused rather than half-installed and discovered at generation time.
func Store(configRoot, from string) (item.Source, bool, error) {
	src, err := Load(from)
	if err != nil {
		return item.Source{}, false, err
	}
	for _, shipped := range mustItems() {
		if shipped.ID == src.Item.ID {
			return item.Source{}, false, fmt.Errorf("spu already ships an item called %q; "+
				"rename yours before importing it", src.Item.ID)
		}
	}

	dir := filepath.Join(LibraryDir(configRoot), src.Item.ID)
	replaced := false
	if _, err := os.Stat(dir); err == nil {
		replaced = true
		if err := os.RemoveAll(dir); err != nil {
			return item.Source{}, false, fmt.Errorf("replace %s: %w", dir, err)
		}
	}
	if err := copyTree(from, dir); err != nil {
		return item.Source{}, false, err
	}
	return item.Source{Item: src.Item, Files: os.DirFS(dir)}, replaced, nil
}

// Remove takes an item back out of the library.
func Remove(configRoot, id string) error {
	dir := filepath.Join(LibraryDir(configRoot), id)
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("no item of your own is called %q", id)
	}
	return os.RemoveAll(dir)
}

func copyTree(from, to string) error {
	return filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		// One definition per directory: a nested one would be copied and then ignored.
		if d.IsDir() && rel != "." && !strings.Contains(rel, string(filepath.Separator)) {
			if _, err := os.Stat(filepath.Join(path, "item.json")); err == nil {
				return fmt.Errorf("%s holds another item; import one at a time", path)
			}
		}
		target := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, 0o644)
	})
}

func mustItems() []item.Item {
	items, err := Items()
	if err != nil {
		panic(err)
	}
	return items
}
