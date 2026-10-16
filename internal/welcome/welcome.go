package welcome

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
	"strings"
	"text/template"

	"github.com/soppylzz/soppy-utils/internal/configstore"
	"github.com/soppylzz/soppy-utils/internal/core"
	"github.com/soppylzz/soppy-utils/internal/form"
	"github.com/soppylzz/soppy-utils/internal/managedfile"
)

//go:embed schema.json schema-v1.json schema-v0.json templates/*.tmpl
var assets embed.FS

const start = "# >>> soppy-utils welcome >>>"
const end = "# <<< soppy-utils welcome <<<"
const templateRevision = 6
const hook = start + "\ncase $- in\n  *i*)\n    if [ -r \"$HOME/.config/soppy-utils/welcome/welcome.sh\" ]; then\n      SPU_WELCOME_INTERACTIVE=1 bash \"$HOME/.config/soppy-utils/welcome/welcome.sh\"\n    fi\n    ;;\nesac\n" + end + "\n"
const legacyHook = start + "\ncase $- in\n  *i*)\n    if [ -r \"$HOME/.config/soppy-utils/welcome.sh\" ]; then\n      . \"$HOME/.config/soppy-utils/welcome.sh\"\n    fi\n    ;;\nesac\n" + end + "\n"

type State struct {
	ScriptSchema     int    `json:"script_schema"`
	TemplateRevision int    `json:"template_revision"`
	ScriptSHA256     string `json:"script_sha256"`
	ProducedBy       string `json:"produced_by"`
}

var Version = "dev"

func Store(home string) configstore.Store {
	schema, _ := assets.ReadFile("schema.json")
	previous, _ := assets.ReadFile("schema-v1.json")
	old, _ := assets.ReadFile("schema-v0.json")
	name := os.Getenv("USER")
	if name == "" {
		name = os.Getenv("USERNAME")
	}
	if name == "" {
		name = "developer"
	}
	device, _ := os.Hostname()
	if device == "" {
		device = "computer"
	}
	return configstore.Store{
		Home: home, Util: "welcome", Current: 2, Schema: schema,
		OldSchemas: map[int][]byte{0: old, 1: previous},
		Defaults: map[string]any{
			"name": name, "device": device,
			"message": "Ready to hack the universe.", "rc": "none",
			// Absent from a v1 file, so this is also the whole of the migration: a
			// stored configuration keeps its values, gains this one, and is warned about
			// until the user chooses to persist it.
			"banner_in_tmux": false,
		},
	}
}

// ShellQuote encodes data as one Bash single-quoted word.
func ShellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }

// bannerInTmux reports whether the banner prints inside tmux.
//
// An absent value is false, which is the default: one banner per pane is not what
// anybody wants.
func bannerInTmux(data map[string]any) bool {
	inTmux, _ := data["banner_in_tmux"].(bool)
	return inTmux
}

