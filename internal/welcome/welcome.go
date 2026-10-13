package welcome

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"charm.land/huh/v2"
	"github.com/soppylzz/soppy-utils/internal/clioutput"
	"github.com/soppylzz/soppy-utils/internal/configstore"
	"github.com/soppylzz/soppy-utils/internal/managedfile"
	"github.com/soppylzz/soppy-utils/internal/ui"
)

//go:embed schema.json schema-v0.json templates/*.tmpl
var assets embed.FS

const start = "# >>> soppy-utils welcome >>>"
const end = "# <<< soppy-utils welcome <<<"
const templateRevision = 5
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
	return configstore.Store{Home: home, Util: "welcome", Current: 1, Schema: schema, OldSchemas: map[int][]byte{0: old}, Defaults: map[string]any{"name": name, "device": device, "message": "Ready to hack the universe.", "rc": "none"}}
}

// ShellQuote encodes data as one Bash single-quoted word.
func ShellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }

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

func install(s configstore.Store, data map[string]any) error {
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
			clioutput.Warn("welcome", "legacy welcome hook remains in ~/.zshrc")
		}
		detectOptionalTools(s.Home).warnMissing()
		clioutput.Info("welcome", "generated "+scriptPath(s))
		return nil
	})
}

func uninstall(s configstore.Store, purge bool) error {
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

func Run(home string, args []string) error {
	s := Store(home)
	v := s.Read()
	if v.Warning != "" {
		clioutput.Warn("welcome", v.Warning)
	}
	if len(args) == 0 {
		fmt.Println("welcome: install, uninstall, edit, migrate, status")
		return nil
	}
	switch args[0] {
	case "-h":
		fmt.Println("welcome: generate an interactive shell banner")
		return nil
	case "--help":
		fmt.Println("Usage: spu welcome install [--name NAME] [--device DEVICE] [--message TEXT] [--rc none|bash|zsh|both]\n       spu welcome uninstall [--purge]\n       spu welcome edit|migrate|status")
		return nil
	case "status":
		fmt.Printf("config contract: %d (required %d)\n", v.StoredVersion, s.Current)
		if err := checkOwned(s); err != nil {
			fmt.Println("script:", err)
		} else if _, err := os.Stat(scriptPath(s)); err == nil {
			var st State
			raw, _ := os.ReadFile(statePath(s))
			_ = json.Unmarshal(raw, &st)
			fmt.Printf("script: managed, schema %d, template revision %d (current %d)\n", st.ScriptSchema, st.TemplateRevision, templateRevision)
			if st.TemplateRevision != templateRevision {
				fmt.Println("refresh: run 'spu welcome install'")
			}
		} else {
			fmt.Println("script: absent")
		}
		if LegacyChecksum(home) {
			fmt.Println("legacy script: valid; run 'spu welcome migrate'")
		}
		detectOptionalTools(home).printStatus()
		return nil
	case "uninstall":
		if len(args) > 2 || len(args) == 2 && args[1] != "--purge" {
			return errors.New("unexpected uninstall option")
		}
		if len(args) == 2 && !v.Writable {
			return errors.New("cannot purge conflicted or future config")
		}
		return managedfile.WithLock(s.Dir(), func() error { return uninstall(s, len(args) == 2) })
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
		if args[0] == "migrate" && LegacyChecksum(home) && !ui.Available() && len(args) == 1 {
			return errors.New("legacy migration needs a terminal or --answers file")
		}
		if *interactive && !ui.Available() {
			return errors.New("interactive terminal required")
		}
		if (args[0] == "edit" || args[0] == "migrate" || *interactive || args[0] == "install" && len(args) == 1) && ui.Available() {
			err := ui.Run(huh.NewGroup(
				huh.NewInput().Title("Banner name").Value(name), huh.NewInput().Title("Device name").Value(device),
				huh.NewInput().Title("Message").Value(message),
				huh.NewSelect[string]().Title("Startup hook").Options(huh.NewOption("None", "none"), huh.NewOption("Bash", "bash"), huh.NewOption("Zsh", "zsh"), huh.NewOption("Both", "both")).Value(rc),
			))
			if err != nil {
				return err
			}
		}
		data = map[string]any{"name": *name, "device": *device, "message": *message, "rc": *rc}
	}
	if args[0] == "migrate" {
		if LegacyChecksum(home) {
			if err := install(s, data); err != nil {
				return err
			}
			base := filepath.Join(home, ".config", "soppy-utils")
			os.Remove(filepath.Join(base, "welcome.sh"))
			os.Remove(filepath.Join(base, "welcome.cksum"))
			return nil
		}
		if _, err := os.Stat(scriptPath(s)); err == nil {
			return install(s, data)
		}
		return managedfile.WithLock(s.Dir(), func() error { return s.Save(data) })
	}
	return install(s, data)
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
