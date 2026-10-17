package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Kind is the shape a decoded message turned out to be.
type Kind string

const (
	KindRequest      Kind = "request"
	KindResponse     Kind = "response"
	KindNotification Kind = "notification"
)

// Incoming is a decoded message, whichever of the three shapes it is. The fields are
// flat rather than a union so that the transport loop can read the method or the id
// without unwrapping anything.
//
// ID is a pointer because a notification has no id at all, which is different from
// having a null one. Result and Error follow the same rule on the wire: an absent key
// leaves a nil RawMessage, while an explicit null decodes to the four bytes "null",
// so len() is enough to tell "present" from "absent" without a second pointer.
type Incoming struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	Result  json.RawMessage `json:"result"`
	Error   *Error          `json:"error"`
}

// Kind reports which of the three message shapes this is.
func (m Incoming) Kind() Kind {
	switch {
	case m.Method != "" && m.ID != nil:
		return KindRequest
	case m.Method != "":
		return KindNotification
	default:
		return KindResponse
	}
}

// Decode parses one message and checks that it is a coherent member of the protocol.
//
// The checks are deliberately strict. Both ends of this protocol are ours, so a
// message that is neither a well-formed request, response, nor notification is a
// disagreement between two builds, and catching it at the boundary is much cheaper
// than catching it three layers into a util.
func Decode(line []byte) (Incoming, error) {
	var m Incoming
	if err := json.Unmarshal(line, &m); err != nil {
		return Incoming{}, fmt.Errorf("malformed message: %w", err)
	}
	if m.JSONRPC != "2.0" {
		return Incoming{}, fmt.Errorf("unsupported jsonrpc version %q, want %q", m.JSONRPC, "2.0")
	}

	hasResult := len(m.Result) > 0
	hasError := m.Error != nil

	switch {
	case m.Method != "":
		if hasResult || hasError {
			return Incoming{}, errors.New("request or notification carries a result or error")
		}
		return m, nil

	case m.ID != nil:
		switch {
		case hasResult && hasError:
			return Incoming{}, errors.New("response carries both result and error")
		case !hasResult && !hasError:
			return Incoming{}, errors.New("response carries neither result nor error")
		}
		return m, nil

	default:
		return Incoming{}, errors.New("message has neither method nor id")
	}
}
