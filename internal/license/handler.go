package license

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/soppylzz/soppy-utils/internal/core"
)

// Handler runs license's commands inside core.
//
// The rendered licence is the file a user saves, so it is collected and handed back
// exactly as rendered rather than written anywhere: core has no stream of its own, and
// a client that tidied the edges would change what gets saved.
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
