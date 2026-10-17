package configstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Store keeps a util's config contract separate from package releases.
type Store struct {
	Home, Util string
	Current    int
	Schema     []byte
	OldSchemas map[int][]byte
	Defaults   map[string]any
}

type envelope struct {
	Util          string         `json:"util"`
	SchemaVersion int            `json:"schema_version"`
	Data          map[string]any `json:"data"`
}

type View struct {
	Data             map[string]any
	StoredVersion    int
	Exists, Writable bool
	Warning          string
}

func Home() (string, error) {
	if native := os.Getenv("SPU_HOME_NATIVE"); native != "" {
		return native, nil
	}
	return os.UserHomeDir()
}

func (s Store) Dir() string         { return filepath.Join(s.Home, ".config", "soppy-utils", s.Util) }
func (s Store) ConfigPath() string  { return filepath.Join(s.Dir(), "config.json") }
func (s Store) VersionPath() string { return filepath.Join(s.Dir(), "VERSION") }

func copyMap(m map[string]any) map[string]any {
	n := make(map[string]any, len(m))
	for k, v := range m {
		n[k] = v
	}
	return n
}

func (s Store) Validate(data map[string]any) error {
	return validateEnvelope(s.Util, s.Schema, envelope{Util: s.Util, SchemaVersion: s.Current, Data: data})
}

func validateEnvelope(util string, schemaBytes []byte, env envelope) error {
	var raw any
	if err := json.Unmarshal(schemaBytes, &raw); err != nil {
		return err
	}
	// The schema is registered under a name of its own rather than a relative one. A
	// relative name is resolved against the working directory of whichever process is
	// validating, so a failure reported a path like file:///tmp/<somewhere>/schema.json:
	// a file that does not exist, in a directory the reader has never heard of, at the
	// top of an otherwise useful message.
	id := "urn:spu:" + util + ":schema"
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(id, raw); err != nil {
		return err
	}
	schema, err := compiler.Compile(id)
	if err != nil {
		return err
	}
	return schema.Validate(map[string]any{"util": env.Util, "schema_version": env.SchemaVersion, "data": env.Data})
}

// Read never persists a polyfill. An invalid or future config is read-only.
func (s Store) Read() View {
	v := View{Data: copyMap(s.Defaults), Writable: true}
	marker, markerErr := limitedRead(s.VersionPath(), 64)
	raw, jsonErr := limitedRead(s.ConfigPath(), 1024*1024)
	if errors.Is(markerErr, os.ErrNotExist) && errors.Is(jsonErr, os.ErrNotExist) {
		return v
	}
	v.Exists = true
	if markerErr != nil || jsonErr != nil {
		v.Writable = false
		v.Warning = "incomplete config; run 'spu " + s.Util + " migrate'"
		return v
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(marker)))
	var env envelope
	if err != nil || json.Unmarshal(raw, &env) != nil || env.Util != s.Util || n != env.SchemaVersion || n < 0 {
		v.Writable = false
		v.Warning = "VERSION/config.json conflict or malformed config; run 'spu " + s.Util + " migrate'"
		return v
	}
	v.StoredVersion = n
	if env.Data == nil {
		v.Writable = false
		v.Warning = "invalid config data; run 'spu " + s.Util + " migrate'"
		return v
	}
	if n > s.Current {
		v.Writable = false
		v.Warning = fmt.Sprintf("config contract %d is newer than supported %d; use a compatible spu", n, s.Current)
		return v
	}
	if n < s.Current {
		oldSchema, known := s.OldSchemas[n]
		if !known {
			v.Writable = false
			v.Warning = fmt.Sprintf("unsupported old config contract %d; use a compatible migration tool", n)
			return v
		}
		if err := validateEnvelope(s.Util, oldSchema, env); err != nil {
			v.Writable = false
			v.Warning = "invalid old config: " + err.Error()
			return v
		}
		v.Warning = fmt.Sprintf("config contract %d; required %d; run 'spu %s migrate'", n, s.Current, s.Util)
	} else if err := s.Validate(env.Data); err != nil {
		v.Writable = false
		v.Warning = "invalid current config: " + err.Error()
		return v
	}
	// Version zero is the known initial shape; absent fields take explicit defaults.
	for k, val := range env.Data {
		if _, known := s.Defaults[k]; known {
			v.Data[k] = val
		}
	}
	if err := s.Validate(v.Data); err != nil {
		v.Writable = false
		v.Data = copyMap(s.Defaults)
		v.Warning = "invalid config: " + err.Error()
		return v
	}
	return v
}

func (s Store) Save(data map[string]any) error {
	if err := s.Validate(data); err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir(), 0700); err != nil {
		return err
	}
	env := envelope{Util: s.Util, SchemaVersion: s.Current, Data: data}
	b, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	// Both files are only written during explicit install, edit, or migrate.
	previous, previousErr := os.ReadFile(s.ConfigPath())
	if previousErr != nil && !errors.Is(previousErr, os.ErrNotExist) {
		return previousErr
	}
	if err := AtomicWrite(s.ConfigPath(), b, 0600); err != nil {
		return err
	}
	if err := AtomicWrite(s.VersionPath(), []byte(strconv.Itoa(s.Current)+"\n"), 0600); err != nil {
		if errors.Is(previousErr, os.ErrNotExist) {
			_ = os.Remove(s.ConfigPath())
		} else {
			_ = AtomicWrite(s.ConfigPath(), previous, 0600)
		}
		return err
	}
	return nil
}

// LoadAnswers accepts the same versioned envelope as config.json.
func (s Store) LoadAnswers(path string) (map[string]any, error) {
	b, err := limitedRead(path, 1024*1024)
	if err != nil {
		return nil, err
	}
	if len(b) > 1024*1024 {
		return nil, errors.New("answers file is too large")
	}
	var env envelope
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, err
	}
	if env.Util != s.Util || env.SchemaVersion != s.Current {
		return nil, errors.New("answers util or schema_version does not match")
	}
	if err := s.Validate(env.Data); err != nil {
		return nil, err
	}
	return env.Data, nil
}

func limitedRead(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, errors.New("file exceeds size limit")
	}
	return b, nil
}

func AtomicWrite(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".spu-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
