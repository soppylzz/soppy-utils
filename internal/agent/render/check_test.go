package render

import (
	"strings"
	"testing"
)

func skill(frontmatter string) []byte {
	return []byte("---\n" + frontmatter + "---\n\n# Body\n")
}

func TestAGoodSkillFrontmatterPasses(t *testing.T) {
	err := Check("skills/x/SKILL.md", skill(
		"name: x\ndescription: Use when doing x. Explains how.\n"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
}

func TestOtherFilesAreNotChecked(t *testing.T) {
	// A convention document is prose and may say anything. Only the harness's own format
	// is checked, because only that one fails in a way nobody sees.
	if err := Check("prompts/one.md", []byte("anything at all\n")); err != nil {
		t.Errorf("Check validated a document it has no rules for: %v", err)
	}
}

func TestABrokenSkillFrontmatterIsRejected(t *testing.T) {
	// Claude Code loads a skill whose frontmatter does not parse but drops every field,
	// falls back to the directory name and the first line of the body, and warns at no
	// normal verbosity. Since we are the ones generating it, this has to fail here.
	cases := []struct {
		name string
		body []byte
		says string
	}{
		{"no frontmatter", []byte("# Just a body\n"), "frontmatter"},
		{"unclosed", []byte("---\nname: x\n\n# Body\n"), "frontmatter"},
		{"no name", skill("description: Use when x.\n"), "name"},
		{"no description", skill("name: x\n"), "description"},
		{"empty name", skill("name:\ndescription: Use when x.\n"), "name"},
		{"tab indentation", skill("name: x\ndescription: Use when x.\n\tbad: y\n"), "tab"},
		{"unbalanced quote", skill("name: x\ndescription: \"Use when x.\n"), "quote"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Check("skills/x/SKILL.md", tc.body)
			if err == nil {
				t.Fatalf("Check accepted %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("error %q does not mention %q", err, tc.says)
			}
		})
	}
}

func TestASkillTooBigForTheHarnessIsRejected(t *testing.T) {
	// Claude Code skips a SKILL.md over a megabyte, so one would install and never load.
	body := skill("name: x\ndescription: Use when x.\n")
	body = append(body, []byte(strings.Repeat("a", 1<<20))...)

	err := Check("skills/x/SKILL.md", body)
	if err == nil {
		t.Fatal("Check accepted a skill larger than the harness will load")
	}
	if !strings.Contains(err.Error(), "large") {
		t.Errorf("error %q does not say what is wrong", err)
	}
}

func TestASkillNamedAfterAReservedDirectoryIsRejected(t *testing.T) {
	// Claude Code treats a skill directory containing one of these as a plugin, so it
	// would be adopted with lifecycle hooks rather than loaded as a skill.
	for _, reserved := range []string{"hooks", "agents", "themes", ".claude-plugin"} {
		err := Check("skills/"+reserved+"/SKILL.md", skill(
			"name: x\ndescription: Use when x.\n"))
		if err == nil {
			t.Errorf("Check accepted a skill directory named %q", reserved)
		}
	}
}
