// Package form describes an input for a client to render.
//
// Core does not draw anything. It says which fields it needs, what each one accepts,
// and what the current values are. A terminal client renders that with a form library
// and a window client renders it with markup, which is what makes the two equal rather
// than one of them a second-class path through a terminal.
//
// The description is derived from the schema each util already has, not written out
// separately: a second hand-maintained description of the same fields is one more thing
// that can disagree with the validation the schema performs.
package form

// Type is the kind of input a field accepts.
//
// Deliberately small. Every kind has to be renderable both as a terminal prompt and as
// markup, so a kind that is not is a design failure rather than a missing feature. It
// grows when a util needs one, not in anticipation.
type Type string

const (
	String  Type = "string"
	Boolean Type = "boolean"

	// Enum is a choice from a fixed list.
	Enum Type = "enum"

	// KeyValue is a small map of strings. A terminal renders it as an object a person
	// edits and markup renders it as a table, which is why it is a kind of its own
	// rather than being flattened into text on the way in.
	KeyValue Type = "keyvalue"

	// Multiple is a set of choices from a fixed list.
	//
	// Not Enum with several values ticked: the answer is a set, and a client that offers
	// one-of and many-of through the same control makes the two indistinguishable in the
	// result. Its Value is a list, and it carries Options like an Enum does.
	Multiple Type = "multiple"
)

// Spec is a form.
type Spec struct {
	Title  string  `json:"title,omitempty"`
	Fields []Field `json:"fields"`
}

// Field is one input.
//
// Value is the current value, and is nil for a field that has none — a fresh install
// still has to render.
type Field struct {
	Key       string   `json:"key"`
	Type      Type     `json:"type"`
	Title     string   `json:"title"`
	Help      string   `json:"help,omitempty"`
	Value     any      `json:"value,omitempty"`
	Options   []Option `json:"options,omitempty"`
	Required  bool     `json:"required,omitempty"`
	MinLength int      `json:"min_length,omitempty"`
	MaxLength int      `json:"max_length,omitempty"`
}

// Option is one choice of an enum field.
//
// Label is what a client shows and Value is what core stores, so that a util can name
// a choice without renaming the value that is already on disk.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
