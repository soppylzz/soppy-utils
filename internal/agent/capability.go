package agent

import (
	"strings"

	"github.com/soppylzz/soppy-utils/internal/agent/content"
	"github.com/soppylzz/soppy-utils/internal/registry"
)

// Register adds the agent capability.
func Register(r *registry.Registry) error {
	commands, err := commandSurface()
	if err != nil {
		return err
	}
	return r.Register(registry.Capability{
		ID:       "agent",
		Title:    "Agent documents",
		Usage:    "spu agent [--items list] [--harnesses list] [--answers path] [status|edit|migrate]",
		Commands: commands,
		Judge:    judge,
	})
}

// commandSurface describes the commands and their flags.
//
// The lists a flag accepts are named in the help rather than only in a message after a
// wrong value: `--items` is unusable without knowing what the item ids are, and an error
// that says "run --help" has to make --help worth running.
func commandSurface() ([]registry.Command, error) {
	items, err := content.Items()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}

	chooseFlags := []registry.Flag{
		{Name: "--items", Arg: "list",
			Summary: "Generate these items and nothing else, comma-separated. Available: " +
				strings.Join(ids, ", ")},
		{Name: "--harnesses", Arg: "list", Options: harnessNames,
			Summary: "Arrange the project for these harnesses, comma-separated"},
		{Name: "--answers", Arg: "path",
			Summary: "Read the selection and the answers to every question from a JSON file " +
				"instead of asking"},
		{Name: "--interactive", Arg: "bool",
			Summary: "Ask anyway, even when the flags are complete"},
	}

	return []registry.Command{
		{Name: "", Summary: "Choose and generate the project's agent documents", Flags: chooseFlags},
		{Name: "status", Summary: "Report what is generated, what is missing, and what changed"},
		{Name: "edit", Summary: "Answer one item's questions again and re-render it",
			Flags: []registry.Flag{
				{Name: "--items", Arg: "list", Summary: "Re-answer only these items"},
				{Name: "--answers", Arg: "path", Summary: "Read the answers from a file"},
				{Name: "--interactive", Arg: "bool", Summary: "Ask anyway"},
			}},
		{Name: "migrate", Summary: "Record documents generated before spu kept a ledger"},
	}, nil
}

// judge reports whether the agent documents can be generated here.
//
// There is no requirement to check and no platform to gate on: generating text needs a
// directory and nothing else. It is still judged per session rather than declared usable
// once, because the answer belongs to the session and not to the daemon.
func judge(registry.Host) registry.Assessment {
	return registry.Assessment{State: registry.Available}
}