func Render(data map[string]any) ([]byte, error) {
	body, err := assets.ReadFile("templates/banner.sh.tmpl")
	if err != nil {
		return nil, err
	}
	t, err := template.New("banner").Parse(string(body))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	// Powerlevel10k can redirect all standard streams while an interactive shell loads its rc file.
	out.WriteString(`#!/usr/bin/env bash
# Managed by soppy-utils: welcome
if [[ ${SPU_WELCOME_INTERACTIVE:-} != 1 ]]; then exit 0; fi
if [[ ! -t 0 && ! -t 1 && ! -t 2 ]] && ! (: </dev/tty) 2>/dev/null; then exit 0; fi
`)
	// Baked into the script rather than looked up at startup: the generated banner runs
	// in the user's shell, and reading configuration there would make it depend on spu
	// being installed and working.
	if !bannerInTmux(data) {
		out.WriteString("# Every new tmux pane runs the startup file too, and a banner in each of them is\n" +
			"# not what anybody wants, so the banner stays out of tmux unless it was asked for.\n" +
			"if [[ -n ${TMUX:-} ]]; then exit 0; fi\n")
	}
	for _, key := range []string{"name", "device", "message"} {
		v, ok := data[key].(string)
		if !ok {
			return nil, fmt.Errorf("invalid %s", key)
		}
		fmt.Fprintf(&out, "_spu_welcome_%s=%s\n", key, ShellQuote(v))
	}
	if err := t.Execute(&out, nil); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func selected(rc, kind string) bool   { return rc == "both" || rc == kind }
func rcPath(home, kind string) string { return filepath.Join(home, "."+kind+"rc") }

func checkMarkers(b []byte) (bool, error) {
	n1 := strings.Count(string(b), start)
	n2 := strings.Count(string(b), end)
	if n1 == 0 && n2 == 0 {
		return false, nil
	}
	if n1 != 1 || n2 != 1 || strings.Index(string(b), start) > strings.Index(string(b), end) {
		return false, errors.New("incomplete or duplicate welcome markers")
	}
	return true, nil
}

func p10kPreambleStart(b []byte) int {
	text := string(b)
	offset := 0
	for _, line := range strings.SplitAfter(text, "\n") {
		if strings.HasPrefix(line, "if [[ -r ") && strings.Contains(line, "p10k-instant-prompt-") {
			header := strings.LastIndex(text[:offset], "# Enable Powerlevel10k instant prompt.")
			if header >= 0 {
				comments := text[header:offset]
				valid := true
				for _, comment := range strings.Split(comments, "\n") {
					trimmed := strings.TrimSpace(comment)
					if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
						valid = false
						break
					}
				}
				if valid {
					return strings.LastIndex(text[:header], "\n") + 1
				}
			}
			return offset
		}
		offset += len(line)
	}
	return -1
}

func editHook(path string, want bool) error {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		b = nil
	} else if err != nil {
		return err
	}
	has, err := checkMarkers(b)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if has && !strings.Contains(string(b), hook) && !strings.Contains(string(b), legacyHook) {
		return fmt.Errorf("%s: managed hook was modified", path)
	}
	if want && has && strings.Contains(string(b), hook) {
		if filepath.Base(path) != ".zshrc" {
			return nil
		}
		position := p10kPreambleStart(b)
		if position < 0 || strings.Index(string(b), hook) < position {
			return nil
		}
	}
	if !want && !has {
		return nil
	}
	original := append([]byte(nil), b...)
	if has {
		from := strings.Index(string(b), start)
		to := strings.Index(string(b), end) + len(end)
		if to < len(b) && b[to] == '\n' {
			to++
		}
		b = append(append([]byte{}, b[:from]...), b[to:]...)
	}
	if want {
		position := -1
		if filepath.Base(path) == ".zshrc" {
			position = p10kPreambleStart(b)
		}
		if position >= 0 {
			b = append(append(append([]byte{}, b[:position]...), []byte(hook+"\n")...), b[position:]...)
		} else {
			if len(b) > 0 && b[len(b)-1] != '\n' {
				b = append(b, '\n')
			}
			b = append(b, []byte(hook)...)
		}
	}
	if bytes.Equal(original, b) {
		return nil
	}
	return configstore.AtomicWrite(path, b, 0600)
}

