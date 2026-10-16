package protocol

import (
	"encoding/json"
	"testing"
)

// The wire format is the contract, so these tests pin the exact bytes rather than
// decoding into a map and checking a few keys. A field that silently changes name
// or becomes optional is a protocol break, and it should fail here.

func TestMarshalRequestWireFormat(t *testing.T) {
	req := NewRequest(7, "initialize", json.RawMessage(`{"kind":"tui"}`))

	got, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	want := `{"jsonrpc":"2.0","id":7,"method":"initialize","params":{"kind":"tui"}}`
	if string(got) != want {
		t.Errorf("request wire format\n got %s\nwant %s", got, want)
	}
}

func TestMarshalNotificationOmitsID(t *testing.T) {
	n := NewNotification("capabilities/changed", json.RawMessage(`{}`))

	got, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	want := `{"jsonrpc":"2.0","method":"capabilities/changed","params":{}}`
	if string(got) != want {
		t.Errorf("notification wire format\n got %s\nwant %s", got, want)
	}
}

func TestMarshalResponseWithResultOmitsError(t *testing.T) {
	resp := NewResult(7, json.RawMessage(`{"protocol_version":1}`))

	got, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	want := `{"jsonrpc":"2.0","id":7,"result":{"protocol_version":1}}`
	if string(got) != want {
		t.Errorf("response wire format\n got %s\nwant %s", got, want)
	}
}

func TestMarshalResponseWithErrorOmitsResult(t *testing.T) {
	resp := NewErrorResponse(7, CodeProtocolMismatch, "no shared protocol version")

	got, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error response: %v", err)
	}

	want := `{"jsonrpc":"2.0","id":7,"error":{"code":-32001,"message":"no shared protocol version"}}`
	if string(got) != want {
		t.Errorf("error response wire format\n got %s\nwant %s", got, want)
	}
}
