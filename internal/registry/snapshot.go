package registry

import "os"

// Host is the machine as one session sees it.
//
// Every field is a fact about a session rather than about this process. The daemon is
// long-lived and was started by whichever client came first, so its own PATH, shell,
// and home are the wrong answer for everyone who arrives later.
type Host struct {
	GOOS  string
	Home  string
	Shell string

	// LookPath resolves a command as this session would.
	LookPath func(name string) (string, error)

	// Stat reports a path as this session would.
	Stat func(path string) (os.FileInfo, error)
}

// Snapshot is every capability, judged for one session.
type Snapshot struct {
	Capabilities []Report `json:"capabilities"`
}

// Report is one capability, judged.
type Report struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Usage    string    `json:"usage,omitempty"`
	State    State     `json:"state"`
	Reason   Reason    `json:"reason,omitempty"`
	Detail   string    `json:"detail,omitempty"`
	Missing  []string  `json:"missing,omitempty"`
	Commands []Command `json:"commands,omitempty"`
}

// Snapshot judges every capability against one session's environment.
//
// The result is a copy, including the slices inside it. A daemon that let a caller
// reach back into its own capability data through a returned slice would be one
// mutation away from reporting something it does not contain.
func (r *Registry) Snapshot(h Host) Snapshot {
	items := r.List()
	out := Snapshot{Capabilities: make([]Report, 0, len(items))}
	for _, c := range items {
		a := Assessment{State: Available}
		if c.Judge != nil {
			a = c.Judge(h)
		}
		out.Capabilities = append(out.Capabilities, Report{
			ID:       c.ID,
			Title:    c.Title,
			Usage:    c.Usage,
			State:    a.State,
			Reason:   a.Reason,
			Detail:   a.Detail,
			Missing:  append([]string(nil), a.Missing...),
			Commands: copyCommands(c.Commands),
		})
	}
	return out
}

func copyCommands(in []Command) []Command {
	if in == nil {
		return nil
	}
	out := make([]Command, len(in))
	for i, c := range in {
		out[i] = c
		out[i].Flags = append([]Flag(nil), c.Flags...)
		for j := range out[i].Flags {
			out[i].Flags[j].Options = append([]string(nil), c.Flags[j].Options...)
		}
	}
	return out
}
