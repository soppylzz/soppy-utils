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

// Fetch runs the configurable fetch flow.
//
// It is a seam rather than a call to git so that a mirror, a token, or a vendored copy
// can be used without changing this package. The default is the script `spu` installs.
type Fetch func(ctx context.Context, url, dest, ref string) error

// Resolver answers requests from a cache directory.
type Resolver struct {
	CacheDir string
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

	satisfied := cached.Version != "" && majorOf(cached.Version) == majorOf(req.Version)
	if satisfied {
		return Result{Path: dir, Version: cached.Version}, nil
	}

	ref := req.Version
	if err := r.fetch(ctx, req.Package, dir, ref); err != nil {
		return Result{}, fmt.Errorf("fetch %s at %s: %w", req.Package, ref, err)
	}
	after, err := readProvenance(dir)
	if err != nil {
		return Result{}, fmt.Errorf("read provenance of %s: %w", dir, err)
	}
	return Result{Path: dir, Version: after.Version, Fetched: true}, nil
}

func (r Resolver) fetch(ctx context.Context, pkg, dir, ref string) error {
	fetch := r.Fetch
	if fetch == nil {
		return errors.New("no fetch flow is configured")
	}
	if r.Log != nil {
		fmt.Fprintf(r.Log, "fetch %s at %s -> %s\n", pkg, ref, dir)
	}
	return fetch(ctx, pkg, dir, ref)
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

// RecordProvenance records what a checkout is, for the fetch flow to call once it has
// succeeded. It is exported because the binary owns the fetching and this package owns
// what is known about the result.
func RecordProvenance(dir, url, ref string) error {
	return writeProvenance(dir, provenance{URL: url, Ref: ref, Version: ref})
}
