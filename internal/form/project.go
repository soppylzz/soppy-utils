package form

import (
	"encoding/json"
	"fmt"
	"sort"
)

// FromSchema projects a form from a util's schema and its current values.
//
// The schema is the envelope every util already ships — util, schema_version, data —
// and the fields come from data's properties, so a form cannot describe a field the
// validation does not know about, or miss one it does.
func FromSchema(schema []byte, values map[string]any) (Spec, error) {
	var doc schemaDoc
	if err := json.Unmarshal(schema, &doc); err != nil {
		return Spec{}, fmt.Errorf("schema: %w", err)
	}
	if doc.Properties.Data == nil {
		return Spec{}, fmt.Errorf("schema has no data object to build a form from")
	}
	return specFrom(doc.Properties.Data.Properties, doc.Properties.Data.Required, doc.UI, values)
}

// FromObjectSchema projects a form from the object itself rather than from an envelope.
//
// A util's schema wraps its fields in a data object; an item's questions are the object.
// Both are the same question to a client, and wrapping the second in a synthetic envelope
// would put a shape in it that nothing wrote.
func FromObjectSchema(schema []byte, values map[string]any) (Spec, error) {
	var doc struct {
		UI *uiHints `json:"x-spu-ui"`
		dataSchema
	}
	if err := json.Unmarshal(schema, &doc); err != nil {
		return Spec{}, fmt.Errorf("schema: %w", err)
	}
	if len(doc.Properties) == 0 {
		return Spec{}, nil
	}
	return specFrom(doc.Properties, doc.Required, doc.UI, values)
}

func specFrom(properties map[string]json.RawMessage, requiredList []string, ui *uiHints,
	values map[string]any) (Spec, error) {

	required := make(map[string]bool, len(requiredList))
	for _, key := range requiredList {
		required[key] = true
	}

	spec := Spec{}
	if ui != nil {
		spec.Title = ui.Title
	}

	for _, key := range orderedKeys(properties, ui) {
		var raw fieldSchema
		if err := json.Unmarshal(properties[key], &raw); err != nil {
			return Spec{}, fmt.Errorf("field %q: %w", key, err)
		}
		kind, err := raw.kind()
		if err != nil {
			return Spec{}, fmt.Errorf("field %q: %w", key, err)
		}

		hints := fieldHints{}
		if ui != nil {
			hints = ui.Fields[key]
		}

		field := Field{
			Key:       key,
			Type:      kind,
			Title:     titleFor(key, hints),
			Help:      hints.Help,
			Value:     values[key],
			Required:  required[key],
			MinLength: raw.MinLength,
			MaxLength: raw.MaxLength,
		}
		switch kind {
		case Enum:
			field.Options = optionsFor(raw.Enum, hints)
		case Multiple:
			// The choices live on the items, because that is where a list keeps them.
			var items fieldSchema
			if err := json.Unmarshal(raw.Items, &items); err != nil {
				return Spec{}, fmt.Errorf("field %q: items: %w", key, err)
			}
			field.Options = optionsFor(items.Enum, hints)
		}
		spec.Fields = append(spec.Fields, field)
	}

	return spec, nil
}

// orderedKeys returns the fields in the order the form should show them.
//
// The order comes from the hints, because a form's field order is a decision somebody
// made rather than an accident of how the schema was written, and JSON object order is
// not preserved by any parser worth using. Fields the hints did not mention follow
// alphabetically, so the order is at least the same on every run.
func orderedKeys(properties map[string]json.RawMessage, ui *uiHints) []string {
	seen := make(map[string]bool, len(properties))
	out := make([]string, 0, len(properties))

	if ui != nil {
		for _, key := range ui.Order {
			if _, ok := properties[key]; ok && !seen[key] {
				seen[key] = true
				out = append(out, key)
			}
		}
	}

	rest := make([]string, 0, len(properties))
	for key := range properties {
		if !seen[key] {
			rest = append(rest, key)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// titleFor falls back to the key rather than prettifying it into words. A plain key is
// honest; an invented label is a claim about wording the util never made.
func titleFor(key string, hints fieldHints) string {
	if hints.Title != "" {
		return hints.Title
	}
	return key
}

func optionsFor(values []string, hints fieldHints) []Option {
	out := make([]Option, 0, len(values))
	for _, v := range values {
		label := hints.Options[v]
		if label == "" {
			label = v
		}
		out = append(out, Option{Value: v, Label: label})
	}
	return out
}

type schemaDoc struct {
	UI         *uiHints `json:"x-spu-ui"`
	Properties struct {
		Data *dataSchema `json:"data"`
	} `json:"properties"`
}

type dataSchema struct {
	Required   []string                   `json:"required"`
	Properties map[string]json.RawMessage `json:"properties"`
}

type uiHints struct {
	Title  string                `json:"title"`
	Order  []string              `json:"order"`
	Fields map[string]fieldHints `json:"fields"`
}

type fieldHints struct {
	Title   string            `json:"title"`
	Help    string            `json:"help"`
	Options map[string]string `json:"options"`
}

type fieldSchema struct {
	Type      string   `json:"type"`
	Enum      []string `json:"enum"`
	MinLength int      `json:"minLength"`
	MaxLength int      `json:"maxLength"`

	// Properties and AdditionalProperties tell an object apart from a map. An object
	// with declared properties is a struct, and no client can invent a control for an
	// arbitrary one; an object whose values are all strings is a map it can.
	Properties           json.RawMessage `json:"properties"`
	AdditionalProperties json.RawMessage `json:"additionalProperties"`

	// Items is what an array holds. Only a list of choices is renderable; anything else
	// is free text with bullets, which is not a control.
	Items json.RawMessage `json:"items"`
}

func (f fieldSchema) kind() (Type, error) {
	switch {
	case len(f.Enum) > 0:
		return Enum, nil
	case f.Type == "string":
		return String, nil
	case f.Type == "boolean":
		return Boolean, nil

	case f.Type == "array":
		var items fieldSchema
		if err := json.Unmarshal(f.Items, &items); err != nil {
			return "", fmt.Errorf("is an array whose items cannot be read: %w", err)
		}
		if len(items.Enum) == 0 {
			return "", fmt.Errorf("is an array that is not a choice from a fixed list, " +
				"so a client cannot render it as a multi-select")
		}
		return Multiple, nil

	case f.Type == "object":
		if len(f.Properties) > 0 {
			return "", fmt.Errorf("is an object with a fixed shape, which a client cannot render")
		}
		var values struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(f.AdditionalProperties, &values); err != nil || values.Type != "string" {
			return "", fmt.Errorf("is an object whose values are not all strings, which a client cannot render")
		}
		return KeyValue, nil

	case f.Type == "":
		return "", fmt.Errorf("has no type and no enum, so a client cannot render it")
	default:
		return "", fmt.Errorf("is of type %q, which a client cannot render", f.Type)
	}
}
