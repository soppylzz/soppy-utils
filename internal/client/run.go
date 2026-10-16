package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/soppylzz/soppy-utils/internal/action"
	"github.com/soppylzz/soppy-utils/internal/core"
	"github.com/soppylzz/soppy-utils/internal/cored"
	"github.com/soppylzz/soppy-utils/internal/form"
	"github.com/soppylzz/soppy-utils/internal/protocol"
)

// Client runs one command against core.
//
// It is the whole of what a terminal adds to core: it draws what core described,
// performs what core asked for, and routes what core produced to the stream core chose.
type Client struct {
	conn   *cored.Conn
	in     io.Reader
	out    io.Writer
	errOut io.Writer

	// Interactive says whether a person can be asked. Core is told this at the
	// handshake and should not send a form to a client that cannot answer one, but a
	// question that arrives anyway must not hang it.
	Interactive bool

	// Ask draws a form and returns the answers.
	Ask func(form.Spec) (map[string]any, error)

	// Confirm asks a yes or no question.
	Confirm func(question string) (bool, error)
}

func New(conn *cored.Conn, in io.Reader, out, errOut io.Writer) *Client {
	return &Client{
		conn:   conn,
		in:     in,
		out:    out,
		errOut: errOut,
		Ask: func(spec form.Spec) (map[string]any, error) {
			return drawForm(context.Background(), in, errOut, spec)
		},
		Confirm: func(question string) (bool, error) {
			return drawConfirm(context.Background(), in, errOut, question)
		},
	}
}

// Run asks core to run one command and follows it to its end, returning the exit code a
// shell should see.
func (c *Client) Run(ctx context.Context, util string, args []string) (int, error) {
	params, err := json.Marshal(map[string]any{"util": util, "args": args})
	if err != nil {
		return 1, err
	}
	if err := c.conn.Send(protocol.NewRequest(1, "command/run", params)); err != nil {
		return 1, err
	}

	sessionID := ""
	for {
		msg, err := c.conn.Read()
		if err != nil {
			return 1, err
		}

		if msg.Kind() == protocol.KindResponse {
			if sessionID == "" {
				// The reply to command/run, which arrives before any of the session's
				// own messages.
				if msg.Error != nil {
					return 1, rpcError{err: msg.Error}
				}
				var started struct {
					SessionID string `json:"session_id"`
				}
				if err := json.Unmarshal(msg.Result, &started); err != nil {
					return 1, fmt.Errorf("core started a session without naming it: %w", err)
				}
				sessionID = started.SessionID
				continue
			}
			// An acknowledgement of something this client sent. Core reports a bad
			// answer here, and it is worth showing, but nothing is waiting on it.
			if msg.Error != nil {
				fmt.Fprintf(c.errOut, "core: %s\n", msg.Error.Message)
			}
			continue
		}

		switch msg.Method {
		case "session/notify":
			if err := c.notify(msg.Params); err != nil {
				return 1, err
			}

		case "session/request_input":
			if err := c.answerForm(ctx, sessionID, msg.Params); err != nil {
				return 1, err
			}

		case "session/request_confirm":
			if err := c.answerConfirm(ctx, sessionID, msg.Params); err != nil {
				return 1, err
			}

		case "session/request_action":
			if err := c.performRequestedActions(ctx, sessionID, msg.Params); err != nil {
				return 1, err
			}

		case "session/finish":
			return c.finish(msg.Params)

		case "server/draining":
			// Restarting the command could apply half of it twice. Saying so is worse
			// than a silent retry only until the retry is the thing that hurts.
			return 1, errors.New("core is shutting down; run the command again")
		}
	}
}

func (c *Client) notify(params json.RawMessage) error {
	var got struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(params, &got); err != nil {
		return fmt.Errorf("session/notify: %w", err)
	}
	// Diagnostics belong on stderr, which is the rule this project already follows: a
	// client that wrote progress to stdout would corrupt anything being piped.
	fmt.Fprintln(c.errOut, got.Message)
	return nil
}

func (c *Client) answerForm(ctx context.Context, sessionID string, params json.RawMessage) error {
	var got struct {
		RequestID int64     `json:"request_id"`
		Spec      form.Spec `json:"spec"`
	}
	if err := json.Unmarshal(params, &got); err != nil {
		return fmt.Errorf("session/request_input: %w", err)
	}

	if !c.Interactive || c.Ask == nil {
		return c.abandon(sessionID, "this command needs a terminal to ask a question")
	}

	values, err := c.Ask(got.Spec)
	if err != nil {
		return c.abandon(sessionID, err.Error())
	}
	answer, err := json.Marshal(values)
	if err != nil {
		return err
	}
	return c.respond(sessionID, got.RequestID, answer)
}

