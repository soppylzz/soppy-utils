package transport

import "errors"

// ErrInUse reports that another core daemon already owns this endpoint.
//
// A client that meets it attaches to the daemon that is already running instead of
// starting a second one. A daemon that meets it exits quietly rather than unlinking
// the winner's socket: two daemons on one endpoint would each believe they own the
// state, and the loser's cleanup would be the more destructive half of the bug.
var ErrInUse = errors.New("another core is already listening on this endpoint")
