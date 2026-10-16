package thirdparty

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// CloneURL turns a package name or URL into something git can clone.
//
// The tool's own description invites a bare `github.com/owner/repo`, and git reads a value
// with no scheme as a local path — which is how a well-formed package name came to be
// reported as a repository that does not exist. Everything that names a package passes
// through here, so the cache key and the clone cannot disagree about what was asked for.
func CloneURL(pkg string) string {
	name := strings.TrimSpace(pkg)
	name = strings.TrimSuffix(name, "/")

	// A local path is how a package is referenced without a network, and is left exactly
	// as it is.
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../") {
		return name
	}
	for _, scheme := range []string{"https://", "http://", "ssh://", "file://", "git://"} {
		if strings.HasPrefix(name, scheme) {
			return name
		}
	}
	// git's own shorthand: git@github.com:owner/repo
	if rest, ok := strings.CutPrefix(name, "git@"); ok {
		if host, path, found := strings.Cut(rest, ":"); found {
			return "https://" + host + "/" + path
		}
	}
	return "https://" + name
}

// CheckVersion rejects a version that is neither a major nor a whole version.
//
// `8.1` reads like a major and a minor, and the major alone is what matching uses — so
// treating it as a major would serve v8.9.0 to a request that named 8.1, which is a
// different answer to a different question. Refusing is the only honest thing to do with
// a request whose meaning is not one of the two things this accepts.
func CheckVersion(want string) error {
	v := strings.TrimPrefix(want, "v")
	if v == "" {
		return nil // nothing asked for: the newest release
	}
	parts := strings.Split(v, ".")
	if len(parts) != 1 && len(parts) != 3 {
		return fmt.Errorf("%q is not a version spu understands: give a major like %q for "+
			"the newest of that major, all three parts like %q for exactly that one, or "+
			"nothing at all for the newest release", want, parts[0], want+".0")
	}
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			return fmt.Errorf("%q is not a version spu understands: %q is not a number",
				want, p)
		}
	}
	return nil
}

// release is a tag that names a published version.
type release struct {
	tag                 string
	major, minor, patch int
}

// PickVersion chooses the tag a request refers to, from the tags a remote has.
//
// The version a caller gives is not a git ref: `8` is a major, and `--branch 8` asks git
// for a branch that does not exist. Resolution happens before anything is cloned, so what
// is checked out and what is recorded are the same thing.
//
// A request for nothing takes the newest release, because that is what "a checkout of this
// package" means for reading somebody else's source. A repository with no releases at all
// gives an empty ref, and the default branch is what the caller gets.
func PickVersion(want string, tags []string) (string, error) {
	releases := publishedReleases(tags)
	if len(releases) == 0 {
		if want == "" {
			return "", nil
		}
		return "", fmt.Errorf("no published version to satisfy %q: the repository has no "+
			"version tags", want)
	}

	if want == "" {
		return newest(releases).tag, nil
	}

	if major := majorOf(want); want == major {
		// Only a major was given, which is the common case: the caller wants whatever is
		// current within it.
		var matching []release
		for _, r := range releases {
			if strconv.Itoa(r.major) == major {
				matching = append(matching, r)
			}
		}
		if len(matching) == 0 {
			return "", fmt.Errorf("no version of major %s is published; the majors that "+
				"are: %s", want, majors(releases))
		}
		return newest(matching).tag, nil
	}

	// A full version, with or without the leading v, names one tag.
	for _, r := range releases {
		if r.tag == want || strings.TrimPrefix(r.tag, "v") == strings.TrimPrefix(want, "v") {
			return r.tag, nil
		}
	}
	// A version with two parts is neither of the two things this accepts, and saying so is
	// more use than saying it was not found.
	if parts := strings.Split(strings.TrimPrefix(want, "v"), "."); len(parts) == 2 {
		return "", fmt.Errorf("%q is not a whole version or a major: give a major like "+
			"%q, or all three parts like %q", want, parts[0], want+".0")
	}
	return "", fmt.Errorf("version %q is not published; the majors that are: %s",
		want, majors(releases))
}

// publishedReleases reads the tags that name a released version, newest first.
//
// A pre-release is not a release: `v8.2.0-rc1` must not be what somebody gets when they
// ask for the current 8.
func publishedReleases(tags []string) []release {
	out := make([]release, 0, len(tags))
	for _, tag := range tags {
		if strings.ContainsAny(tag, "-+") {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(tag, "v"), ".")
		if len(parts) != 3 {
			continue
		}
		numbers := make([]int, 3)
		ok := true
		for i, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil {
				ok = false
				break
			}
			numbers[i] = n
		}
		if !ok {
			continue
		}
		out = append(out, release{tag: tag, major: numbers[0], minor: numbers[1], patch: numbers[2]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].less(out[j]) })
	return out
}

func (r release) less(other release) bool {
	if r.major != other.major {
		return r.major < other.major
	}
	if r.minor != other.minor {
		return r.minor < other.minor
	}
	return r.patch < other.patch
}

func newest(rs []release) release { return rs[len(rs)-1] }

// majors lists the majors that are published, so an answer that cannot be given can say
// what could.
func majors(rs []release) string {
	seen := map[int]bool{}
	var numbers []int
	for _, r := range rs {
		if !seen[r.major] {
			seen[r.major] = true
			numbers = append(numbers, r.major)
		}
	}
	// Sorted as numbers, not as text: a repository that reaches 10 would otherwise list
	// its majors as 1, 10, 11, 2, 3 — which is the message somebody reads to find out what
	// they should have asked for.
	sort.Ints(numbers)
	out := make([]string, 0, len(numbers))
	for _, n := range numbers {
		out = append(out, strconv.Itoa(n))
	}
	return strings.Join(out, ", ")
}
