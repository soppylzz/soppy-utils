package thirdparty

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRemote stands in for a git remote: it has tags, it can be asked for them, and it
// checks something out and reports what it checked out.
//
// It works out that version itself rather than echoing what it was asked for. A double
// that echoes the request agrees with an implementation that echoes the request, which is
// how the version came to be recorded as the caller's wish instead of the checkout's
// reality — and the tests said everything was fine.
type fakeRemote struct {
	tags  []string
	head  string // what the default branch is called when no tag is resolved
	calls []string
	urls  []string
	fail  error
}

func (f *fakeRemote) tagsFor(context.Context, string) ([]string, error) {
	if f.fail != nil {
		return nil, f.fail
	}
	return f.tags, nil
}

func (f *fakeRemote) clone(_ context.Context, url, dest, ref string) (string, error) {
	if f.fail != nil {
		return "", f.fail
	}
	f.calls = append(f.calls, ref)
	f.urls = append(f.urls, url)
	// git refuses to clone into a directory that is not empty, and the double has to
	// refuse it too. A double that tidily overwrites whatever is there agreed with an
	// implementation that told every second major it could not be fetched, and the tests
	// said both were fine.
	if entries, err := os.ReadDir(dest); err == nil && len(entries) > 0 {
		return "", fmt.Errorf("destination path '%s' already exists and is not an empty directory", dest)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", err
	}
	// A real checkout leaves files behind, and the tests that care about reuse depend on
	// there being something there.
	if err := os.WriteFile(filepath.Join(dest, "README.md"), []byte("checked out\n"), 0o644); err != nil {
		return "", err
	}
	if ref == "" {
		return f.head, nil
	}
	return ref, nil
}

func remote(t *testing.T) *fakeRemote {
	t.Helper()
	return &fakeRemote{
		tags: []string{"v1.0.0", "v1.4.0", "v2.0.0", "v2.3.1", "v2.9.0"},
		head: "commit:9f3c1ab",
	}
}

func resolver(t *testing.T, cache string, f *fakeRemote) Resolver {
	t.Helper()
	return Resolver{CacheDir: cache, Tags: f.tagsFor, Fetch: f.clone, Log: &strings.Builder{}}
}

func TestAFetchPopulatesTheCache(t *testing.T) {
	cache := t.TempDir()
	f := remote(t)

	got, err := resolver(t, cache, f).Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "v2.3.1",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !got.Fetched {
		t.Error("the first request did not fetch")
	}
	if got.Path != filepath.Join(cache, "github.com", "foo", "bar")+"@v2" {
		t.Errorf("Path = %q, want it under the cache, named for the major", got.Path)
	}
	if got.Version != "v2.3.1" {
		t.Errorf("Version = %q, want what was checked out", got.Version)
	}
}

func TestABarePackageNameIsClonedFromAURL(t *testing.T) {
	// The tool's description invites `github.com/owner/repo`, and git reads that as a
	// local path. It has to reach git as a URL.
	f := remote(t)
	if _, err := resolver(t, t.TempDir(), f).Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "v2.3.1",
	}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(f.urls) != 1 || f.urls[0] != "https://github.com/foo/bar" {
		t.Errorf("git was asked for %v, want the URL", f.urls)
	}
}

