package thirdparty

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeFetch records what it was asked for and writes a provenance file, standing in for
// the configurable shell script so these tests describe the rules and not git.
type fakeFetch struct {
	calls []string
	urls  []string
}

func (f *fakeFetch) run(_ context.Context, url, dest, ref string) error {
	f.calls = append(f.calls, ref)
	f.urls = append(f.urls, url)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	return writeProvenance(dest, provenance{URL: url, Ref: ref, Version: ref})
}

func resolver(t *testing.T, cache string, fetch *fakeFetch) Resolver {
	t.Helper()
	return Resolver{CacheDir: cache, Fetch: fetch.run, Log: &strings.Builder{}}
}

func TestAFetchPopulatesTheCache(t *testing.T) {
	cache := t.TempDir()
	fetch := &fakeFetch{}

	got, err := resolver(t, cache, fetch).Resolve(context.Background(), Request{
		Package: "github.com/foo/bar",
		Version: "v2.3.1",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Fetched != true {
		t.Error("the first request did not fetch")
	}
	if got.Path != filepath.Join(cache, "github.com", "foo", "bar") {
		t.Errorf("Path = %q, want it under the cache", got.Path)
	}
	if len(fetch.calls) != 1 || fetch.calls[0] != "v2.3.1" {
		t.Errorf("the fetch script was called with %v, want the version", fetch.calls)
	}
}

func TestAnExistingCheckoutIsReusedWithoutFetching(t *testing.T) {
	// It is a cache, not a downloader. Reading must not touch the network.
	cache := t.TempDir()
	fetch := &fakeFetch{}
	r := resolver(t, cache, fetch)

	first, err := r.Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "v2.3.1",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	before := len(fetch.calls)

	second, err := r.Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "v2.9.0",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(fetch.calls) != before {
		t.Errorf("a cached major was fetched again: %v", fetch.calls)
	}
	if second.Path != first.Path {
		t.Errorf("Path = %q, want the checkout already there", second.Path)
	}
	if second.Version != "v2.3.1" {
		t.Errorf("Version = %q, want the version actually on disk, not the one asked for",
			second.Version)
	}
}

func TestADifferentMajorIsFetched(t *testing.T) {
	// The major version is what matters: a v3 is not a newer v2, it is a different
	// library that happens to share a name.
	cache := t.TempDir()
	fetch := &fakeFetch{}
	r := resolver(t, cache, fetch)

	if _, err := r.Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "v2.3.1"}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	got, err := r.Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "v3.0.0"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !got.Fetched {
		t.Error("a different major was served from the cache")
	}
	if got.Version != "v3.0.0" {
		t.Errorf("Version = %q, want the major that was asked for", got.Version)
	}
}

func TestAnExactVersionIsAQuestionForTheUser(t *testing.T) {
	// A tool cannot put a question to a person. It says what it needs and why, and the
	// agent asks — pulling a pinned version without being told to is how a cache becomes
	// a supply chain.
	cache := t.TempDir()
	fetch := &fakeFetch{}
	r := resolver(t, cache, fetch)

	got, err := r.Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "v2.3.1",
		Pin: true, Reason: "the API changed in v2.4",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Decision == nil {
		t.Fatal("a pinned version was fetched without asking")
	}
	if got.Decision.Version != "v2.3.1" || got.Decision.Reason != "the API changed in v2.4" {
		t.Errorf("Decision = %+v, want the version and the reason carried through", got.Decision)
	}
	if len(fetch.calls) != 0 {
		t.Errorf("a pinned version was fetched anyway: %v", fetch.calls)
	}
}

func TestAnUnsatisfiableMajorIsReportedNotGuessed(t *testing.T) {
	// Nothing in the cache matches, and the fetch script failed. Saying so beats
	// returning something whose version is not what was asked for.
	cache := t.TempDir()
	r := Resolver{
		CacheDir: cache,
		Fetch: func(context.Context, string, string, string) error {
			return os.ErrPermission
		},
		Log: &strings.Builder{},
	}
	_, err := r.Resolve(context.Background(), Request{Package: "github.com/foo/bar", Version: "v2.0.0"})
	if err == nil {
		t.Fatal("Resolve returned a result although nothing could be fetched")
	}
}

func TestEveryFetchIsLogged(t *testing.T) {
	// The developer should be able to see what their agents have been reading.
	cache := t.TempDir()
	var log strings.Builder
	r := Resolver{CacheDir: cache, Fetch: (&fakeFetch{}).run, Log: &log}

	if _, err := r.Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "v1.0.0"}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !strings.Contains(log.String(), "github.com/foo/bar") {
		t.Errorf("nothing was logged about the fetch: %q", log.String())
	}
}

func TestProvenanceRoundTrips(t *testing.T) {
	dir := t.TempDir()
	want := provenance{URL: "https://example.com/x.git", Ref: "v1.2.3", Version: "v1.2.3"}
	if err := writeProvenance(dir, want); err != nil {
		t.Fatalf("writeProvenance: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, provenanceFile))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var got provenance
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
