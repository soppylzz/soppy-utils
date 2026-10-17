// Package render turns an item's answers into the bytes of one output.
//
// Nothing here touches the filesystem or a terminal: it is given answers, an output, and
// the embedded content, and it returns a file's worth of text. That is what makes the
// decision about what to do with those bytes a separate question, answered separately.
package render

import (
	"bytes"
	"fmt"
	"io/fs"
	"strconv"

	"github.com/soppylzz/soppy-utils/internal/agent/item"
)

// Output renders one output's content.
//
// A segment whose condition does not hold is absent from the result rather than present
// and empty, so a document for a project that does not use the thing reads as though the
// thing was never mentioned.
func Output(out item.Output, answers map[string]any, files fs.FS) ([]byte, error) {
	var buf bytes.Buffer
	for _, seg := range out.Content {
		if !holds(seg.When, answers) {
			continue
		}
		var body []byte
		if seg.Text != "" {
			body = []byte(seg.Text)
		} else {
			read, err := fs.ReadFile(files, seg.File)
			if err != nil {
				return nil, fmt.Errorf("content %q: %w", seg.File, err)
			}
			body = read
		}
		// Answers are written into the text, whichever it came from: a document that names
		// something the reader chose cannot say which until the choice is made.
		body, err := substitute(body, answers, "content "+segName(seg))
		if err != nil {
			return nil, err
		}
		buf.Write(body)
	}
	return buf.Bytes(), nil
}

// holds reports whether a condition is satisfied by the answers.
//
// An absent answer never satisfies a condition. The alternative — treating "nobody was
// asked" as yes — adds text to a file whose reader never chose it, and the only reason
// the answer is absent is that the question did not exist when the item was recorded.
func holds(when *item.Condition, answers map[string]any) bool {
	if when == nil {
		return true
	}
	answer, ok := answers[when.Answer]
	if !ok {
		return false
	}
	if when.Is == "" {
		return truthy(answer)
	}
	return matches(answer, when.Is)
}

func truthy(v any) bool {
	switch got := v.(type) {
	case nil:
		return false
	case bool:
		return got
	case string:
		return got != ""
	case []string:
		return len(got) > 0
	case []any:
		return len(got) > 0
	default:
		return true
	}
}

// matches reads a choice, or membership of one.
//
// The second case is the item picker: its answer is a set, and a segment that belongs to
// one item is written when that item is in it.
func matches(v any, want string) bool {
	switch got := v.(type) {
	case string:
		return got == want
	case []string:
		for _, s := range got {
			if s == want {
				return true
			}
		}
	case []any:
		for _, s := range got {
			if text, ok := s.(string); ok && text == want {
				return true
			}
		}
	}
	return false
}

// segName is how a segment is referred to in an error.
func segName(seg item.Segment) string {
	if seg.File != "" {
		return strconv.Quote(seg.File)
	}
	return "written inline"
}
