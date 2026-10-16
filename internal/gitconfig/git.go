package gitconfig

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/soppylzz/soppy-utils/internal/configstore"
	"github.com/soppylzz/soppy-utils/internal/core"
	"github.com/soppylzz/soppy-utils/internal/form"
	"github.com/soppylzz/soppy-utils/internal/managedfile"
)

//go:embed schema.json schema-v0.json
var schemaFS embed.FS

type State struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func Store(home string) configstore.Store {
	b, _ := schemaFS.ReadFile("schema.json")
	old, _ := schemaFS.ReadFile("schema-v0.json")
	crlf := "input"
	if runtime.GOOS == "windows" {
		crlf = "true"
	}
	aliases := map[string]any{"st": "status", "co": "checkout", "sw": "switch", "br": "branch", "ci": "commit", "lg": "log", "df": "diff", "ad": "add", "lge": "log --oneline --graph --all --decorate", "cae": "commit --amend --no-edit", "cre": "config --local core.autocrlf " + crlf}
	return configstore.Store{Home: home, Util: "git", Current: 1, Schema: b, OldSchemas: map[int][]byte{0: old}, Defaults: map[string]any{"editor": "code --wait --new-window", "autocrlf": crlf, "diff_tool": "vscode", "merge_tool": "vscode", "aliases": aliases}}
}
func configPath(home string) string {
	return filepath.Join(home, ".config", "soppy-utils", "git-custom-ini.gitconfig")
}
func statePath(s configstore.Store) string { return filepath.Join(s.Dir(), "state.json") }

func render(data map[string]any) ([]byte, error) {
	editor, ok := data["editor"].(string)
	if !ok || strings.ContainsAny(editor, "\r\n\x00") || strings.TrimSpace(editor) == "" {
		return nil, errors.New("invalid editor command")
	}
	crlf, ok := data["autocrlf"].(string)
	if !ok || crlf != "input" && crlf != "true" && crlf != "false" {
		return nil, errors.New("invalid autocrlf")
	}
	diff, ok := data["diff_tool"].(string)
	if !ok || !validValue(diff) {
		return nil, errors.New("invalid diff tool")
	}
	merge, ok := data["merge_tool"].(string)
	if !ok || !validValue(merge) {
		return nil, errors.New("invalid merge tool")
	}
	aliases, ok := data["aliases"].(map[string]any)
	if !ok {
		return nil, errors.New("invalid aliases")
	}
	var b bytes.Buffer
	b.WriteString("# Managed by soppy-utils: git-custom-ini\n[alias]\n")
	keys := make([]string, 0, len(aliases))
	for k := range aliases {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v, ok := aliases[k].(string)
		if !ok || !validAlias(k) || !validValue(v) {
			return nil, fmt.Errorf("invalid alias %q", k)
		}
		fmt.Fprintf(&b, "    %s = %s\n", k, v)
	}
	fmt.Fprintf(&b, "[core]\n    editor = %s\n    autocrlf = %s\n[diff]\n    tool = %s\n[merge]\n    tool = %s\n", editor, crlf, diff, merge)
	b.WriteString("[mergetool \"vscode\"]\n    cmd = code --wait --merge \"$LOCAL\" \"$REMOTE\" \"$BASE\" \"$MERGED\"\n[difftool \"vscode\"]\n    cmd = code --wait --diff \"$LOCAL\" \"$REMOTE\"\n")
	return b.Bytes(), nil
}

func validValue(s string) bool {
	return strings.TrimSpace(s) != "" && !strings.ContainsAny(s, "\r\n\x00")
}
func validAlias(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 'a' || r > 'z' {
			if r < '0' || r > '9' {
				if r != '-' {
					return false
				}
			}
		}
	}
	return true
}

