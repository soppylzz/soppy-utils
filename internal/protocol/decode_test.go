package protocol

import "testing"

// Decode is the boundary where a hostile or merely stale peer gets caught. It is
// deliberately strict: a message that is neither a well-formed request, response,
// nor notification is a bug in one of the two implementations, and finding it here
// is far cheaper than finding it three layers into a util.

func TestDecodeRejectsMalformedJSON(t *testing.T) {
	if _, err := Decode([]byte(`{"jsonrpc":`)); err == nil {
		t.Fatal("Decode accepted malformed JSON, want error")
	}
}

func TestDecodeRejectsAbsentJSONRPCVersion(t *testing.T) {
	if _, err := Decode([]byte(`{"id":1,"method":"initialize"}`)); err == nil {
		t.Fatal("Decode accepted a message with no jsonrpc field, want error")
	}
}

func TestDecodeRejectsForeignJSONRPCVersion(t *testing.T) {
	if _, err := Decode([]byte(`{"jsonrpc":"1.0","id":1,"method":"initialize"}`)); err == nil {
		t.Fatal("Decode accepted jsonrpc 1.0, want error")
	}
}

func TestDecodeClassifiesRequest(t *testing.T) {
	msg, err := Decode([]byte(`{"jsonrpc":"2.0","id":7,"method":"initialize","params":{"kind":"tui"}}`))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if msg.Kind() != KindRequest {
		t.Errorf("Kind() = %q, want %q", msg.Kind(), KindRequest)
	}
	if msg.Method != "initialize" {
		t.Errorf("Method = %q, want initialize", msg.Method)
	}
	if msg.ID == nil || *msg.ID != 7 {
		t.Errorf("ID = %v, want 7", msg.ID)
	}
}

func TestDecodeClassifiesNotification(t *testing.T) {
	msg, err := Decode([]byte(`{"jsonrpc":"2.0","method":"capabilities/changed","params":{}}`))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if msg.Kind() != KindNotification {
		t.Errorf("Kind() = %q, want %q", msg.Kind(), KindNotification)
	}
	if msg.ID != nil {
		t.Errorf("ID = %v, want nil for a notification", *msg.ID)
	}
}

func TestDecodeClassifiesResponse(t *testing.T) {
	msg, err := Decode([]byte(`{"jsonrpc":"2.0","id":7,"result":{"protocol_version":1}}`))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if msg.Kind() != KindResponse {
		t.Errorf("Kind() = %q, want %q", msg.Kind(), KindResponse)
	}
	if len(msg.Result) == 0 {
		t.Error("Result is empty, want the response payload")
	}
}

func TestDecodeTreatsExplicitNullResultAsPresent(t *testing.T) {
	msg, err := Decode([]byte(`{"jsonrpc":"2.0","id":7,"result":null}`))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if msg.Kind() != KindResponse {
		t.Errorf("Kind() = %q, want %q: an explicit null result is a response", msg.Kind(), KindResponse)
	}
}

func TestDecodeClassifiesErrorResponse(t *testing.T) {
	msg, err := Decode([]byte(`{"jsonrpc":"2.0","id":7,"error":{"code":-32001,"message":"no shared version"}}`))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if msg.Kind() != KindResponse {
		t.Errorf("Kind() = %q, want %q", msg.Kind(), KindResponse)
	}
	if msg.Error == nil || msg.Error.Code != -32001 {
		t.Errorf("Error = %v, want code -32001", msg.Error)
	}
}

func TestDecodeRejectsResponseCarryingBothResultAndError(t *testing.T) {
	line := `{"jsonrpc":"2.0","id":7,"result":{},"error":{"code":-1,"message":"x"}}`
	if _, err := Decode([]byte(line)); err == nil {
		t.Fatal("Decode accepted a response with both result and error, want error")
	}
}

func TestDecodeRejectsResponseCarryingNeitherResultNorError(t *testing.T) {
	if _, err := Decode([]byte(`{"jsonrpc":"2.0","id":7}`)); err == nil {
		t.Fatal("Decode accepted a response with neither result nor error, want error")
	}
}

func TestDecodeRejectsRequestCarryingAResult(t *testing.T) {
	line := `{"jsonrpc":"2.0","id":7,"method":"initialize","result":{}}`
	if _, err := Decode([]byte(line)); err == nil {
		t.Fatal("Decode accepted a request carrying a result, want error")
	}
}

func TestDecodeRejectsMethodlessIDlessMessage(t *testing.T) {
	if _, err := Decode([]byte(`{"jsonrpc":"2.0"}`)); err == nil {
		t.Fatal("Decode accepted a message with no method and no id, want error")
	}
}
