package agent

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/soppylzz/soppy-utils/internal/agent/content"
	"github.com/soppylzz/soppy-utils/internal/agent/item"
	"github.com/soppylzz/soppy-utils/internal/agent/ledger"
	"github.com/soppylzz/soppy-utils/internal/core"
)

// status reports what the ledger records against what is on disk. It writes nothing.
func status(project, configRoot string, args []string, reply core.Responder) error {
	if err := acceptNoFlags("agent status", args); err != nil {
		return err
	}
	led, err := ledger.Read(canonical(project))
	if err != nil {
		return err
	}
	if len(led.Items) == 0 {
		if err := reply.Warn("nothing has been generated in " + canonical(project)); err != nil {
			return err
		}
		return reply.Finish(nil)
	}

	items, err := content.Library(configRoot)
	if err != nil {
		return err
	}
	known := make(map[string]item.Item, len(items))
	for _, src := range items {
		known[src.Item.ID] = src.Item
	}

	for _, recorded := range led.Items {
		// The index is generated from the selection rather than from an item, so it is
		// not in the list to be looked up in. Reporting it as an unknown item was a
		// warning about a file spu wrote itself.
		if recorded.ID == indexID {
			continue
		}
		it, still := known[recorded.ID]
		switch {
		case !still:
			_ = reply.Notify(fmt.Sprintf("%s: this spu does not know this item", recorded.ID))
		case it.Version != recorded.ItemVersion:
			_ = reply.Notify(fmt.Sprintf("%s: generated from version %d, this spu has %d",
				recorded.ID, recorded.ItemVersion, it.Version))
		}
		for _, out := range recorded.Outputs {
			path := filepath.Join(canonical(project), filepath.FromSlash(out.Path))
			body, err := os.ReadFile(path)
			if err != nil {
				_ = reply.Notify(fmt.Sprintf("%s: missing", out.Path))
				continue
			}
			if hashOf(body) != out.SHA256 {
				_ = reply.Notify(fmt.Sprintf("%s: changed since it was generated", out.Path))
			}
		}
	}
	return reply.Finish(nil)
}