func statePath(s configstore.Store) string  { return filepath.Join(s.Dir(), "state.json") }
func scriptPath(s configstore.Store) string { return filepath.Join(s.Dir(), "welcome.sh") }
func checkOwned(s configstore.Store) error {
	if st, err := os.Lstat(scriptPath(s)); err == nil && !st.Mode().IsRegular() {
		return errors.New("welcome script is not a regular managed file")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	b, err := os.ReadFile(scriptPath(s))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(statePath(s))
	if err != nil {
		return errors.New("welcome script is unmanaged")
	}
	var st State
	if json.Unmarshal(raw, &st) != nil || st.ScriptSchema != 1 || st.TemplateRevision < 1 || st.ScriptSHA256 != configstore.Hash(b) {
		return errors.New("welcome script was modified or has an unsupported schema")
	}
	return nil
}

func install(s configstore.Store, data map[string]any, reply core.Responder) error {
	if err := s.Validate(data); err != nil {
		return err
	}
	if err := checkOwned(s); err != nil {
		return err
	}
	for _, kind := range []string{"bash", "zsh"} {
		b, err := os.ReadFile(rcPath(s.Home, kind))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if _, err = checkMarkers(b); err != nil {
			return fmt.Errorf("%s: %w", kind, err)
		}
	}
	rendered, err := Render(data)
	if err != nil {
		return err
	}
	paths := []string{scriptPath(s), statePath(s), s.ConfigPath(), s.VersionPath(), rcPath(s.Home, "bash"), rcPath(s.Home, "zsh")}
	return managedfile.WithRollback(paths, func() error {
		if err := configstore.AtomicWrite(scriptPath(s), rendered, 0700); err != nil {
			return err
		}
		st := State{ScriptSchema: 1, TemplateRevision: templateRevision, ScriptSHA256: configstore.Hash(rendered), ProducedBy: Version}
		b, _ := json.MarshalIndent(st, "", "  ")
		if err := configstore.AtomicWrite(statePath(s), append(b, '\n'), 0600); err != nil {
			return err
		}
		if err := s.Save(data); err != nil {
			return err
		}
		rc := data["rc"].(string)
		for _, kind := range []string{"bash", "zsh"} {
			if err := editHook(rcPath(s.Home, kind), selected(rc, kind)); err != nil {
				return err
			}
		}
		zshrc, _ := os.ReadFile(rcPath(s.Home, "zsh"))
		if strings.Contains(string(zshrc), "$SOPPY/shell/welcome.sh") {
			reply.Warn("legacy welcome hook remains in ~/.zshrc")
		}
		reply.Notify("generated " + scriptPath(s))
		return nil
	})
}

func uninstall(s configstore.Store, purge bool, reply core.Responder) error {
	if err := checkOwned(s); err != nil {
		return err
	}
	for _, kind := range []string{"bash", "zsh"} {
		b, err := os.ReadFile(rcPath(s.Home, kind))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if _, err = checkMarkers(b); err != nil {
			return err
		}
	}
	for _, kind := range []string{"bash", "zsh"} {
		if err := editHook(rcPath(s.Home, kind), false); err != nil {
			return err
		}
	}
	os.Remove(scriptPath(s))
	os.Remove(statePath(s))
	if purge {
		os.Remove(s.ConfigPath())
		os.Remove(s.VersionPath())
	}
	return nil
}

// Run performs one welcome command.
//
// It writes what it wants shown and asks through reply; it never touches a terminal
// itself and never prints to this process's output. That is the whole of the
// arrangement: the same code serves a terminal client and a window, because neither is
// visible from here, and the same code runs under a test with no terminal at all.
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
		fmt.Fprintln(out, "welcome: install, uninstall, edit, migrate, status")
		return nil
	}
	switch args[0] {
	case "status":
		fmt.Fprintf(out, "config contract: %d (required %d)\n", v.StoredVersion, s.Current)
		if err := checkOwned(s); err != nil {
			fmt.Fprintln(out, "script:", err)
		} else if _, err := os.Stat(scriptPath(s)); err == nil {
			var st State
			raw, _ := os.ReadFile(statePath(s))
			_ = json.Unmarshal(raw, &st)
			fmt.Fprintf(out, "script: managed, schema %d, template revision %d (current %d)\n", st.ScriptSchema, st.TemplateRevision, templateRevision)
			if st.TemplateRevision != templateRevision {
				fmt.Fprintln(out, "refresh: run 'spu welcome install'")
			}
		} else {
			fmt.Fprintln(out, "script: absent")
		}
		if LegacyChecksum(home) {
			fmt.Fprintln(out, "legacy script: valid; run 'spu welcome migrate'")
		}
		detectOptionalTools(home).printStatus(out)
		return nil
	case "uninstall":
		if len(args) > 2 || len(args) == 2 && args[1] != "--purge" {
			return errors.New("unexpected uninstall option")
		}
		if len(args) == 2 && !v.Writable {
			return errors.New("cannot purge conflicted or future config")
		}
		return managedfile.WithLock(s.Dir(), func() error { return uninstall(s, len(args) == 2, reply) })
	case "install", "edit", "migrate":
	default:
		return fmt.Errorf("unknown welcome action %q", args[0])
	}
	if !v.Writable {
		if len(args) == 3 && args[0] == "migrate" && args[1] == "--answers" {
			data, err := s.LoadAnswers(args[2])
			if err != nil {
				return err
			}
			return managedfile.WithLock(s.Dir(), func() error { return s.Save(data) })
		}
		return errors.New("cannot write with conflicted or future config")
	}
	data := v.Data
	if args[0] == "install" && !v.Exists && LegacyChecksum(home) {
		return errors.New("legacy welcome state found; run 'spu welcome migrate'")
	}
	fs := flag.NewFlagSet("welcome", flag.ContinueOnError)
	answers := fs.String("answers", "", "")
	name := fs.String("name", data["name"].(string), "")
	device := fs.String("device", data["device"].(string), "")
	message := fs.String("message", data["message"].(string), "")
	rc := fs.String("rc", data["rc"].(string), "")
	showInTmux := fs.Bool("banner-in-tmux", bannerInTmux(data), "")
	interactive := fs.Bool("interactive", false, "")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected arguments")
	}
	if *answers != "" {
		resolved, err := s.LoadAnswers(*answers)
		if err != nil {
			return err
		}
		data = resolved
	} else {
		// Whether a terminal is attached is a fact about the client, not about this
		// process: core runs detached and cannot see one. So the client is asked
		// whether it can ask.
		canAsk := reply.CanAsk()
		if args[0] == "migrate" && LegacyChecksum(home) && !canAsk && len(args) == 1 {
			return errors.New("legacy migration needs a terminal or --answers file")
		}
		if *interactive && !canAsk {
			return errors.New("interactive terminal required")
		}

		// The flags are the starting values, exactly as they were when the form was
		// built here.
		data = map[string]any{}
		for key, value := range v.Data {
			data[key] = value
		}
		data["name"], data["device"], data["message"], data["rc"] = *name, *device, *message, *rc
		data["banner_in_tmux"] = *showInTmux

		shouldAsk := args[0] == "edit" || args[0] == "migrate" || *interactive ||
			args[0] == "install" && len(args) == 1
		if shouldAsk && canAsk {
			// The form is projected from the schema rather than written out here, so it
			// cannot describe a field the validation does not know about or miss one it
			// does. A field added to the schema appears in the form by itself.
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
	}
	installed := false
	switch {
	case args[0] == "migrate" && LegacyChecksum(home):
		if err := install(s, data, reply); err != nil {
			return err
		}
		base := filepath.Join(home, ".config", "soppy-utils")
		os.Remove(filepath.Join(base, "welcome.sh"))
		os.Remove(filepath.Join(base, "welcome.cksum"))
		installed = true

	case args[0] == "migrate":
		if _, err := os.Stat(scriptPath(s)); err == nil {
			if err := install(s, data, reply); err != nil {
				return err
			}
			installed = true
		} else if err := managedfile.WithLock(s.Dir(), func() error { return s.Save(data) }); err != nil {
			return err
		}

	default:
		if err := install(s, data, reply); err != nil {
			return err
		}
		installed = true
	}

	if !installed {
		return nil
	}
	return offerDependencies(ctx, session, s, reply)
}

