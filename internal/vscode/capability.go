package vscode

import (
	"github.com/soppylzz/soppy-utils/internal/registry"
)

// Register adds vscode's capability.
func Register(r *registry.Registry) error {
	return r.Register(registry.Capability{
		ID:       "vscode",
		Title:    "VS Code configuration",
		Usage:    "spu vscode -S|--snippets|-s|--settings|-k|--keys",
		Commands: commands,
		Judge:    judge,
	})
}

var commands = []registry.Command{
	{Name: "-S, --snippets", Summary: "Open the snippets directory"},
	{Name: "-s, --settings", Summary: "Open settings.json"},
	{Name: "-k, --keys", Summary: "Open keybindings.json"},
}

// judge reports whether the editor can be reached.
//
// Everything this does is hand a path to code, so without it there is nothing to do. The
// directory it opens may not exist yet — VS Code creates it — so its absence is not a
// reason to report anything.
func judge(h registry.Host) registry.Assessment {
	if h.LookPath == nil {
		return registry.Assessment{
			State: registry.Unavailable, Reason: registry.MissingDependency,
			Missing: []string{"code"}, Detail: "opening the configuration needs the code command",
		}
	}
	if _, err := h.LookPath("code"); err != nil {
		return registry.Assessment{
			State: registry.Unavailable, Reason: registry.MissingDependency,
			Missing: []string{"code"}, Detail: "opening the configuration needs the code command",
		}
	}
	return registry.Assessment{State: registry.Available}
}
