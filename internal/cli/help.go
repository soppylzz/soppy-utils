package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/soppylzz/soppy-utils/internal/registry"
)

// buildRegistry is the one place the list of utils becomes a registry.
func buildRegistry() (*registry.Registry, error) {
	r := registry.New()
	for _, u := range utils {
		if err := u.register(r); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// helpFor writes what a client shows for `spu --help`.
//
// It reads the registry rather than a list of its own. A list beside the registry is a
// list that can be wrong, and this project has already had three of those.
func helpFor(r *registry.Registry, out io.Writer) error {
	fmt.Fprintln(out, "Usage: spu <util> [args]")
	fmt.Fprintln(out)

	items := r.List()
	width := 0
	for _, c := range items {
		if len(c.ID) > width {
			width = len(c.ID)
		}
	}
	for _, c := range items {
		fmt.Fprintf(out, "  %-*s  %s\n", width, c.ID, c.Title)
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Run spu <util> --help for a util's own options.")
	return nil
}

// briefFor writes the one line a util answers -h with.
//
// The project has always answered -h with a line and --help with the options, and that
// survives the move: the line is the capability's title.
func briefFor(r *registry.Registry, out io.Writer, id string) error {
	c, ok := r.Lookup(id)
	if !ok {
		return fmt.Errorf("unknown util %q", id)
	}
	fmt.Fprintf(out, "%s: %s\n", c.ID, c.Title)
	return nil
}

// utilHelpFor writes what a client shows for `spu <util> --help`.
//
// A util's options are described by the capability that owns them, so help and the
// argument parser cannot disagree about which flags exist.
func utilHelpFor(r *registry.Registry, out io.Writer, id string) error {
	c, ok := r.Lookup(id)
	if !ok {
		return fmt.Errorf("unknown util %q", id)
	}

	fmt.Fprintln(out, c.Usage)
	fmt.Fprintln(out)
	for _, cmd := range c.Commands {
		fmt.Fprintf(out, "  %s\n", cmd.Name)
		if cmd.Summary != "" {
			fmt.Fprintf(out, "      %s\n", cmd.Summary)
		}
		for _, f := range cmd.Flags {
			fmt.Fprintf(out, "      %s\n", flagText(f))
		}
	}
	return nil
}

// flagText is one flag as a line of help.
func flagText(f registry.Flag) string {
	text := f.Name
	switch f.Arg {
	case "", "bool":
		// Nothing to supply.
	case "enum":
		text += " <" + strings.Join(f.Options, "|") + ">"
	default:
		text += " <" + f.Arg + ">"
	}
	if f.Summary != "" {
		text += "  " + f.Summary
	}
	return text
}
