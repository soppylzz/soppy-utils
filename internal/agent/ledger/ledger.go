// Package ledger reads and writes spu.json, the record of what was generated.
//
// It is the only way to tell a file spu wrote from one somebody else did, and the only
// place the hash of what was written is kept. Everything that decides what to do about an
// existing file reads its facts here.
package ledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FileName is where the record lives, inside the canonical directory.
const FileName = "spu.json"

// CurrentVersion is the ledger format this build writes.
const CurrentVersion = 1

// ConflictError is a ledger this build must not write.
//
// A newer version is the case: it was written by a spu that knows fields this one does
// not, so rewriting it would drop them. It is separated from corruption because the two
// want different words — one says upgrade, the other says repair.
type ConflictError struct {
	Path    string
	Version int
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s was written by a newer spu (version %d); upgrade rather than "+
		"letting this one overwrite it", e.Path, e.Version)
}

// Ledger is the record of one project.
type Ledger struct {
	Version   int
	Harnesses []string
	Items     []Item

	// extra is every top-level field this build does not know. It is carried so that
	// reading, changing one item, and writing back does not quietly delete a peer's data.
	extra map[string]json.RawMessage
}

// Item is one generated item and the answers it was generated from.
//
// The tags are the file's contract rather than a convenience. Without them a struct
// marshals as `ItemVersion`, which reads differently from `item_version` beside it and
// silently disagrees with what the design documents.
type Item struct {
	ID          string         `json:"id"`
	ItemVersion int            `json:"item_version"`
	Answers     map[string]any `json:"answers,omitempty"`
	Outputs     []Output       `json:"outputs,omitempty"`

	extra map[string]json.RawMessage
}

// Output is one file an item wrote, and the hash of what was written.
type Output struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Find returns a recorded item.
func (l Ledger) Find(id string) (Item, bool) {
	for _, it := range l.Items {
		if it.ID == id {
			return it, true
		}
	}
	return Item{}, false
}

// HashOf returns the hash recorded for one of an item's outputs.
//
// A path the item did not write has no hash, which is the answer that makes it somebody
// else's file.
func (it Item) HashOf(path string) (string, bool) {
	for _, out := range it.Outputs {
		if out.Path == path {
			return out.SHA256, true
		}
	}
	return "", false
}

// Read loads the ledger from a canonical directory.
//
// A directory with no ledger is an empty ledger: every project meets that case on its
// first run, and it is not a failure.
func Read(dir string) (Ledger, error) {
	path := filepath.Join(dir, FileName)
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Ledger{Version: CurrentVersion}, nil
	}
	if err != nil {
		return Ledger{}, fmt.Errorf("read %s: %w", path, err)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return Ledger{}, fmt.Errorf("%s: %w", path, err)
	}

	var l Ledger
	if raw, ok := top["version"]; ok {
		if err := json.Unmarshal(raw, &l.Version); err != nil {
			return Ledger{}, fmt.Errorf("%s: version: %w", path, err)
		}
	}
	if l.Version > CurrentVersion {
		return Ledger{}, &ConflictError{Path: path, Version: l.Version}
	}
	if raw, ok := top["harnesses"]; ok {
		if err := json.Unmarshal(raw, &l.Harnesses); err != nil {
			return Ledger{}, fmt.Errorf("%s: harnesses: %w", path, err)
		}
	}
	if raw, ok := top["items"]; ok {
		var raws []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &raws); err != nil {
			return Ledger{}, fmt.Errorf("%s: items: %w", path, err)
		}
		for _, r := range raws {
			it, err := readItem(r)
			if err != nil {
				return Ledger{}, fmt.Errorf("%s: %w", path, err)
			}
			l.Items = append(l.Items, it)
		}
	}

	delete(top, "version")
	delete(top, "harnesses")
	delete(top, "items")
	l.extra = top
	return l, nil
}

func readItem(raw map[string]json.RawMessage) (Item, error) {
	var it Item
	if r, ok := raw["id"]; ok {
		if err := json.Unmarshal(r, &it.ID); err != nil {
			return Item{}, fmt.Errorf("item id: %w", err)
		}
	}
	if r, ok := raw["item_version"]; ok {
		if err := json.Unmarshal(r, &it.ItemVersion); err != nil {
			return Item{}, fmt.Errorf("item %s: item_version: %w", it.ID, err)
		}
	}
	if r, ok := raw["answers"]; ok {
		if err := json.Unmarshal(r, &it.Answers); err != nil {
			return Item{}, fmt.Errorf("item %s: answers: %w", it.ID, err)
		}
	}
	if r, ok := raw["outputs"]; ok {
		if err := json.Unmarshal(r, &it.Outputs); err != nil {
			return Item{}, fmt.Errorf("item %s: outputs: %w", it.ID, err)
		}
	}
	delete(raw, "id")
	delete(raw, "item_version")
	delete(raw, "answers")
	delete(raw, "outputs")
	it.extra = raw
	return it, nil
}

// Write saves the ledger, creating the directory if it is not there.
//
// The fields this build does not know are written back untouched. A file two tools share
// is only shareable if the one that understands less does not delete the rest.
func (l Ledger) Write(dir string) error {
	if l.Version == 0 {
		l.Version = CurrentVersion
	}
	if l.Version > CurrentVersion {
		return &ConflictError{Path: filepath.Join(dir, FileName), Version: l.Version}
	}

	top := make(map[string]json.RawMessage, len(l.extra)+3)
	for k, v := range l.extra {
		top[k] = v
	}
	if err := put(top, "version", l.Version); err != nil {
		return err
	}
	if err := put(top, "harnesses", l.Harnesses); err != nil {
		return err
	}
	items := make([]map[string]json.RawMessage, 0, len(l.Items))
	for _, it := range l.Items {
		raw := make(map[string]json.RawMessage, len(it.extra)+4)
		for k, v := range it.extra {
			raw[k] = v
		}
		for key, value := range map[string]any{
			"id":           it.ID,
			"item_version": it.ItemVersion,
			"answers":      it.Answers,
			"outputs":      it.Outputs,
		} {
			if err := put(raw, key, value); err != nil {
				return err
			}
		}
		items = append(items, raw)
	}
	if err := put(top, "items", items); err != nil {
		return err
	}

	body, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", FileName, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), append(body, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", FileName, err)
	}
	return nil
}

func put(into map[string]json.RawMessage, key string, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode %s: %w", key, err)
	}
	into[key] = body
	return nil
}
