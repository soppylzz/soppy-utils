package thirdparty

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ToolName is the one tool this server offers.
const ToolName = "reference_package"

// Serve speaks MCP over a pair of streams until in reaches EOF.
//
// MCP is JSON-RPC 2.0, one message per line, the same envelope the daemon speaks — which
// is a coincidence of both being JSON-RPC rather than a shared implementation, and is why
// this does not reach into internal/protocol: that one is the contract between spu and
// its own client, and this one is somebody else's.
func Serve(ctx context.Context, in io.Reader, out io.Writer, r Resolver) error {
	dec := json.NewDecoder(in)
	enc := json.NewEncoder(out)
	for {
		var msg rpcRequest
		if err := dec.Decode(&msg); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		reply, ok := handle(ctx, msg, r)
		if !ok {
			continue // a notification: no reply is expected or allowed
		}
		if err := enc.Encode(reply); err != nil {
			return err
		}
	}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func handle(ctx context.Context, msg rpcRequest, r Resolver) (rpcResponse, bool) {
	if len(msg.ID) == 0 {
		return rpcResponse{}, false
	}
	reply := rpcResponse{JSONRPC: "2.0", ID: msg.ID}

	switch msg.Method {
	case "initialize":
		reply.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "soppy-clone-third-party", "version": "1"},
		}

	case "tools/list":
		reply.Result = map[string]any{"tools": []any{toolDefinition()}}

	case "tools/call":
		result, err := callTool(ctx, msg.Params, r)
		if err != nil {
			reply.Result = map[string]any{
				"content": []any{map[string]any{"type": "text", "text": err.Error()}},
				"isError": true,
			}
			break
		}
		reply.Result = map[string]any{
			"content": []any{map[string]any{"type": "text", "text": result}},
		}

	default:
		// A notification we do not know is ignored rather than answered: MCP requires
		// that a notification gets no reply, and answering one desynchronises the stream.
		if strings.HasPrefix(msg.Method, "notifications/") {
			return rpcResponse{}, false
		}
		reply.Error = &rpcError{Code: -32601, Message: "unknown method " + msg.Method}
	}
	return reply, true
}

func toolDefinition() map[string]any {
	return map[string]any{
		"name": ToolName,
		"description": "Return a local checkout of a third-party package to read. " +
			"A checkout whose major version matches is reused without any network access. " +
			"An exact version is never fetched: the call returns what it needs and why, " +
			"and you ask the user before fetching it.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"package": map[string]any{
					"type":        "string",
					"description": "A package name like github.com/owner/repo, or a URL.",
				},
				"version": map[string]any{
					"type": "string",
					"description": "The version wanted, as a major (\"8\"), a full version " +
						"(\"8.1.0\" or \"v8.1.0\"), or omitted for the newest release. A major " +
						"means the newest release of that major. The major decides whether a " +
						"checkout already in the cache will do.",
				},
				"pin": map[string]any{
					"type":        "boolean",
					"description": "Ask for that exact version. Needs a reason, and needs the user.",
				},
				"reason": map[string]any{
					"type":        "string",
					"description": "Why that exact version is needed, shown to the user.",
				},
			},
			"required": []string{"package"},
		},
	}
}

func callTool(ctx context.Context, params json.RawMessage, r Resolver) (string, error) {
	var call struct {
		Name      string `json:"name"`
		Arguments struct {
			Package string `json:"package"`
			Version string `json:"version"`
			Pin     bool   `json:"pin"`
			Reason  string `json:"reason"`
		} `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return "", fmt.Errorf("tool arguments: %w", err)
	}
	if call.Name != ToolName {
		return "", fmt.Errorf("unknown tool %q", call.Name)
	}
	if call.Arguments.Package == "" {
		return "", fmt.Errorf("a package name or URL is required")
	}

	got, err := r.Resolve(ctx, Request{
		Package: call.Arguments.Package,
		Version: call.Arguments.Version,
		Pin:     call.Arguments.Pin,
		Reason:  call.Arguments.Reason,
	})
	if err != nil {
		return "", err
	}
	if got.Decision != nil {
		// Not an error: the tool did what it can, and the rest is a question for a
		// person. Saying so plainly is what lets the agent ask rather than guess.
		return fmt.Sprintf(
			"%s is pinned to %s, which is not in the local cache.\n"+
				"Reason given: %s\n"+
				"Nothing was fetched. Ask the user whether to fetch that exact version, "+
				"then call again with pin=false to accept the matching major.",
			got.Decision.Package, got.Decision.Version, orNone(got.Decision.Reason)), nil
	}
	return fmt.Sprintf("Reading %s at %s:\n%s", got.Version, got.Path, got.Path), nil
}

func orNone(s string) string {
	if s == "" {
		return "(none given)"
	}
	return s
}
