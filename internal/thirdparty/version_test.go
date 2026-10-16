package thirdparty

import (
	"strings"
	"testing"
)

func TestCloneURL(t *testing.T) {
	// The tool's own description invites a bare package name. Handing that to git makes it
	// a local path, which is how `github.com/owner/repo` came to be reported as a
	// repository that does not exist.
	cases := map[string]string{
		"github.com/owner/repo":             "https://github.com/owner/repo",
		"github.com/owner/repo/":            "https://github.com/owner/repo",
		"https://github.com/owner/repo":     "https://github.com/owner/repo",
		"https://github.com/owner/repo.git": "https://github.com/owner/repo.git",
		"http://example.com/owner/repo":     "http://example.com/owner/repo",
		"git@github.com:owner/repo.git":     "https://github.com/owner/repo.git",
		"ssh://git@example.com/owner/repo":  "ssh://git@example.com/owner/repo",
		"file:///tmp/some/repo":             "file:///tmp/some/repo",
		"/tmp/some/repo":                    "/tmp/some/repo",
	}
	for pkg, want := range cases {
		if got := CloneURL(pkg); got != want {
			t.Errorf("CloneURL(%q) = %q, want %q", pkg, got, want)
		}
	}
}

func TestPickVersion(t *testing.T) {
	tags := []string{"v7.1.0", "v7.2.0", "v8.0.0", "v8.1.0", "v8.1.0-rc1", "v10.0.0", "not-a-version"}

	cases := []struct {
		name string
		want string
		tags []string
		got  string
	}{
		{"nothing asked for takes the newest release", "", tags, "v10.0.0"},
		{"a major takes the newest of that major", "8", tags, "v8.1.0"},
		{"a major is matched numerically, not as text", "7", tags, "v7.2.0"},
		{"a bare semver takes its tag", "8.1.0", tags, "v8.1.0"},
		{"a tagged semver takes itself", "v8.1.0", tags, "v8.1.0"},
		{"a release candidate is not the newest release", "8", []string{"v8.0.0", "v8.2.0-rc1"}, "v8.0.0"},
		{"no tags at all means the default branch", "", []string{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PickVersion(tc.want, tc.tags)
			if err != nil {
				t.Fatalf("PickVersion(%q): %v", tc.want, err)
			}
			if got != tc.got {
				t.Errorf("PickVersion(%q) = %q, want %q", tc.want, got, tc.got)
			}
		})
	}
}

func TestPickVersionSaysWhatIsAvailable(t *testing.T) {
	// An agent that asked for a major nobody published needs to be told what is there, or
	// it will ask again with another guess.
	_, err := PickVersion("9", []string{"v7.2.0", "v8.1.0", "v8.0.0"})
	if err == nil {
		t.Fatal("PickVersion accepted a major nobody published")
	}
	said := err.Error()
	for _, want := range []string{"9", "8", "7"} {
		if !strings.Contains(said, want) {
			t.Errorf("error %q does not mention %q", said, want)
		}
	}
}

func TestPickVersionOnARepositoryWithNoTags(t *testing.T) {
	// Somebody asked for a major and there are no releases at all. That is not the same
	// as asking for nothing.
	if _, err := PickVersion("8", []string{}); err == nil {
		t.Fatal("PickVersion invented a version for a repository with no tags")
	}
}

func TestMajorOf(t *testing.T) {
	cases := map[string]string{
		"v8.1.0": "8", "8.1.0": "8", "8": "8", "v10.2.3": "10", "": "",
	}
	for in, want := range cases {
		if got := majorOf(in); got != want {
			t.Errorf("majorOf(%q) = %q, want %q", in, got, want)
		}
	}
}