func TestAMajorIsResolvedToATagBeforeAnythingIsCloned(t *testing.T) {
	// `8` is a major, not a branch. Asking git for `--branch 8` is what produced
	// "Remote branch 8 not found".
	f := remote(t)
	if _, err := resolver(t, t.TempDir(), f).Resolve(context.Background(), Request{
		Package: "https://github.com/foo/bar", Version: "2",
	}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(f.calls) != 1 || f.calls[0] != "v2.9.0" {
		t.Errorf("git was asked to check out %v, want the newest tag of that major", f.calls)
	}
}

func TestAnUnversionedRequestIsServedFromTheCacheTheSecondTime(t *testing.T) {
	// The whole point of the tool, and the case that was never tested: a request with no
	// version at all. The first call checks out the newest release; the second must not
	// touch the network, and must not try to clone over what is already there.
	cache := t.TempDir()
	f := remote(t)
	r := resolver(t, cache, f)

	first, err := r.Resolve(context.Background(), Request{Package: "https://github.com/foo/bar"})
	if err != nil {
		t.Fatalf("first Resolve: %v", err)
	}
	if !first.Fetched || first.Version != "v2.9.0" {
		t.Errorf("first = %+v, want the newest release, fetched", first)
	}
	if len(f.calls) != 1 {
		t.Fatalf("the first request made %d fetches, want 1", len(f.calls))
	}

	second, err := r.Resolve(context.Background(), Request{Package: "https://github.com/foo/bar"})
	if err != nil {
		t.Fatalf("second Resolve: %v", err)
	}
	if second.Fetched {
		t.Error("the second request fetched again although the cache could answer")
	}
	if len(f.calls) != 1 {
		t.Errorf("the cache was not reused: %v", f.calls)
	}
	if second.Version != "v2.9.0" {
		t.Errorf("second Version = %q, want what is on disk", second.Version)
	}
}

func TestTheRecordedVersionIsWhatWasCheckedOutNotWhatWasAskedFor(t *testing.T) {
	// A repository with no releases: the caller asked for nothing, and what they got is
	// the default branch. Recording the request would record an empty version, which
	// reads as "this checkout is nothing" and stops the cache ever matching again.
	cache := t.TempDir()
	f := remote(t)
	f.tags = nil

	got, err := resolver(t, cache, f).Resolve(context.Background(), Request{
		Package: "https://github.com/foo/bar",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Version != "commit:9f3c1ab" {
		t.Errorf("Version = %q, want what the checkout reported", got.Version)
	}
	recorded, err := readProvenance(got.Path)
	if err != nil {
		t.Fatalf("readProvenance: %v", err)
	}
	if recorded.Version != "commit:9f3c1ab" {
		t.Errorf("the ledger records %q, want the checkout's own version", recorded.Version)
	}
}

func TestAnExistingCheckoutIsReusedWithoutFetching(t *testing.T) {
	cache := t.TempDir()
	f := remote(t)
	r := resolver(t, cache, f)

	if _, err := r.Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "v2.3.1"}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	before := len(f.calls)

	second, err := r.Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "v2.9.0"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(f.calls) != before {
		t.Errorf("a cached major was fetched again: %v", f.calls)
	}
	if second.Version != "v2.3.1" {
		t.Errorf("Version = %q, want the version actually on disk", second.Version)
	}
}

func TestDifferentMajorsAreKeptApart(t *testing.T) {
	// A v1 is not a newer v2 but a different library that happens to share a name, so both
	// have to be able to sit in the cache at once. One directory per package meant the
	// second major could never be fetched at all.
	f := remote(t)
	r := resolver(t, t.TempDir(), f)

	two, err := r.Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "2"})
	if err != nil {
		t.Fatalf("Resolve v2: %v", err)
	}
	if two.Version != "v2.9.0" {
		t.Fatalf("Version = %q, want the newest of that major", two.Version)
	}

	one, err := r.Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "1"})
	if err != nil {
		t.Fatalf("Resolve v1: %v", err)
	}
	if !one.Fetched {
		t.Error("a different major was served from the cache")
	}
	if one.Version != "v1.4.0" {
		t.Errorf("Version = %q, want the newest of the major that was asked for", one.Version)
	}
	if one.Path == two.Path {
		t.Error("both majors were put in one directory")
	}

	// And each is still answered from its own directory afterwards.
	again, err := r.Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "2"})
	if err != nil {
		t.Fatalf("Resolve v2 again: %v", err)
	}
	if again.Fetched || again.Version != "v2.9.0" {
		t.Errorf("v2 was not answered from the cache: %+v", again)
	}
	before := len(f.calls)
	if _, err := r.Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "1"}); err != nil {
		t.Fatalf("Resolve v1 again: %v", err)
	}
	if len(f.calls) != before {
		t.Errorf("v1 was fetched again: %v", f.calls)
	}
}

func TestAnUnversionedRequestTakesWhatIsAlreadyThere(t *testing.T) {
	// Nothing asked for constrains nothing, so whatever is checked out answers it. There
	// is no directory for "no version": the majors are the directories.
	f := remote(t)
	r := resolver(t, t.TempDir(), f)

	if _, err := r.Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "2"}); err != nil {
		t.Fatalf("Resolve v2: %v", err)
	}
	before := len(f.calls)
	got, err := r.Resolve(context.Background(), Request{Package: "github.com/foo/bar"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(f.calls) != before {
		t.Errorf("an unversioned request fetched although a major was cached: %v", f.calls)
	}
	if got.Version != "v2.9.0" {
		t.Errorf("Version = %q, want what is on disk", got.Version)
	}
}

func TestAnExactVersionIsAQuestionForTheUser(t *testing.T) {
	cache := t.TempDir()
	f := remote(t)

	got, err := resolver(t, cache, f).Resolve(context.Background(), Request{
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
	if len(f.calls) != 0 {
		t.Errorf("a pinned version was fetched anyway: %v", f.calls)
	}
}

func TestAPinnedVersionAlreadyThereIsAnsweredFromTheCache(t *testing.T) {
	cache := t.TempDir()
	f := remote(t)
	r := resolver(t, cache, f)

	if _, err := r.Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "v2.3.1"}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	got, err := r.Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "v2.3.1", Pin: true, Reason: "why not"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Decision != nil {
		t.Error("a pinned version that is already checked out was turned into a question")
	}
}

func TestAFailedFetchIsReportedNotGuessed(t *testing.T) {
	cache := t.TempDir()
	f := remote(t)
	f.fail = os.ErrPermission

	_, err := resolver(t, cache, f).Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "v2.0.0"})
	if err == nil {
		t.Fatal("Resolve returned a result although nothing could be fetched")
	}
}

func TestAnUnpublishedVersionIsReported(t *testing.T) {
	_, err := resolver(t, t.TempDir(), remote(t)).Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "9"})
	if err == nil {
		t.Fatal("Resolve accepted a major nobody published")
	}
	if !strings.Contains(err.Error(), "9") {
		t.Errorf("error %q does not say what was asked for", err)
	}
}

func TestEveryFetchIsLogged(t *testing.T) {
	cache := t.TempDir()
	var log strings.Builder
	f := remote(t)
	r := Resolver{CacheDir: cache, Tags: f.tagsFor, Fetch: f.clone, Log: &log}

	if _, err := r.Resolve(context.Background(), Request{
		Package: "github.com/foo/bar", Version: "1"}); err != nil {
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
