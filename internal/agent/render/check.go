package render

import (
	"fmt"
	"path"
	"strings"
)

// skillMaxBytes is the largest SKILL.md a harness will load. Claude Code skips a bigger
// one, so a skill over this installs and never loads — a failure with no symptom.
const skillMaxBytes = 1 << 20

// reservedSkillDirs are directory names Claude Code adopts as a plugin rather than
// loading as a skill, which brings lifecycle hooks and monitors with them.
var reservedSkillDirs = map[string]bool{
	".claude-plugin": true,
	"agents":         true,
	"output-styles":  true,
	"themes":         true,
	"hooks":          true,
	"monitors":       true,
	"workflows":      true,
}

// Check validates rendered output that has a format its reader insists on.
//
// Only a skill is checked, because only a skill fails quietly: a harness that cannot
// parse its frontmatter still loads it, drops every field, falls back to the directory
// name and the first line of the body, and says nothing at normal verbosity. A document
// is prose and may say anything.
//
// This reads the frontmatter structurally rather than parsing YAML. It is a guard on text
// we generate ourselves from templates we control, not a general YAML implementation, and
// it catches the breakages that matter here: a missing block, a missing field, a tab where
// indentation belongs, and an unbalanced quote.
func Check(outPath string, body []byte) error {
	if path.Base(outPath) != "SKILL.md" {
		return nil
	}

	for _, dir := range strings.Split(path.Dir(outPath), "/") {
		if reservedSkillDirs[strings.ToLower(dir)] {
			return fmt.Errorf("%s: %q is a directory name the harness adopts as a plugin "+
				"rather than loading as a skill", outPath, dir)
		}
	}
	if len(body) > skillMaxBytes {
		return fmt.Errorf("%s: %d bytes is larger than the harness loads, so the skill "+
			"would install and never load", outPath, len(body))
	}

	front, rest, ok := splitFrontmatter(string(body))
	if !ok {
		return fmt.Errorf("%s: no frontmatter block: a SKILL.md must open with --- and "+
			"close it again", outPath)
	}
	if strings.Contains(front, "\t") {
		return fmt.Errorf("%s: the frontmatter contains a tab; YAML does not allow one "+
			"for indentation, and the harness would drop every field", outPath)
	}

	fields := map[string]string{}
	for _, line := range strings.Split(front, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, " ") {
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			return fmt.Errorf("%s: frontmatter line %q is not a key and a value", outPath, line)
		}
		value = strings.TrimSpace(value)
		if err := checkQuotes(outPath, key, value); err != nil {
			return err
		}
		fields[strings.TrimSpace(key)] = strings.Trim(value, `"' `)
	}

	for _, key := range []string{"name", "description"} {
		if fields[key] == "" {
			return fmt.Errorf("%s: the frontmatter has no %s, so the harness would fall "+
				"back to the directory name and the first line of the body", outPath, key)
		}
	}
	_ = rest
	return nil
}

// splitFrontmatter returns the block between the opening and closing --- lines.
func splitFrontmatter(body string) (front, rest string, ok bool) {
	if !strings.HasPrefix(body, "---\n") && body != "---" {
		return "", body, false
	}
	lines := strings.Split(strings.TrimPrefix(body, "---\n"), "\n")
	for i, line := range lines {
		if strings.TrimRight(line, " \r") == "---" {
			return strings.Join(lines[:i], "\n"), strings.Join(lines[i+1:], "\n"), true
		}
	}
	return "", body, false
}

func checkQuotes(outPath, key, value string) error {
	for _, q := range []string{`"`, `'`} {
		if strings.HasPrefix(value, q) != strings.HasSuffix(value, q) || strings.Count(value, q)%2 != 0 {
			return fmt.Errorf("%s: the value of %q has an unbalanced quote, so the "+
				"frontmatter would not parse and every field would be dropped", outPath, strings.TrimSpace(key))
		}
	}
	return nil
}
