package form

import (
	"strings"
	"testing"
)

// envelope wraps a data schema the way every util's schema.json does, so these tests
// exercise the real shape rather than a simplification of it.
func envelope(data string) []byte {
	return []byte(`{
	  "$schema": "https://json-schema.org/draft/2020-12/schema",
	  "type": "object",
	  "required": ["util", "schema_version", "data"],
	  "properties": {
	    "util": {"const": "example"},
	    "schema_version": {"const": 1},
	    "data": ` + data + `
	  },
	  "additionalProperties": false
	}`)
}

func fieldByKey(t *testing.T, spec Spec, key string) Field {
	t.Helper()
	for _, f := range spec.Fields {
		if f.Key == key {
			return f
		}
	}
	t.Fatalf("no field %q in the spec; got %v", key, keysOf(spec))
	return Field{}
}

func keysOf(spec Spec) []string {
	out := make([]string, 0, len(spec.Fields))
	for _, f := range spec.Fields {
		out = append(out, f.Key)
	}
	return out
}

func TestProjectAStringField(t *testing.T) {
	spec, err := FromSchema(envelope(`{
	  "type": "object",
	  "required": ["name"],
	  "properties": {"name": {"type": "string", "minLength": 1, "maxLength": 40}}
	}`), map[string]any{"name": "Alice"})
	if err != nil {
		t.Fatalf("FromSchema: %v", err)
	}

	got := fieldByKey(t, spec, "name")
	if got.Type != String {
		t.Errorf("type = %q, want %q", got.Type, String)
	}
	if !got.Required {
		t.Error("name is in the schema's required list but the field is not required")
	}
	if got.Value != "Alice" {
		t.Errorf("value = %v, want Alice", got.Value)
	}
	if got.MaxLength != 40 {
		t.Errorf("max length = %d, want 40", got.MaxLength)
	}
}

func TestProjectAnEnumField(t *testing.T) {
	spec, err := FromSchema(envelope(`{
	  "type": "object",
	  "required": ["rc"],
	  "properties": {"rc": {"enum": ["none", "bash", "zsh", "both"]}}
	}`), map[string]any{"rc": "both"})
	if err != nil {
		t.Fatalf("FromSchema: %v", err)
	}

	got := fieldByKey(t, spec, "rc")
	if got.Type != Enum {
		t.Fatalf("type = %q, want %q", got.Type, Enum)
	}
	if len(got.Options) != 4 {
		t.Fatalf("got %d options, want 4", len(got.Options))
	}
	// Without a hint the label is the value. Inventing a prettier one would be a guess
	// about wording that the util owns.
	if got.Options[3].Value != "both" || got.Options[3].Label != "both" {
		t.Errorf("option = %+v, want both labelled both", got.Options[3])
	}
	if got.Value != "both" {
		t.Errorf("value = %v, want both", got.Value)
	}
}

func TestProjectABooleanField(t *testing.T) {
	spec, err := FromSchema(envelope(`{
	  "type": "object",
	  "properties": {"banner_in_tmux": {"type": "boolean"}}
	}`), map[string]any{"banner_in_tmux": true})
	if err != nil {
		t.Fatalf("FromSchema: %v", err)
	}

	got := fieldByKey(t, spec, "banner_in_tmux")
	if got.Type != Boolean {
		t.Errorf("type = %q, want %q", got.Type, Boolean)
	}
	if got.Value != true {
		t.Errorf("value = %v, want true", got.Value)
	}
}

