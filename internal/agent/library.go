package agent

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/soppylzz/soppy-utils/internal/agent/content"
	"github.com/soppylzz/soppy-utils/internal/core"
)

// importItem copies a template of somebody's own into the library.
//
// The definition is read before anything is copied, so a directory that is not an item is
// refused rather than half-installed and discovered while generating.
func importItem(configRoot string, args []string, reply core.Responder) error {
	fs := flag.NewFlagSet("agent import", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("agent import needs one directory holding an item.json")
	}
	src, replaced, err := content.Store(configRoot, fs.Arg(0))
	if err != nil {
		return err
	}
	what := "imported"
	if replaced {
		what = "replaced"
	}
	if err := reply.Notify(fmt.Sprintf("%s %s: %s", what, src.Item.ID, titleOf(src.Item))); err != nil {
		return err
	}
	if err := reply.Notify("it is in " + filepath.Join(content.LibraryDir(configRoot), src.Item.ID)); err != nil {
		return err
	}
	return reply.Finish(nil)
}

// removeItem takes one back out. The documents it generated stay where they are and become
// the project's, which is what happens to any item that stops being chosen.
func removeItem(configRoot string, args []string, reply core.Responder) error {
	fs := flag.NewFlagSet("agent remove", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("agent remove needs the id of an item of your own")
	}
	if err := content.Remove(configRoot, fs.Arg(0)); err != nil {
		return err
	}
	if err := reply.Notify("removed " + fs.Arg(0) + "; documents it generated stay where they are"); err != nil {
		return err
	}
	return reply.Finish(nil)
}
