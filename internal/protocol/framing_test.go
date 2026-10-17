package protocol

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

func TestReaderDecodesOneWrittenMessage(t *testing.T) {
	var buf bytes.Buffer
	params := json.RawMessage(`{"kind":"tui"}`)
	if err := NewWriter(&buf).WriteMessage(NewRequest(1, "initialize", params)); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	msg, err := NewReader(&buf).Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if msg.Method != "initialize" {
		t.Errorf("Method = %q, want initialize", msg.Method)
	}
	if msg.ID == nil || *msg.ID != 1 {
		t.Errorf("ID = %v, want 1", msg.ID)
	}
}

func TestReaderDecodesTwoMessagesArrivingTogether(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	for _, method := range []string{"capabilities/list", "server/shutdown"} {
		if err := w.WriteMessage(NewRequest(1, method, nil)); err != nil {
			t.Fatalf("WriteMessage(%s): %v", method, err)
		}
	}

	r := NewReader(&buf)
	for _, want := range []string{"capabilities/list", "server/shutdown"} {
		msg, err := r.Read()
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if msg.Method != want {
			t.Errorf("Method = %q, want %q", msg.Method, want)
		}
	}
}

func TestReaderReassemblesAMessageSplitAcrossChunks(t *testing.T) {
	var buf bytes.Buffer
	if err := NewWriter(&buf).WriteMessage(NewRequest(1, "initialize", nil)); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	// One byte at a time is the worst case a real socket can produce.
	msg, err := NewReader(iotest.OneByteReader(&buf)).Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if msg.Method != "initialize" {
		t.Errorf("Method = %q, want initialize", msg.Method)
	}
}

func TestWriterKeepsAMessageContainingNewlinesOnOneLine(t *testing.T) {
	var buf bytes.Buffer
	params := json.RawMessage(`{"message":"line one\nline two"}`)
	if err := NewWriter(&buf).WriteMessage(NewRequest(1, "command/run", params)); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	if got := strings.Count(buf.String(), "\n"); got != 1 {
		t.Fatalf("wrote %d newlines, want exactly 1: framing would break", got)
	}

	msg, err := NewReader(bytes.NewReader(buf.Bytes())).Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	var got struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(msg.Params, &got); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if got.Message != "line one\nline two" {
		t.Errorf("message = %q, want the newline preserved", got.Message)
	}
}

func TestReaderReturnsEOFAtAMessageBoundary(t *testing.T) {
	var buf bytes.Buffer
	if err := NewWriter(&buf).WriteMessage(NewNotification("capabilities/changed", nil)); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	r := NewReader(&buf)
	if _, err := r.Read(); err != nil {
		t.Fatalf("first Read: %v", err)
	}
	if _, err := r.Read(); err != io.EOF {
		t.Errorf("second Read = %v, want io.EOF", err)
	}
}

func TestReaderRejectsATruncatedTrailingMessage(t *testing.T) {
	// A peer that dies mid-write leaves this behind. Silently treating it as a clean
	// end of stream would turn a crash into a mystery.
	complete := `{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n"
	truncated := `{"jsonrpc":"2.0","id":2,`

	r := NewReader(strings.NewReader(complete + truncated))
	if _, err := r.Read(); err != nil {
		t.Fatalf("first Read: %v", err)
	}
	if _, err := r.Read(); err == nil || err == io.EOF {
		t.Errorf("second Read = %v, want a truncation error", err)
	}
}

func TestReaderRejectsAMessageLargerThanTheCap(t *testing.T) {
	// One client must not be able to make the daemon allocate without bound.
	huge := `{"jsonrpc":"2.0","id":1,"method":"x","params":{"pad":"` +
		strings.Repeat("a", MaxMessageBytes) + `"}}` + "\n"

	if _, err := NewReader(strings.NewReader(huge)).Read(); err == nil {
		t.Fatal("Read accepted a message over the cap, want error")
	}
}

func TestReaderRejectsAnInvalidMessage(t *testing.T) {
	// Framing and validation are separate concerns, but a caller should never have to
	// remember to call both.
	if _, err := NewReader(strings.NewReader(`{"jsonrpc":"1.0","id":1,"method":"x"}` + "\n")).Read(); err == nil {
		t.Fatal("Read accepted jsonrpc 1.0, want error")
	}
}
