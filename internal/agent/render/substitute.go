package render

import (
	"bytes"
	"fmt"
	"regexp"
)

// placeholder finds an answer written into the text.
//
// `{{key}}` and nothing looser: a single brace is common in shell and in format strings,
// and a template that substituted those would corrupt the documents that contain them.
var placeholder = regexp.MustCompile(`\{\{([a-zA-Z0-9_-]+)\}\}`)

// substitute writes the answers into a segment's text.
//
// A document that names the directory somebody chose has to say which directory that is,
// and the choice is not known when the template is written. Only a segment that contains a
// placeholder is touched, so the documents that have none are copied through unchanged.
//
// An answer nobody gave is an error rather than a placeholder left in the file: a
// generated document containing `{{reference-dir}}` reads as a document that meant to say
// something and did not.
func substitute(body []byte, answers map[string]any, what string) ([]byte, error) {
	if !bytes.Contains(body, []byte("{{")) {
		return body, nil
	}

	var failure error
	out := placeholder.ReplaceAllFunc(body, func(match []byte) []byte {
		key := string(placeholder.FindSubmatch(match)[1])
		value, ok := answers[key]
		if !ok {
			if failure == nil {
				failure = fmt.Errorf("%s asks for %q, which nothing answered", what, key)
			}
			return match
		}
		return []byte(text(value))
	})
	if failure != nil {
		return nil, failure
	}
	return out, nil
}

// text is how an answer reads inside a sentence.
func text(value any) string {
	switch got := value.(type) {
	case nil:
		return ""
	case string:
		return got
	case bool:
		if got {
			return "true"
		}
		return "false"
	case []string:
		return join(got)
	case []any:
		out := make([]string, 0, len(got))
		for _, v := range got {
			out = append(out, text(v))
		}
		return join(out)
	default:
		return fmt.Sprint(got)
	}
}

func join(values []string) string {
	out := ""
	for i, v := range values {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return out
}
