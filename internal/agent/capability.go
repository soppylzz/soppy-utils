package agent

import "github.com/soppylzz/soppy-utils/internal/registry"

// Register adds the agent capability.
func Register(r *registry.Registry) error {
	return r.Register(registry.Capability{
		ID:       "agent",
		Title:    "Agent documents",
		Usage:    "spu agent [status|edit|migrate]",
		Commands: commands,
		Judge:    judge,
	})
}

// commands is the command surface. Help and usage errors render from this, so a command
// that exists in the parser and not here is a command nobody can find.
var commands = []registry.Command{
	{Name: "", Summary: "Choose and generate the project's agent documents"},
	{Name: "status", Summary: "Report what is generated, what is missing, and what changed"},
	{Name: "edit", Summary: "Answer one item's questions again and re-render it"},
	{Name: "migrate", Summary: "Record documents generated before spu kept a ledger"},
}

// judge reports whether the agent documents can be generated here.
//
// There is no requirement to check and no platform to gate on: generating text needs a
// directory and nothing else. It is still judged per session rather than declared usable
// once, because the answer belongs to the session and not to the daemon.
func judge(registry.Host) registry.Assessment {
	return registry.Assessment{State: registry.Available}
}
