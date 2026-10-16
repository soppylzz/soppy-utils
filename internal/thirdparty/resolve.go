// Package thirdparty keeps local checkouts of packages an agent wants to read.
//
// It is a cache, not a downloader. The point is that an agent reads a directory on disk
// instead of making the same web request repeatedly, so the common case is a lookup that
// touches no network at all.
package thirdparty

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// provenanceFile records where a checkout came from, inside the checkout.
const provenanceFile = ".spu-source.json"

// DefaultCacheDir is where checkouts are kept unless told otherwise.
//
// A reference directory rather than a project directory: the point is to read somebody
// else's code, and it is the same code whichever project is asking.
var DefaultCacheDir = filepath.Join("~", "code", "reference")

// Request is what an agent wants to read.
type Request struct {
	// Package is a name like github.com/owner/repo, or a URL.
	Package string

	// Version is what is wanted. The major is what decides whether a cached checkout
	// will do; the rest is a preference.
	Version string

	// Pin asks for that exact version rather than that major. It is a question for a
	// person, never something this package does on its own.
	Pin bool

	// Reason is why the exact version is wanted, carried to whoever is asked.
	Reason string
}

// Decision is a question the caller has to put to a person.
//
// An MCP tool cannot ask: it has no terminal and no user. So it returns what it needs and
// why, and the agent — which does have a user — asks. Pulling a pinned version without
// being told to is how a cache quietly becomes a supply chain.
type Decision struct {
	Package string
	Version string
	Reason  string
}

// Result is where the package can be read, and which version is actually there.
type Result struct {
	Path    string
	Version string
	Fetched bool

	// Decision, when set, means nothing was returned to read: the caller has a question
	// to ask first.
	Decision *Decision
}

// Tags lists the version tags a remote publishes.
//
// It is asked only when the cache cannot answer, so a hit still touches no network. The
// seam exists because the work it does — deciding which tag a request means — is the part
// that was wrong, and it has to be testable without a network.
type Tags func(ctx context.Context, url string) ([]string, error)

// Fetch checks a package out and reports the version it checked out.
//
// Returning the version rather than having the caller assume it is the whole point: what
// gets recorded has to be what is on disk, and only the thing that did the checkout knows
// that. It is a seam rather than a call to git so that a mirror, a token, or a vendored
// copy can be used without changing this package.
type Fetch func(ctx context.Context, url, dest, ref string) (version string, err error)

// Resolver answers requests from a cache directory.
type Resolver struct {
	CacheDir string
	Tags     Tags
	Fetch    Fetch

	// Log records what was fetched, so a developer can see what their agents have been
	// reading.
	Log io.Writer
}

// Resolve returns a local checkout satisfying the request.
//
// An existing checkout whose major matches is returned as it is, with no network access:
// that is the whole purpose. A different major is fetched, because a v3 is not a newer v2
// but a different library that happens to share a name.
func (r Resolver) Resolve(ctx context.Context, req Request) (Result, error) {
	dir, err := r.pathFor(req.Package)
	if err != nil {
		return Result{}, err
	}
	url := CloneURL(req.Package)

	cached, err := readProvenance(dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Result{}, err
	}

	if req.Pin {
		// A pinned version is never fetched on this package's own authority. If it is
		// already there, saying so costs nothing and answers the question by itself.
		if cached.Version == req.Version {
			return Result{Path: dir, Version: cached.Version}, nil
		}
		return Result{Decision: &Decision{
			Package: req.Package, Version: req.Version, Reason: req.Reason,
		}}, nil
	}

	if satisfies(cached, req.Version) {
		return Result{Path: dir, Version: cached.Version}, nil
	}

	// What a caller gives is a version, not a ref: `8` is a major, and asking git for a
	// branch called 8 is what produced "Remote branch 8 not found". It is resolved here,
	// before anything is cloned, so what is checked out and what is recorded are the same.
	ref, err := r.resolveRef(ctx, url, req.Version)
	if err != nil {
		return Result{}, err
	}

	version, err := r.fetch(ctx, url, dir, ref)
	if err != nil {
		return Result{}, fmt.Errorf("fetch %s at %s: %w", req.Package, ref, err)
	}
	if version == "" {
		return Result{}, fmt.Errorf("fetch %s reported no version, so there would be "+
			"nothing to record and the cache could never answer again", req.Package)
	}

	// Recorded after the checkout and from what the checkout reported, so a fetch that
	// failed leaves nothing behind and a fetch that succeeded records the truth.
	if err := writeProvenance(dir, provenance{URL: url, Ref: ref, Version: version}); err != nil {
		return Result{}, fmt.Errorf("record what was checked out of %s: %w", req.Package, err)
	}
	return Result{Path: dir, Version: version, Fetched: true}, nil
}

