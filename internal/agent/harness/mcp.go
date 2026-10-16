package harness

import (
	"encoding/json"

	"fmt"
	"github.com/BurntSushi/toml"
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
		for _, s := range servers {
			var what string
			var err error
			if target.toml {
				what, err = unregisterTOML(target.path, s)
			} else {
				what, err = unregisterJSON(target.path, s)
			}
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

// registerTOML adds a table to a Codex registration file.
//
// `.codex/config.toml` is a project's general configuration — sandbox, hooks, model,
// reasoning — so it is somebody's file with their own comments and ordering in it. The
// entry is therefore appended to the text rather than merged by re-encoding: decoding into
// a map and encoding it back would produce a valid file that had lost everything the
// person put there for their own reasons.
//
// That makes the parser a way to prove the result is still valid rather than a way to
// produce it. Nothing is written until the parser has read the file and then read the
// result, so a file that was already broken is left alone and a file this would break is
// never written.
func registerTOML(harness, path string, s Server) (string, error) {
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, []byte(tomlTable(s)), 0o644); err != nil {
			return "", err
		}
		return "registered " + s.Name + " for " + harness, nil
	}
	if err != nil {
		return "", err
	}

	var parsed map[string]any
	if _, err := toml.Decode(string(body), &parsed); err != nil {
		return "", fmt.Errorf("%s is not valid TOML, so it was left alone: %w", path, err)
	}

	if entry, ok := tomlEntry(parsed, s.Name); ok {
		if equalEntry(entry, s) {
			return "", nil
		}
		return "\n[mcp_servers." + s.Name + "] is already in " + path + " with different " +
			"values; change it by hand if it should point at " + s.Command, nil
	}

	next := appendTable(string(body), s)
	if _, err := toml.Decode(next, &parsed); err != nil {
		return "", fmt.Errorf("adding [mcp_servers.%s] would leave %s invalid (%w), so it "+
			"was left alone", s.Name, path, err)
	}
	if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
		return "", err
	}
	return "registered " + s.Name + " for " + harness, nil
}

// unregisterTOML takes our table back out, and nothing else.
//
// The table is cut out as text, for the same reason it is appended as text, and the
// result is parsed before it is written.
func unregisterTOML(path string, s Server) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	next, removed := removeTable(string(body), s.Name)
	if !removed {
		return "", nil
	}
	var parsed map[string]any
	if _, err := toml.Decode(next, &parsed); err != nil {
		return "", fmt.Errorf("removing [mcp_servers.%s] would leave %s invalid (%w), so it "+
			"was left alone", s.Name, path, err)
	}
	if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
		return "", err
	}
	return "removed " + s.Name + " from " + path, nil
}

// tomlEntry finds a registered server in a parsed file.
func tomlEntry(parsed map[string]any, name string) (map[string]any, bool) {
	named, ok := parsed["mcp_servers"].(map[string]any)
	if !ok {
		return nil, false
	}
	entry, ok := named[name].(map[string]any)
	return entry, ok
}

func equalEntry(entry map[string]any, s Server) bool {
	if command, _ := entry["command"].(string); command != s.Command {
		return false
	}
	return reflect.DeepEqual(stringList(entry["args"]), s.Args)
}

func stringList(v any) []string {
	switch got := v.(type) {
	case nil:
		return nil
	case []string:
		return got
	case []any:
		out := make([]string, 0, len(got))
		for _, item := range got {
			text, ok := item.(string)
			if !ok {
				return nil
			}
			out = append(out, text)
		}
		return out
	default:
		return nil
	}
}

// appendTable adds the table at the end, separated from whatever is above it.
func appendTable(body string, s Server) string {
	if strings.TrimSpace(body) == "" {
		return tomlTable(s)
	}
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return body + "\n" + tomlTable(s)
}

// removeTable cuts a table and its body out of a TOML document.
//
// A table runs from its header to the next header, so that is the span removed. Whatever
// separated it from its neighbours is tidied back to a single blank line, because the
// blank lines around an appended table were put there by the append.
func removeTable(body, name string) (string, bool) {
	header := tomlHeader(name)
	lines := strings.Split(body, "\n")

	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == header {
			start = i
			break
		}
	}
	if start < 0 {
		return body, false
	}

	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "[") {
			end = i
			break
		}
	}

	out := append(append([]string{}, lines[:start]...), lines[end:]...)
	joined := strings.Join(out, "\n")
	for strings.Contains(joined, "\n\n\n") {
		joined = strings.ReplaceAll(joined, "\n\n\n", "\n\n")
	}
	return joined, true
}

func tomlHeader(name string) string {
	return "[mcp_servers." + name + "]"
}

func tomlTable(s Server) string {
	var b strings.Builder
	fmt.Fprintln(&b, tomlHeader(s.Name))
	entry := map[string]any{"command": s.Command}
	if len(s.Args) > 0 {
		entry["args"] = s.Args
	}
	// Encoded rather than formatted by hand so that a value needing quotes or escapes
	// gets them.
	_ = toml.NewEncoder(&b).Encode(entry)
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
