package welcome

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/soppylzz/soppy-utils/internal/core"
)

// Handler runs welcome's commands inside core.
//
// Everything it produces is collected and handed back as the command's output rather
// than written anywhere, so that core never owns a stream. The command decides which
// stream the output belongs on; a client decides nothing but how to render what it was
// told.
type Handler struct{}

func (Handler) Run(ctx context.Context, cmd core.Command, reply core.Responder) error {
	var out bytes.Buffer
	if err := Run(ctx, cmd.Home, cmd.Args, &out, reply); err != nil {
		return err
	}

	body, err := json.Marshal(core.Result{Output: out.String()})
	if err != nil {
		return err
	}
	return reply.Finish(body)
}
