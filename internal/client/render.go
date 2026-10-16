package client

import (
	"context"
	"io"

	"charm.land/huh/v2"

	"github.com/soppylzz/soppy-utils/internal/form"
)

// This file translates decisions into a drawing library and has none of its own: which
// control, what title, and which choices were settled in Controls, where they can be
// checked. What is here needs a terminal to run, so it is kept thin on purpose.

func drawForm(ctx context.Context, in io.Reader, out io.Writer, spec form.Spec) (map[string]any, error) {
	controls, err := Controls(spec)
	if err != nil {
		return nil, err
	}
	if len(controls) == 0 {
		return map[string]any{}, nil
	}

	fields := make([]huh.Field, 0, len(controls))
	read := make([]func() (string, any), 0, len(controls))

	for _, c := range controls {
		switch c.Kind {
		case Input:
			v := stringValue(c.Initial)
			field := huh.NewInput().Title(c.Title).Value(&v)
			if c.Help != "" {
				field = field.Description(c.Help)
			}
			fields = append(fields, field)
			read = append(read, func() (string, any) { return c.Key, v })

		case Confirm:
			v := boolValue(c.Initial)
			field := huh.NewConfirm().Title(c.Title).Value(&v)
			if c.Help != "" {
				field = field.Description(c.Help)
			}
			fields = append(fields, field)
			read = append(read, func() (string, any) { return c.Key, v })

		case Select:
			v := stringValue(c.Initial)
			options := make([]huh.Option[string], 0, len(c.Options))
			for _, o := range c.Options {
				options = append(options, huh.NewOption(o.Label, o.Value))
			}
			field := huh.NewSelect[string]().Title(c.Title).Options(options...).Value(&v)
			if c.Help != "" {
				field = field.Description(c.Help)
			}
			fields = append(fields, field)
			read = append(read, func() (string, any) { return c.Key, v })
		}
	}

	err = huh.NewForm(huh.NewGroup(fields...)).WithInput(in).WithOutput(out).RunWithContext(ctx)
	if err != nil {
		return nil, err
	}

	answers := make(map[string]any, len(read))
	for _, r := range read {
		key, value := r()
		answers[key] = value
	}
	return answers, nil
}

func drawConfirm(ctx context.Context, in io.Reader, out io.Writer, question string) (bool, error) {
	ok := false
	field := huh.NewConfirm().Title(question).Value(&ok)
	if err := huh.NewForm(huh.NewGroup(field)).WithInput(in).WithOutput(out).RunWithContext(ctx); err != nil {
		return false, err
	}
	return ok, nil
}

// The values here are the zero value of their kind when core sent none, which is what
// Controls already normalised for a boolean; a mismatch would mean core described a
// field with a value of the wrong type, which its own validation prevents.
func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func boolValue(v any) bool {
	b, _ := v.(bool)
	return b
}