// satisfies reports whether a checkout on disk answers the request.
//
// A request with no version constrains nothing, so anything there will do — which is what
// the comment on majorOf always said and what the old guard, by also requiring the cache
// to have a version, made impossible: an unversioned fetch recorded an empty version, so
// it could never satisfy itself and every later call cloned over its own directory.
func satisfies(cached provenance, want string) bool {
	if cached.Version == "" {
		return false
	}
	if majorOf(want) == "" {
		return true
	}
	return majorOf(cached.Version) == majorOf(want)
}

// resolveRef turns a requested version into a tag the remote actually publishes.
func (r Resolver) resolveRef(ctx context.Context, url, want string) (string, error) {
	if r.Tags == nil {
		return "", errors.New("no way to list the versions a remote publishes is configured")
	}
	tags, err := r.Tags(ctx, url)
	if err != nil {
		return "", fmt.Errorf("list the versions of %s: %w", url, err)
	}
	ref, err := PickVersion(want, tags)
	if err != nil {
		return "", fmt.Errorf("%s: %w", url, err)
	}
	return ref, nil
}

func (r Resolver) fetch(ctx context.Context, url, dir, ref string) (string, error) {
	fetch := r.Fetch
	if fetch == nil {
		return "", errors.New("no fetch flow is configured")
	}
	if r.Log != nil {
		fmt.Fprintf(r.Log, "fetch %s at %s -> %s\n", url, ref, dir)
	}
	return fetch(ctx, url, dir, ref)
}

// pathFor maps a package to a directory under the cache.
//
// The host is part of the path so that a package from one host cannot be served for a
// request naming another, which is the difference between reading a library and reading
// whatever somebody put at that name.
func (r Resolver) pathFor(pkg string) (string, error) {
	cache := r.CacheDir
	if cache == "" {
		cache = DefaultCacheDir
	}
	cache = expandHome(cache)
	if cache == "" {
		return "", errors.New("no cache directory is configured")
	}

	trimmed := strings.TrimSuffix(strings.TrimSuffix(pkg, "/"), ".git")
	trimmed = strings.TrimPrefix(trimmed, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	trimmed = strings.TrimPrefix(trimmed, "git@")
	trimmed = strings.Replace(strings.SplitN(trimmed, "#", 2)[0], ":", "/", 1)

	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("package %q is not a name or a URL", pkg)
	}
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			return "", fmt.Errorf("package %q has an empty or relative path segment", pkg)
		}
	}
	return filepath.Join(append([]string{cache}, parts...)...), nil
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	return path
}

// majorOf reads the major version, which is the part that decides compatibility.
//
// An unversioned request has no major, and then any checkout satisfies it: the caller
// asked for whatever is there.
func majorOf(version string) string {
	v := strings.TrimPrefix(version, "v")
	if v == "" {
		return ""
	}
	major, _, _ := strings.Cut(v, ".")
	return major
}

// provenance is what is recorded about a checkout.
type provenance struct {
	URL     string `json:"url"`
	Ref     string `json:"ref"`
	Version string `json:"version"`
}

func writeProvenance(dir string, p provenance) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, provenanceFile), append(body, '\n'), 0o644)
}

func readProvenance(dir string) (provenance, error) {
	var p provenance
	body, err := os.ReadFile(filepath.Join(dir, provenanceFile))
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return provenance{}, err
	}
	return p, nil
}