// offerDependencies tells the client what the banner could use and cannot find.
//
// A client that cannot run anything is told the same thing rather than nothing: the
// warning still has to reach a human reading the output, and the session continues. A
// dependency that cannot be installed leaves a util degraded, which is a legitimate
// state rather than a failure.
func offerDependencies(ctx context.Context, session core.Session, s configstore.Store, reply core.Responder) error {
	req := dependencyActions(session.GOOS, detectOptionalTools(s.Home))
	if req.Reason == "" {
		return nil
	}
	_, err := reply.AskActions(ctx, req)
	return err
}

// LegacyChecksum reports whether a previous generated banner still has its recorded cksum.
func LegacyChecksum(home string) bool {
	base := filepath.Join(home, ".config", "soppy-utils")
	script, err := os.ReadFile(filepath.Join(base, "welcome.sh"))
	if err != nil || !bytes.Contains(script, []byte("# Managed by soppy-utils: welcome")) {
		return false
	}
	stored, err := os.ReadFile(filepath.Join(base, "welcome.cksum"))
	if err != nil {
		return false
	}
	cmd := exec.Command("cksum")
	cmd.Stdin = bytes.NewReader(script)
	actual, err := cmd.Output()
	return err == nil && bytes.Equal(bytes.TrimSpace(actual), bytes.TrimSpace(stored))
}
