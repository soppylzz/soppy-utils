package license

import (
	"bytes"
	"embed"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"charm.land/huh/v2"
	"github.com/soppylzz/soppy-utils/internal/clioutput"
	"github.com/soppylzz/soppy-utils/internal/configstore"
	"github.com/soppylzz/soppy-utils/internal/managedfile"
	"github.com/soppylzz/soppy-utils/internal/ui"
)

//go:embed schema.json schema-v0.json templates/*.tmpl
var assets embed.FS

func Store(home string) configstore.Store {
	schema, _ := assets.ReadFile("schema.json")
	old, _ := assets.ReadFile("schema-v0.json")
	return configstore.Store{Home: home, Util: "license", Current: 1, Schema: schema, OldSchemas: map[int][]byte{0: old}, Defaults: map[string]any{"type": "mit", "author": "", "output": "LICENSE"}}
}

func author() string {
	if b, err := exec.Command("git", "config", "user.name").Output(); err == nil && strings.TrimSpace(string(b)) != "" {
		return strings.TrimSpace(string(b))
	}
	if s := os.Getenv("USER"); s != "" {
		return s
	}
	return os.Getenv("USERNAME")
}

func Render(kind, name string, year int) ([]byte, error) {
	if kind != "mit" && kind != "isc" {
		return nil, errors.New("type must be mit or isc")
	}
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("author name is required")
	}
	if year < 1000 || year > 9999 {
		return nil, errors.New("year must have four digits")
	}
	b, err := assets.ReadFile("templates/" + kind + ".tmpl")
	if err != nil {
		return nil, err
	}
	t, err := template.New(kind).Option("missingkey=error").Parse(string(b))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	err = t.Execute(&out, struct {
		Name string
		Year int
	}{name, year})
	return out.Bytes(), err
}

func savePreferences(s configstore.Store, data map[string]any) error {
	return managedfile.WithLock(s.Dir(), func() error { return s.Save(data) })
}

