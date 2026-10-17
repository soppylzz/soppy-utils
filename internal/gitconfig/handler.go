package gitconfig

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/soppylzz/soppy-utils/internal/core"
)

// Handler runs git's commands inside core.
type Handler struct{}

func (Handler) Run(ctx context.Context, cmd core.Command, reply core.Responder) error {
	var out bytes.Buffer
	if err := Run(ctx, cmd.Session, cmd.Args, &out, reply); err != nil {
		return err
	}

	body, err := json.Marshal(core.Result{Output: out.String()})
	if err != nil {
		return err
	}
	return reply.Finish(body)
}
