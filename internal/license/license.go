package license

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/soppylzz/soppy-utils/internal/configstore"
	"github.com/soppylzz/soppy-utils/internal/core"
	"github.com/soppylzz/soppy-utils/internal/form"
	"github.com/soppylzz/soppy-utils/internal/managedfile"
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

func render(kind, name string, year int) ([]byte, error) {
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

// Run performs one license command.
//
// It writes what it wants shown and asks through reply; it never touches a terminal
// itself. The licence body goes to out verbatim, because those bytes are the file the
// user saves.
func Run(ctx context.Context, session core.Session, args []string, out io.Writer, reply core.Responder) error {
	home := session.Home
	s := Store(home)
	v := s.Read()
	if v.Warning != "" {
		if err := reply.Warn(v.Warning); err != nil {
			return err
		}
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
		case "edit", "migrate":
			return edit(ctx, s, v, args, reply)
		}
	}
	fs := flag.NewFlagSet("license", flag.ContinueOnError)
	// The message belongs on the client's stderr, and it gets there as the returned
	// error. Writing it here would put it in the daemon's log instead, where the user
	// cannot see it.
	fs.SetOutput(io.Discard)
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
		// Whether a terminal is attached is a fact about the client, and core runs
		// detached without one.
		if !reply.CanAsk() {
			if *interactive {
				return errors.New("interactive terminal required")
			}
			fmt.Fprintln(out, "Usage: spu license --mit|--isc [--name NAME] [--year YEAR] [--output FILE]")
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
		spec, err := createForm(s, map[string]any{
			"type": kind, "author": *name, "output": *output,
			"year": yearText, "save_defaults": false,
		})
		if err != nil {
			return err
		}
		answered, err := reply.Ask(ctx, spec)
		if err != nil {
			return err
		}

		if got, ok := answered["type"].(string); ok {
			kind = got
		}
		if got, ok := answered["author"].(string); ok {
			*name = strings.TrimSpace(got)
		}
		if got, ok := answered["output"].(string); ok {
			*output = got
		}
		if got, ok := answered["year"].(string); ok {
			yearText = got
		}
		save, _ := answered["save_defaults"].(bool)

		if *name == "" {
			return errors.New("author required")
		}
		// A client can check that the answer is four characters wide; whether it is a
		// year is core's to decide, and the answer is not taken on trust.
		n, err := strconv.Atoi(yearText)
		if err != nil || n < 1000 || n > 9999 {
			return errors.New("use a four-digit year")
		}
		*year = n

		// The summary is the question rather than a line beside it: what is about to be
		// written is what the user is being asked to approve.
		confirmed, err := reply.Confirm(ctx, fmt.Sprintf("Generate %s license for %s (%d) at %s?",
			strings.ToUpper(kind), *name, *year, *output))
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}

		if *output == "" {
			return errors.New("output file is required")
		}
		if _, err := os.Stat(*output); err == nil {
			replace, err := reply.Confirm(ctx, *output+" already exists. Replace it?")
			if err != nil {
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
	body, err := render(kind, *name, *year)
	if err != nil {
		return err
	}
	if *output == "" {
		_, err = out.Write(body)
		return err
	}
	return configstore.AtomicWrite(*output, body, 0644)
}

func edit(ctx context.Context, s configstore.Store, v configstore.View, args []string, reply core.Responder) error {
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
	fs.SetOutput(io.Discard)
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
	if !reply.CanAsk() {
		// A migration has nothing to ask: the stored shape already says everything the
		// new one does.
		if args[0] == "migrate" {
			return savePreferences(s, data)
		}
		return errors.New("interactive terminal required")
	}

	// The edit form is the stored preferences and nothing else, so it is a plain
	// projection of the schema.
	spec, err := form.FromSchema(s.Schema, map[string]any{
		"type": kind, "author": name, "output": output,
	})
	if err != nil {
		return err
	}
	answered, err := reply.Ask(ctx, spec)
	if err != nil {
		return err
	}

	if got, ok := answered["type"].(string); ok {
		kind = got
	}
	if got, ok := answered["author"].(string); ok {
		name = got
	}
	if got, ok := answered["output"].(string); ok {
		output = got
	}
	if output == "" {
		return errors.New("output path required")
	}
	data = map[string]any{"type": kind, "author": name, "output": filepath.Clean(output)}
	return savePreferences(s, data)
}

// createForm describes what a licence needs.
//
// The three stored preferences are projected from the schema, exactly as the edit form
// is. Two fields are added that are not configuration: the year, resolved per
// invocation because a stored one would be wrong next January, and whether to keep the
// answers as the new defaults.
func createForm(s configstore.Store, values map[string]any) (form.Spec, error) {
	spec, err := form.FromSchema(s.Schema, values)
	if err != nil {
		return form.Spec{}, err
	}
	spec.Title = "Generate a license"

	// The schema describes stored preferences, so its titles read as defaults. Here the
	// same fields are being chosen, not configured.
	createTitles := map[string]string{
		"type": "License", "author": "Author", "output": "Output file",
	}
	fields := make([]form.Field, 0, len(spec.Fields)+2)
	for _, f := range spec.Fields {
		if title, ok := createTitles[f.Key]; ok {
			f.Title = title
		}
		fields = append(fields, f)
		if f.Key == "author" {
			fields = append(fields, form.Field{
				Key: "year", Type: form.String, Title: "Year",
				Value: values["year"], MinLength: 4, MaxLength: 4,
			})
		}
	}
	fields = append(fields, form.Field{
		Key: "save_defaults", Type: form.Boolean,
		Title: "Save these as the defaults", Value: values["save_defaults"],
	})
	spec.Fields = fields
	return spec, nil
}
