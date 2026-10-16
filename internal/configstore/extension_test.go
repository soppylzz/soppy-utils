package configstore

import "testing"

// The form hints live in the same file as the validation rules, under an extension
// keyword, so that a util has one schema and one version to migrate. That only works
// if the pinned validator ignores a keyword it does not recognise — otherwise every
// schema carrying UI hints would fail to validate, and the hints would have to move to
// a second file whose version could drift from the first.
//
// Draft 2020-12 permits unknown keywords, but the pinned library is what decides, so
// this is checked rather than assumed.
func TestASchemaWithAnExtensionKeywordStillValidates(t *testing.T) {
	schema := []byte(`{
	  "$schema": "https://json-schema.org/draft/2020-12/schema",
	  "x-spu-ui": {
	    "title": "Example",
	    "order": ["name"],
	    "fields": {"name": {"title": "Name"}}
	  },
	  "type": "object",
	  "required": ["util", "schema_version", "data"],
	  "properties": {
	    "util": {"const": "example"},
	    "schema_version": {"const": 1},
	    "data": {
	      "type": "object",
	      "required": ["name"],
	      "properties": {"name": {"type": "string", "minLength": 1}},
	      "additionalProperties": false
	    }
	  },
	  "additionalProperties": false
	}`)

	s := Store{Home: "/tmp/spu-test", Util: "example", Current: 1, Schema: schema}
	if err := s.Validate(map[string]any{"name": "Alice"}); err != nil {
		t.Fatalf("a schema carrying an extension keyword failed to validate: %v", err)
	}

	// And the keyword must not have weakened the rules it sits beside.
	if err := s.Validate(map[string]any{"name": ""}); err == nil {
		t.Error("the extension keyword made the schema stop enforcing minLength")
	}
}