func TestTitlesAndOrderComeFromTheUIHints(t *testing.T) {
	schema := []byte(`{
	  "$schema": "https://json-schema.org/draft/2020-12/schema",
	  "x-spu-ui": {
	    "title": "Welcome banner",
	    "order": ["device", "name"],
	    "fields": {
	      "name": {"title": "Your name", "help": "Shown in the banner"},
	      "device": {"title": "Machine name"},
	      "rc": {"title": "Shell hooks", "options": {"none": "None", "both": "Bash and Zsh"}}
	    }
	  },
	  "type": "object",
	  "properties": {
	    "util": {"const": "welcome"},
	    "schema_version": {"const": 1},
	    "data": {
	      "type": "object",
	      "properties": {
	        "name": {"type": "string"},
	        "device": {"type": "string"},
	        "rc": {"enum": ["none", "both"]},
	        "message": {"type": "string"}
	      }
	    }
	  }
	}`)

	spec, err := FromSchema(schema, map[string]any{"name": "Alice", "rc": "none"})
	if err != nil {
		t.Fatalf("FromSchema: %v", err)
	}

	if spec.Title != "Welcome banner" {
		t.Errorf("title = %q, want Welcome banner", spec.Title)
	}

	// Listed fields first in the stated order, then the ones the hints did not mention,
	// alphabetically. A form whose field order changes between runs is a bug that only
	// shows up on somebody else's machine.
	want := []string{"device", "name", "message", "rc"}
	got := keysOf(spec)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("field order = %v, want %v", got, want)
	}

	if f := fieldByKey(t, spec, "name"); f.Title != "Your name" || f.Help == "" {
		t.Errorf("name = %+v, want the hinted title and help", f)
	}
	if f := fieldByKey(t, spec, "rc"); f.Options[1].Label != "Bash and Zsh" {
		t.Errorf("rc option label = %q, want the hint", f.Options[1].Label)
	}
	if f := fieldByKey(t, spec, "message"); f.Title != "message" {
		// Falling back to the key rather than inventing a label: a plain key is honest,
		// a guessed title is a claim about wording the util never made.
		t.Errorf("unhinted title = %q, want the key itself", f.Title)
	}
}

func TestAnUnsupportedShapeIsRejectedByName(t *testing.T) {
	// A field the client cannot render must fail here, where the util author sees it,
	// rather than reaching a client that quietly draws nothing.
	_, err := FromSchema(envelope(`{
	  "type": "object",
	  "properties": {"tags": {"type": "array", "items": {"type": "string"}}}
	}`), nil)
	if err == nil {
		t.Fatal("FromSchema accepted a shape it cannot render, want an error")
	}
	if !strings.Contains(err.Error(), "tags") {
		t.Errorf("error %q does not name the field", err)
	}
}

func TestAFieldWithNoValueIsStillDescribed(t *testing.T) {
	// A fresh install has no saved value, and the form still has to render.
	spec, err := FromSchema(envelope(`{
	  "type": "object",
	  "properties": {"name": {"type": "string"}}
	}`), nil)
	if err != nil {
		t.Fatalf("FromSchema: %v", err)
	}
	if got := fieldByKey(t, spec, "name"); got.Value != nil {
		t.Errorf("value = %v, want nil", got.Value)
	}
}

func TestProjectAKeyValueField(t *testing.T) {
	// A map of strings is a shape a terminal can render as an object and markup can
	// render as a table, which is why it is a kind rather than being flattened into text
	// on the way in.
	spec, err := FromSchema(envelope(`{
	  "type": "object",
	  "properties": {"aliases": {
	    "type": "object",
	    "additionalProperties": {"type": "string"}
	  }}
	}`), map[string]any{"aliases": map[string]any{"st": "status"}})
	if err != nil {
		t.Fatalf("FromSchema: %v", err)
	}

	got := fieldByKey(t, spec, "aliases")
	if got.Type != KeyValue {
		t.Fatalf("type = %q, want %q", got.Type, KeyValue)
	}
	entries, ok := got.Value.(map[string]any)
	if !ok || entries["st"] != "status" {
		t.Errorf("value = %v, want the stored map", got.Value)
	}
}

func TestAnObjectWithAFixedShapeIsNotAKeyValueField(t *testing.T) {
	// A field the client cannot render has to fail here, where the util author sees it.
	// An object with declared properties is a struct rather than a map, and no client
	// can invent a control for an arbitrary one.
	_, err := FromSchema(envelope(`{
	  "type": "object",
	  "properties": {"nested": {
	    "type": "object",
	    "properties": {"a": {"type": "string"}}
	  }}
	}`), nil)
	if err == nil {
		t.Fatal("FromSchema accepted an object with a fixed shape, want an error")
	}
	if !strings.Contains(err.Error(), "nested") {
		t.Errorf("error %q does not name the field", err)
	}
}

func TestAKeyValueMapOfNonStringsIsRejected(t *testing.T) {
	_, err := FromSchema(envelope(`{
	  "type": "object",
	  "properties": {"counts": {
	    "type": "object",
	    "additionalProperties": {"type": "integer"}
	  }}
	}`), nil)
	if err == nil {
		t.Fatal("FromSchema accepted a map of something other than strings, want an error")
	}
}
