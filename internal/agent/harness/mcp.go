package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// Server is one MCP server to register with a harness.
type Server struct {
	Name string `json:"name"`

	// Command is the program a harness starts. It is a bare name rather than a path so
	// that a registration committed to a repository works on any machine where spu is
	// installed; an absolute path would work only on the one that generated it.
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

// mcpTarget is where a harness reads its project-level MCP registration.
type mcpTarget struct {
	harness string
	path    string

	// toml marks the one harness whose project configuration is a general-purpose file
	// rather than one that exists to hold servers. See registerTOML.
	toml bool
}

func mcpTargets(project string, harnesses []string) []mcpTarget {
	var out []mcpTarget
	seen := map[string]bool{}
	for _, h := range harnesses {
		if seen[h] {
			continue
		}
		seen[h] = true
		switch h {
		case "claude":
			// At the project root, not under .claude/. That is where Claude Code reads it,
			// and it is why this cannot be a link into .agents/: it is a file, not a
			// directory, and other tools write to the same one.
			out = append(out, mcpTarget{harness: h, path: filepath.Join(project, ".mcp.json")})
		case "pi":
			out = append(out, mcpTarget{harness: h, path: filepath.Join(project, ".pi", "mcp.json")})
		case "codex":
			out = append(out, mcpTarget{
				harness: h,
				path:    filepath.Join(project, ".codex", "config.toml"),
				toml:    true,
			})
		}
	}
	return out
}

// RegisterMCP writes each server into every chosen harness's registration file.
//
// Claude Code and Pi both use the same JSON shape, so they are merged the same way: our
// own entry is written and every other entry is left exactly as it was. Codex is
// different, and the reason is in registerOne.
func RegisterMCP(project string, harnesses []string, servers []Server) ([]string, error) {
	var done []string
	for _, target := range mcpTargets(project, harnesses) {
		for _, s := range servers {
			what, err := registerOne(target, s)
			if err != nil {
				return done, err
			}
			if what != "" {
				done = append(done, what)
			}
		}
	}
	return done, nil
}

// UnregisterMCP removes our entries, and only ours.
//
// A registration that outlives the choice keeps offering an agent a tool the project no
// longer uses, which is why this exists rather than being left to the user.
func UnregisterMCP(project string, harnesses []string, servers []Server) ([]string, error) {
	var done []string
	for _, target := range mcpTargets(project, harnesses) {
		if target.toml {
			// The same reason the write is refused: removing a table from somebody's
			// config without a parser has the same problem as adding one.
			continue
		}
		for _, s := range servers {
			what, err := unregisterJSON(target.path, s)
			if err != nil {
				return done, err
			}
			if what != "" {
				done = append(done, what)
			}
		}
	}
	return done, nil
}

func registerOne(target mcpTarget, s Server) (string, error) {
	if target.toml {
		return registerTOML(target.harness, target.path, s)
	}
	return registerJSON(target.harness, target.path, s)
}

// registerJSON adds our entry to a file that is a map of servers.
//
// Merging is safe here because the file exists to hold exactly this: a key naming a
// server. Everything else in it is carried through untouched, so a user's own servers and
// any keys this build does not know both survive.
func registerJSON(harness, path string, s Server) (string, error) {
	doc, err := readJSONObject(path)
	if err != nil {
		return "", err
	}
	named, err := objectAt(doc, "mcpServers")
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}

	want := entryFor(s)
	if existing, ok := named[s.Name]; ok && sameEntry(existing, want) {
		return "", nil
	}
	named[s.Name] = mustJSON(want)
	doc["mcpServers"] = mustJSON(named)
	if err := writeJSONObject(path, doc); err != nil {
		return "", err
	}
	return "registered " + s.Name + " for " + harness, nil
}

func unregisterJSON(path string, s Server) (string, error) {
	if _, err := os.Stat(path); err != nil {
		return "", nil
	}
	doc, err := readJSONObject(path)
	if err != nil {
		return "", err
	}
	named, err := objectAt(doc, "mcpServers")
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	if _, ok := named[s.Name]; !ok {
		return "", nil
	}
	delete(named, s.Name)
	doc["mcpServers"] = mustJSON(named)
	if err := writeJSONObject(path, doc); err != nil {
		return "", err
	}
	return "removed " + s.Name + " from " + path, nil
}

// registerTOML writes a Codex registration, and only into a file that is not there.
//
// `.codex/config.toml` is a project's general configuration — sandbox, hooks, model,
// reasoning — rather than a file that exists to hold servers the way .mcp.json does.
// Editing TOML correctly needs a parser this project does not depend on, and appending a
// table to somebody's config without one is how a config gets corrupted. So a file that
// exists is left exactly as it is and the lines to add are reported instead.
func registerTOML(harness, path string, s Server) (string, error) {
	if body, err := os.ReadFile(path); err == nil {
		if strings.Contains(string(body), "[mcp_servers."+s.Name+"]") {
			return "", nil
		}
		return "add this to " + path + " by hand:\n" + tomlTable(s), nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(tomlTable(s)), 0o644); err != nil {
		return "", err
	}
	return "registered " + s.Name + " for " + harness, nil
}

func tomlTable(s Server) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[mcp_servers.%s]\n", s.Name)
	fmt.Fprintf(&b, "command = %q\n", s.Command)
	if len(s.Args) > 0 {
		quoted := make([]string, 0, len(s.Args))
		for _, a := range s.Args {
			quoted = append(quoted, fmt.Sprintf("%q", a))
		}
		fmt.Fprintf(&b, "args = [%s]\n", strings.Join(quoted, ", "))
	}
	return b.String()
}

func entryFor(s Server) map[string]any {
	entry := map[string]any{"command": s.Command}
	if len(s.Args) > 0 {
		entry["args"] = s.Args
	}
	return entry
}

// readJSONObject reads a file as an object, keeping every key it does not understand.
//
// A file that is there but is not an object is somebody else's, or half-written. It is
// reported rather than replaced: the alternative is discarding whatever they meant by it.
func readJSONObject(path string) (map[string]json.RawMessage, error) {
	doc := map[string]json.RawMessage{}
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return doc, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("%s is not a JSON object, so it was left alone: %w", path, err)
	}
	return doc, nil
}

func objectAt(doc map[string]json.RawMessage, key string) (map[string]json.RawMessage, error) {
	raw, ok := doc[key]
	if !ok {
		return map[string]json.RawMessage{}, nil
	}
	var named map[string]json.RawMessage
	if err := json.Unmarshal(raw, &named); err != nil {
		return nil, fmt.Errorf("%q is not an object", key)
	}
	return named, nil
}

func writeJSONObject(path string, doc map[string]json.RawMessage) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o644)
}

// sameEntry compares an entry in a file with one to write by value rather than by text.
//
// The file is written by an indenting encoder, so the same entry read back is not the
// same bytes. Comparing text made every second run think it had work to do.
func sameEntry(existing json.RawMessage, want map[string]any) bool {
	var got map[string]any
	if err := json.Unmarshal(existing, &got); err != nil {
		return false
	}
	return reflect.DeepEqual(got, want)
}

func mustJSON(v any) json.RawMessage {
	body, err := json.Marshal(v)
	if err != nil {
		// Unreachable: every value here came from json.Unmarshal or is a string map.
		panic(err)
	}
	return body
}
