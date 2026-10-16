package thirdparty

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func serve(t *testing.T, cache string, lines ...string) []map[string]any {
	t.Helper()
	r := Resolver{CacheDir: cache, Fetch: (&fakeFetch{}).run, Log: &strings.Builder{}}
	var out strings.Builder
	in := strings.NewReader(strings.Join(lines, "\n") + "\n")
	if err := Serve(context.Background(), in, &out, r); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	var replies []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("reply %q is not JSON: %v", line, err)
		}
		replies = append(replies, m)
	}
	return replies
}

func TestServeAnswersInitialiseAndListsTheTool(t *testing.T) {
	replies := serve(t, t.TempDir(),
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)

	if len(replies) != 2 {
		t.Fatalf("got %d replies, want 2", len(replies))
	}
	if _, ok := replies[0]["result"]; !ok {
		t.Errorf("initialize was not answered: %v", replies[0])
	}
	body, _ := json.Marshal(replies[1])
	if !strings.Contains(string(body), ToolName) {
		t.Errorf("tools/list does not name the tool: %s", body)
	}
}

func TestANotificationIsNotAnswered(t *testing.T) {
	// MCP requires it: a reply to a notification desynchronises the stream, and the
	// harness is entitled to send one at any time.
	replies := serve(t, t.TempDir(), `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if len(replies) != 0 {
		t.Errorf("a notification was answered: %v", replies)
	}
}

func TestCallingTheToolReturnsACheckoutPath(t *testing.T) {
	cache := t.TempDir()
	replies := serve(t, cache,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"reference_package",`+
			`"arguments":{"package":"github.com/foo/bar","version":"v1.4.0"}}}`)

	body, _ := json.Marshal(replies[0])
	if !strings.Contains(string(body), "github.com/foo/bar") {
		t.Errorf("the path was not returned: %s", body)
	}
	if isErr, _ := replies[0]["result"].(map[string]any)["isError"].(bool); isErr {
		t.Errorf("the call reported an error: %s", body)
	}
}

func TestAPinnedVersionComesBackAsAQuestion(t *testing.T) {
	replies := serve(t, t.TempDir(),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"reference_package",`+
			`"arguments":{"package":"github.com/foo/bar","version":"v1.4.0","pin":true,`+
			`"reason":"the API changed in v1.5"}}}`)

	body, _ := json.Marshal(replies[0])
	if !strings.Contains(string(body), "Ask the user") {
		t.Errorf("a pinned version did not come back as a question: %s", body)
	}
	if !strings.Contains(string(body), "the API changed in v1.5") {
		t.Errorf("the reason was not carried through: %s", body)
	}
}

func TestAnUnknownMethodIsAnError(t *testing.T) {
	replies := serve(t, t.TempDir(), `{"jsonrpc":"2.0","id":1,"method":"tools/nope"}`)
	if replies[0]["error"] == nil {
		t.Errorf("an unknown method was answered with a result: %v", replies[0])
	}
}
