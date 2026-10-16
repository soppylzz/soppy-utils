package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

		case KeyValue:
			text := keyValueText(c.Initial)
			field := huh.NewText().Title(c.Title).Value(&text).Validate(func(s string) error {
				if _, err := parseKeyValue(s); err != nil {
					return errors.New(`write this as a JSON object, for example {"st": "status"}`)
				}
				return nil
			})
			if c.Help != "" {
				field = field.Description(c.Help)
			}
			fields = append(fields, field)
			read = append(read, func() (string, any) {
				values, err := parseKeyValue(text)
				if err != nil {
					// Validation held while the form was open, so this is unreachable
					// through a terminal. Leaving the field out beats inventing a value.
					return c.Key, nil
				}
				return c.Key, values
			})

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

// keyValueText renders a map as the text a person edits.
//
// Sorted and indented, because somebody is going to look at it: Go marshals map keys in
// order, so the text does not shuffle between openings.
func keyValueText(v any) string {
	values, ok := v.(map[string]any)
	if !ok || len(values) == 0 {
		return "{}"
	}
	body, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(body)
}

// parseKeyValue reads back what keyValueText produced, and rejects anything else.
//
// The schema says a map of strings, so a number or a nested object is somebody editing
// JSON the command would refuse later. Saying so while the form is open is where they
// can still do something about it.
func parseKeyValue(text string) (map[string]any, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, err
	}
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		var s string
		if err := json.Unmarshal(value, &s); err != nil {
			return nil, fmt.Errorf("%s is not a string", key)
		}
		out[key] = s
	}
	return out, nil
}
