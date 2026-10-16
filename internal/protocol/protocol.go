// Package protocol defines the wire representation shared by the spu client and the
// spu core daemon.
//
// It knows nothing about how the bytes travel. Transport is a separate package, so
// nothing here learns whether the peer is a Unix socket, a Windows named pipe, or
// something added later; and nothing in a util package imports this package, so
// domain code never sees a JSON-RPC type.
package protocol

import "encoding/json"

// CodeProtocolMismatch is returned when the client and core share no protocol
// version. The client responds by asking core to stop and reconnecting, which starts
// the newly installed build.
const CodeProtocolMismatch = -32001

// Request is a message that expects a response.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Notification is a message that expects no response. The absent id is what
// distinguishes it on the wire.
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response answers a Request. Exactly one of Result and Error is set.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error is the wire form of a failure.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func NewRequest(id int64, method string, params json.RawMessage) Request {
	return Request{JSONRPC: "2.0", ID: id, Method: method, Params: params}
}

func NewNotification(method string, params json.RawMessage) Notification {
	return Notification{JSONRPC: "2.0", Method: method, Params: params}
}

func NewResult(id int64, result json.RawMessage) Response {
	return Response{JSONRPC: "2.0", ID: id, Result: result}
}

func NewErrorResponse(id int64, code int, message string) Response {
	return Response{JSONRPC: "2.0", ID: id, Error: &Error{Code: code, Message: message}}
}
