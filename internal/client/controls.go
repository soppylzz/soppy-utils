// Package client is the terminal client: it holds the terminal, renders what core
// describes, and performs what core cannot.
//
// It knows nothing about what any util does. It draws forms core described and runs
// commands core asked for, which is why a second client can be written without any of
// this being duplicated into core.
package client

import (
	"fmt"

	"github.com/soppylzz/soppy-utils/internal/form"
)

// kind is the control a field becomes in a terminal.
type kind string

const (
	inputKind       kind = "input"
	confirmKind     kind = "confirm"
	selectKind      kind = "select"
	keyValueKind    kind = "keyvalue"
	multiSelectKind kind = "multiselect"
)

// control is a field as this client will draw it.
//
// The mapping from what core described to what is drawn lives here, apart from the
// library that draws it, because this is the part with decisions in it: which control,
// what title, which choices. The library underneath is a detail.
type control struct {
	Key     string
	kind    kind
	Title   string
	Help    string
	Initial any
	Options []form.Option
}

// controls maps a described form to the controls this client will draw.
func controls(spec form.Spec) ([]control, error) {
	out := make([]control, 0, len(spec.Fields))
	for _, f := range spec.Fields {
		c := control{Key: f.Key, Title: f.Title, Help: f.Help, Initial: f.Value}
		switch f.Type {
		case form.String:
			c.kind = inputKind

		case form.Boolean:
			c.kind = confirmKind
			if c.Initial == nil {
				// A question with an unset answer starts at no. Leaving it nil would
				// hand the drawing library a value it cannot bind.
				c.Initial = false
			}

		case form.Enum:
			c.kind = selectKind
			c.Options = f.Options

		case form.KeyValue:
			c.kind = keyValueKind

		case form.Multiple:
			c.kind = multiSelectKind
			// A set that arrives from JSON is a list of anything; the control below binds
			// a list of strings, and a nil one means nothing was chosen yet.
			c.Initial = stringSlice(f.Value)
			c.Options = f.Options

		default:
			// Nothing is dropped silently. A window client and a terminal client draw
			// different controls, but neither may submit a form that was missing a
			// question, because the user would never know they were not asked.
			return nil, fmt.Errorf("field %q is of type %q, which this client cannot draw",
				f.Key, f.Type)
		}
		out = append(out, c)
	}
	return out, nil
}
