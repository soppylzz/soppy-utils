package configstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionPolyfillAndConflict(t *testing.T) {
	s := Store{Home: t.TempDir(), Util: "sample", Current: 1, Schema: []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","required":["util","schema_version","data"],"properties":{"util":{"const":"sample"},"schema_version":{"const":1},"data":{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}}}`), OldSchemas: map[int][]byte{0: []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","required":["util","schema_version","data"],"properties":{"util":{"const":"sample"},"schema_version":{"const":0},"data":{"type":"object","properties":{"name":{"type":"string"}}}}}`)}, Defaults: map[string]any{"name": "default"}}
	if err := os.MkdirAll(s.Dir(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.VersionPath(), []byte("0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"util":"sample","schema_version":0,"data":{}}`)
	if err := os.WriteFile(s.ConfigPath(), raw, 0600); err != nil {
		t.Fatal(err)
	}
	v := s.Read()
	if !v.Writable || v.Data["name"] != "default" || !strings.Contains(v.Warning, "migrate") {
		t.Fatalf("unexpected polyfill: %+v", v)
	}
	after, _ := os.ReadFile(s.ConfigPath())
	if string(after) != string(raw) {
		t.Fatal("read changed config")
	}
	if err := os.WriteFile(s.VersionPath(), []byte("2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	v = s.Read()
	if v.Writable || !strings.Contains(v.Warning, "conflict") {
		t.Fatalf("expected conflict: %+v", v)
	}
	if _, err := os.Stat(filepath.Join(s.Dir(), "state.json")); !os.IsNotExist(err) {
		t.Fatal("read created state")
	}
}