func (c *Client) answerConfirm(ctx context.Context, sessionID string, params json.RawMessage) error {
	var got struct {
		RequestID int64  `json:"request_id"`
		Question  string `json:"question"`
	}
	if err := json.Unmarshal(params, &got); err != nil {
		return fmt.Errorf("session/request_confirm: %w", err)
	}

	confirmed := false
	if c.Interactive && c.Confirm != nil {
		ok, err := c.Confirm(got.Question)
		if err != nil {
			return c.abandon(sessionID, err.Error())
		}
		confirmed = ok
	}
	// A question that cannot be asked is answered no, and the session carries on: the
	// command decides what a refusal means, and a destructive step must not proceed
	// without somebody approving it.
	answer, err := json.Marshal(map[string]any{"confirmed": confirmed})
	if err != nil {
		return err
	}
	return c.respond(sessionID, got.RequestID, answer)
}

func (c *Client) performRequestedActions(ctx context.Context, sessionID string, params json.RawMessage) error {
	var got struct {
		RequestID int64           `json:"request_id"`
		Reason    string          `json:"reason"`
		Actions   []action.Action `json:"actions"`
	}
	if err := json.Unmarshal(params, &got); err != nil {
		return fmt.Errorf("session/request_action: %w", err)
	}

	req := action.Request{Reason: got.Reason, Actions: got.Actions}

	var confirm func() bool
	if c.Confirm != nil {
		confirm = func() bool {
			ok, err := c.Confirm("Run this?")
			return err == nil && ok
		}
	}

	// performActions decides whether to ask. Only work that changes the machine is put to
	// the user, and that is a fact about the work rather than about this client.
	outcome := performActions(ctx, c.in, c.errOut, req, confirm)

	answer, err := json.Marshal(outcome)
	if err != nil {
		return err
	}
	return c.respond(sessionID, got.RequestID, answer)
}

// abandon cancels the session and reports why.
//
// A question the client cannot answer has to end the command. Waiting would hang with
// no way out, and answering with something invented would be worse than failing.
func (c *Client) abandon(sessionID, reason string) error {
	params, err := json.Marshal(map[string]string{"session_id": sessionID})
	if err == nil {
		_ = c.conn.Send(protocol.NewRequest(1, "session/cancel", params))
	}
	return errors.New(reason)
}

func (c *Client) respond(sessionID string, requestID int64, answer json.RawMessage) error {
	params, err := json.Marshal(map[string]any{
		"session_id": sessionID,
		"request_id": requestID,
		"answer":     answer,
	})
	if err != nil {
		return err
	}
	// Sent without waiting for the acknowledgement. The reply arrives in the same
	// stream and is reported when it does, so nothing is lost by not blocking on it —
	// and blocking would mean the messages that arrived meanwhile had nowhere to go.
	return c.conn.Send(protocol.NewRequest(1, "session/respond", params))
}

func (c *Client) finish(params json.RawMessage) (int, error) {
	var got struct {
		Result core.Result `json:"result"`
		Error  string      `json:"error"`
	}
	if err := json.Unmarshal(params, &got); err != nil {
		return 1, fmt.Errorf("session/finish: %w", err)
	}

	if got.Error != "" {
		// Returned rather than printed: whoever ran the client decides what to do with a
		// failure, and a library that prints leaves no way for the caller to.
		return 1, errors.New(got.Error)
	}

	if got.Result.Output != "" {
		dest := c.out
		if got.Result.OutputTo == "stderr" {
			dest = c.errOut
		}
		// Written verbatim. The command owns these bytes: a licence body has to reach
		// the file exactly as rendered, and a client that tidied the edges would change
		// what the user saves.
		fmt.Fprint(dest, got.Result.Output)
	}
	return 0, nil
}

// actionPrompt is what the user is shown before anything runs.
func actionPrompt(req action.Request) string {
	lines := make([]string, 0, len(req.Actions)+1)
	if req.Reason != "" {
		lines = append(lines, req.Reason)
	}
	for _, a := range req.Actions {
		switch {
		case len(a.Argv) > 0:
			command := strings.Join(a.Argv, " ")
			if a.NeedsElevation {
				command = "sudo " + command
			}
			lines = append(lines, "  "+command)
		case a.URL != "":
			lines = append(lines, "  download "+a.URL)
		}
	}
	return strings.Join(lines, "\n")
}

// rpcError carries a protocol error out of the client so its code stays available.
type rpcError struct{ err *protocol.Error }

func (e rpcError) Error() string {
	return fmt.Sprintf("core: %s (code %d)", e.err.Message, e.err.Code)
}