func Run(home string, args []string) error {
	s := Store(home)
	v := s.Read()
	if v.Warning != "" {
		clioutput.Warn("license", v.Warning)
	}
	if len(args) > 0 {
		switch args[0] {
		case "purge":
			if len(args) != 1 {
				return errors.New("purge takes no options")
			}
			if !v.Writable {
				return errors.New("cannot purge conflicted or future config")
			}
			return managedfile.WithLock(s.Dir(), func() error {
				if err := os.Remove(s.ConfigPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
				if err := os.Remove(s.VersionPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
				return nil
			})
		case "--help":
			fmt.Println("Usage: spu license [--mit|--isc] [--name NAME] [--year YEAR] [--output FILE]\n       spu license edit|migrate [--answers FILE]")
			return nil
		case "-h":
			fmt.Println("license: generate an MIT or ISC license")
			return nil
		case "edit", "migrate":
			return edit(s, v, args)
		}
	}
	fs := flag.NewFlagSet("license", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	mit := fs.Bool("mit", false, "")
	isc := fs.Bool("isc", false, "")
	M := fs.Bool("M", false, "")
	I := fs.Bool("I", false, "")
	output := fs.String("output", "", "")
	fs.StringVar(output, "o", "", "")
	name := fs.String("name", "", "")
	year := fs.Int("year", time.Now().Year(), "")
	interactive := fs.Bool("interactive", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected arguments")
	}
	if (*mit || *M) && (*isc || *I) {
		return errors.New("conflicting license types")
	}
	kind := ""
	if *mit || *M {
		kind = "mit"
	}
	if *isc || *I {
		kind = "isc"
	}
	if kind == "" || *interactive {
		if !ui.Available() {
			if *interactive {
				return errors.New("interactive terminal required")
			}
			fmt.Println("Usage: spu license --mit|--isc [--name NAME] [--year YEAR] [--output FILE]")
			return nil
		}
		if !v.Writable {
			return errors.New("cannot write with invalid or future config")
		}
		if kind == "" {
			kind = v.Data["type"].(string)
		}
		if *name == "" {
			*name = v.Data["author"].(string)
		}
		if *name == "" {
			*name = author()
		}
		if *output == "" {
			*output = v.Data["output"].(string)
		}
		yearText := strconv.Itoa(*year)
		save := false
		err := ui.Run(huh.NewGroup(
			huh.NewSelect[string]().Title("License").Options(huh.NewOption("MIT", "mit"), huh.NewOption("ISC", "isc")).Value(&kind),
			huh.NewInput().Title("Author").Value(name).Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("author required")
				}
				return nil
			}),
			huh.NewInput().Title("Year").Value(&yearText).Validate(func(s string) error {
				n, e := strconv.Atoi(s)
				if e != nil || n < 1000 || n > 9999 {
					return errors.New("use a four-digit year")
				}
				return nil
			}),
			huh.NewInput().Title("Output file").Value(output),
			huh.NewConfirm().Title("Save these defaults?").Value(&save),
		))
		if err != nil {
			return err
		}
		clioutput.Note(fmt.Sprintf("Generate %s license for %s (%s) at %s", strings.ToUpper(kind), *name, yearText, *output))
		confirm := false
		if err := ui.Run(huh.NewGroup(huh.NewConfirm().Title("Generate license?").Value(&confirm))); err != nil {
			return err
		}
		if !confirm {
			return nil
		}
		*year, _ = strconv.Atoi(yearText)
		if *output == "" {
			return errors.New("output file is required")
		}
		if _, err := os.Stat(*output); err == nil {
			replace := false
			if err := ui.Run(huh.NewGroup(huh.NewConfirm().Title("Replace existing file?").Value(&replace))); err != nil {
				return err
			}
			if !replace {
				return nil
			}
		}
		if save {
			data := map[string]any{"type": kind, "author": *name, "output": *output}
			if err := savePreferences(s, data); err != nil {
				return err
			}
		}
	}
	if *name == "" {
		*name = v.Data["author"].(string)
	}
	if *name == "" {
		*name = author()
	}
	body, err := Render(kind, *name, *year)
	if err != nil {
		return err
	}
	if *output == "" {
		_, err = os.Stdout.Write(body)
		return err
	}
	return configstore.AtomicWrite(*output, body, 0644)
}

func edit(s configstore.Store, v configstore.View, args []string) error {
	if !v.Writable {
		if len(args) == 3 && args[0] == "migrate" && args[1] == "--answers" {
			data, err := s.LoadAnswers(args[2])
			if err != nil {
				return err
			}
			return savePreferences(s, data)
		}
		return errors.New("cannot migrate conflicted or future config")
	}
	fs := flag.NewFlagSet("license edit", flag.ContinueOnError)
	answers := fs.String("answers", "", "")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	data := v.Data
	if *answers != "" {
		resolved, err := s.LoadAnswers(*answers)
		if err != nil {
			return err
		}
		return savePreferences(s, resolved)
	}
	kind := data["type"].(string)
	name := data["author"].(string)
	output := data["output"].(string)
	if name == "" {
		name = author()
	}
	if !ui.Available() {
		if args[0] == "migrate" {
			return savePreferences(s, data)
		}
		return errors.New("interactive terminal required")
	}
	err := ui.Run(huh.NewGroup(
		huh.NewSelect[string]().Title("Preferred license").Options(huh.NewOption("MIT", "mit"), huh.NewOption("ISC", "isc")).Value(&kind),
		huh.NewInput().Title("Default author").Value(&name),
		huh.NewInput().Title("Default output path").Value(&output),
	))
	if err != nil {
		return err
	}
	if output == "" {
		return errors.New("output path required")
	}
	data = map[string]any{"type": kind, "author": name, "output": filepath.Clean(output)}
	return savePreferences(s, data)
}