func git(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	return cmd.CombinedOutput()
}
func includes() (map[string]bool, error) {
	b, err := git("config", "--global", "--get-all", "include.path")
	if err != nil && len(b) > 0 {
		return nil, fmt.Errorf("git: %s", strings.TrimSpace(string(b)))
	}
	m := map[string]bool{}
	for _, s := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if s != "" {
			m[s] = true
		}
	}
	return m, nil
}
func equivalent(a, b string) bool {
	if a == b {
		return true
	}
	if runtime.GOOS == "windows" {
		conv, err := exec.Command("cygpath", "-w", a).Output()
		if err == nil {
			a = strings.TrimSpace(string(conv))
		}
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
func included(entries map[string]bool, path string) bool {
	for e := range entries {
		if equivalent(e, path) {
			return true
		}
	}
	return false
}

func LegacyPresent(home string) bool {
	s := Store(home)
	if s.Read().Exists {
		return false
	}
	if err := checkOwned(s); err != nil {
		return false
	}
	if _, err := os.Stat(configPath(home)); err != nil {
		return false
	}
	entries, err := includes()
	return err == nil && included(entries, configPath(home))
}

func checkOwned(s configstore.Store) error {
	path := configPath(s.Home)
	if st, err := os.Lstat(path); err == nil && !st.Mode().IsRegular() {
		return errors.New("managed Git file is not regular")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	stB, err := os.ReadFile(statePath(s))
	if errors.Is(err, os.ErrNotExist) {
		old := legacyRender(s.Defaults["autocrlf"].(string))
		if bytes.Equal(old, b) {
			return nil
		}
		return errors.New("managed Git file has no valid state")
	}
	if err != nil {
		return err
	}
	var st State
	if json.Unmarshal(stB, &st) != nil || !equivalent(st.Path, path) || st.SHA256 != configstore.Hash(b) {
		return errors.New("managed Git file was modified")
	}
	return nil
}
func legacyRender(crlf string) []byte {
	var b bytes.Buffer
	b.WriteString("# Managed by soppy-utils: git-custom-ini\n[alias]\n    st = status\n    co = checkout\n    sw = switch\n    br = branch\n    ci = commit\n    lg = log\n    df = diff\n    ad = add\n    lge = log --oneline --graph --all --decorate\n    cae = commit --amend --no-edit\n")
	fmt.Fprintf(&b, "    cre = config --local core.autocrlf %s\n[core]\n    editor = code --wait --new-window\n    autocrlf = %s\n", crlf, crlf)
	b.WriteString("[diff]\n    tool = vscode\n[merge]\n    tool = vscode\n[mergetool \"vscode\"]\n    cmd = code --wait --merge \"$LOCAL\" \"$REMOTE\" \"$BASE\" \"$MERGED\"\n[difftool \"vscode\"]\n    cmd = code --wait --diff \"$LOCAL\" \"$REMOTE\"\n")
	return b.Bytes()
}
func install(s configstore.Store, data map[string]any, reply core.Responder) error {
	if err := s.Validate(data); err != nil {
		return err
	}
	if err := checkOwned(s); err != nil {
		return err
	}
	rendered, err := render(data)
	if err != nil {
		return err
	}
	parse := exec.Command("git", "config", "--file", "-", "--list")
	parse.Stdin = bytes.NewReader(rendered)
	if out, err := parse.CombinedOutput(); err != nil {
		return fmt.Errorf("generated Git config is invalid: %s: %w", strings.TrimSpace(string(out)), err)
	}
	path := configPath(s.Home)
	return managedfile.WithRollback([]string{path, statePath(s), s.ConfigPath(), s.VersionPath()}, func() error {
		if err := configstore.AtomicWrite(path, rendered, 0600); err != nil {
			return err
		}
		st := State{Path: path, SHA256: configstore.Hash(rendered)}
		b, _ := json.MarshalIndent(st, "", "  ")
		if err := configstore.AtomicWrite(statePath(s), append(b, '\n'), 0600); err != nil {
			return err
		}
		if err := s.Save(data); err != nil {
			return err
		}
		entries, err := includes()
		if err != nil {
			return err
		}
		if !included(entries, path) {
			if b, err := git("config", "--global", "--add", "include.path", path); err != nil {
				return fmt.Errorf("git include: %s: %w", strings.TrimSpace(string(b)), err)
			}
		}
		reply.Notify("installed global defaults")
		return nil
	})
}
func uninstall(s configstore.Store, purge bool, reply core.Responder) error {
	if err := checkOwned(s); err != nil {
		return err
	}
	path := configPath(s.Home)
	entries, err := includes()
	if err != nil {
		return err
	}
	for entry := range entries {
		if equivalent(entry, path) {
			if b, err := git("config", "--global", "--fixed-value", "--unset-all", "include.path", entry); err != nil {
				return fmt.Errorf("git include: %s: %w", strings.TrimSpace(string(b)), err)
			}
		}
	}
	os.Remove(path)
	os.Remove(statePath(s))
	if purge {
		os.Remove(s.ConfigPath())
		os.Remove(s.VersionPath())
	}
	return nil
}

// Run performs one git command.
//
// It writes what it wants shown and asks through reply; it never touches a terminal
// itself. Git stays the authority for the configuration: this writes a managed file and
// asks git to include it.
func Run(ctx context.Context, session core.Session, args []string, out io.Writer, reply core.Responder) error {
	home := session.Home
	s := Store(home)
	v := s.Read()
	if v.Warning != "" {
		if err := reply.Warn(v.Warning); err != nil {
			return err
		}
	}
	if len(args) == 0 {
		fmt.Fprintln(out, "git: install, uninstall, edit, migrate, status")
		return nil
	}
	switch args[0] {
	case "status":
		entries, err := includes()
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "config contract: %d (required %d)\n", v.StoredVersion, s.Current)
		fmt.Fprintf(out, "global include: %t\n", included(entries, configPath(home)))
		return nil
	case "uninstall":
		if len(args) > 2 || len(args) == 2 && args[1] != "--purge" {
			return errors.New("unexpected uninstall option")
		}
		if len(args) == 2 && !v.Writable {
			return errors.New("cannot purge conflicted or future config")
		}
		return managedfile.WithLock(filepath.Dir(configPath(home)), func() error { return uninstall(s, len(args) == 2, reply) })
	case "install", "edit", "migrate":
	default:
		return fmt.Errorf("unknown git action %q", args[0])
	}
	if !v.Writable {
		if len(args) == 3 && args[0] == "migrate" && args[1] == "--answers" {
			data, err := s.LoadAnswers(args[2])
			if err != nil {
				return err
			}
			return managedfile.WithLock(filepath.Dir(configPath(home)), func() error { return s.Save(data) })
		}
		return errors.New("cannot write with conflicted or future config")
	}
	fs := flag.NewFlagSet("git", flag.ContinueOnError)
	// The message belongs on the client's stderr, and it gets there as the returned
	// error. Writing it here would put it in the daemon's log instead.
	fs.SetOutput(io.Discard)
	answers := fs.String("answers", "", "")
	editor := fs.String("editor", v.Data["editor"].(string), "")
	crlf := fs.String("autocrlf", v.Data["autocrlf"].(string), "")
	diff := fs.String("diff-tool", v.Data["diff_tool"].(string), "")
	merge := fs.String("merge-tool", v.Data["merge_tool"].(string), "")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected arguments")
	}
	if *answers != "" {
		data, err := s.LoadAnswers(*answers)
		if err != nil {
			return err
		}
		if args[0] == "migrate" {
			if _, err := os.Stat(configPath(home)); errors.Is(err, os.ErrNotExist) {
				return managedfile.WithLock(filepath.Dir(configPath(home)), func() error { return s.Save(data) })
			}
		}
		return install(s, data, reply)
	}

	data := map[string]any{}
	for key, value := range v.Data {
		data[key] = value
	}
	data["editor"], data["autocrlf"] = *editor, *crlf
	data["diff_tool"], data["merge_tool"] = *diff, *merge

	if (args[0] == "edit" || args[0] == "migrate") && reply.CanAsk() {
		// The form is the schema's, so the aliases arrive as a map rather than as a blob
		// of JSON a text box handed back for this code to parse.
		spec, err := form.FromSchema(s.Schema, data)
		if err != nil {
			return err
		}
		answered, err := reply.Ask(ctx, spec)
		if err != nil {
			return err
		}
		for key, value := range answered {
			data[key] = value
		}
	}
	if args[0] == "migrate" {
		if _, err := os.Stat(configPath(home)); err == nil {
			return install(s, data, reply)
		}
		return managedfile.WithLock(filepath.Dir(configPath(home)), func() error { return s.Save(data) })
	}
	if args[0] == "edit" && !reply.CanAsk() && len(args) == 1 {
		return errors.New("interactive terminal required")
	}
	return install(s, data, reply)
}
