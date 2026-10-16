// Package cored is the core daemon: one process per user that owns util behaviour,
// persisted state, and the registry.
//
// It is started on demand by the first client that cannot reach it, and it exits
// after an idle period. Nothing here assumes an attached terminal. The client owns
// the terminal and the presentation; core only describes what it needs and what it
// found, which is what lets a terminal client and a future window client share it.
package cored

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/soppylzz/soppy-utils/internal/action"
	"github.com/soppylzz/soppy-utils/internal/form"
	"github.com/soppylzz/soppy-utils/internal/protocol"
	"github.com/soppylzz/soppy-utils/internal/registry"
	"github.com/soppylzz/soppy-utils/internal/transport"
)

// Command is one client request for core to do something.
type Command struct {
	Util string
	Args []string
}

// Responder is how a running command talks to the client that asked for it.
type Responder interface {
	// Notify reports progress. Nobody is expected to answer.
	Notify(message string) error

	// Ask sends a form and waits for the values. It is how a command collects what it
	// needs without knowing how the client will ask.
	Ask(ctx context.Context, spec form.Spec) (map[string]any, error)

	// AskActions offers work core cannot do itself, such as installing a package that
	// needs privileges. A client that cannot perform it is told what is missing and
	// asked nothing, and the outcome says so.
	AskActions(ctx context.Context, req action.Request) (action.Outcome, error)

	// Finish ends the session and carries its result.
	Finish(result json.RawMessage) error
}

// Handler runs commands. A util package implements its behaviour behind this seam;
// the daemon knows nothing about what any particular util does, and a Handler knows
// nothing about sockets or JSON-RPC.
type Handler interface {
	Run(ctx context.Context, cmd Command, reply Responder) error
}

// Options configures a Daemon.
type Options struct {
	Endpoint transport.Endpoint
	Handler  Handler
	Version  string

	// IdleTimeout is how long the daemon stays resident with nobody connected. Zero
	// or negative means never exit on idle.
	IdleTimeout time.Duration

	// DrainGrace bounds how long a shutdown waits for running sessions. A session that
	// overruns it is abandoned: spu update is waiting on this, and so is the user.
	DrainGrace time.Duration

	// Registry is what core can do. Nil means nothing is registered, which is how a
	// daemon starts before any util has moved onto it.
	Registry *registry.Registry

	Log io.Writer
}

// Daemon serves clients on one endpoint.
type Daemon struct {
	opts Options
	ln   *transport.Listener
	log  io.Writer
	wg   sync.WaitGroup

	// sessionSeq names sessions within this daemon's lifetime. Ids only have to be
	// unique among the sessions a client can currently see.
	sessionSeq atomic.Uint64

	draining  chan struct{}
	drainOnce sync.Once

	mu        sync.Mutex
	conns     map[*clientConn]struct{}
	idleSince time.Time

	// inflight counts running sessions across every connection, which is what a
	// shutdown waits for.
	inflight sync.WaitGroup
}

// New binds the endpoint. Binding here rather than in Serve means a daemon that loses
// the race to start reports it immediately, instead of running far enough to be
// confusing before discovering it is not the only one.
func New(opts Options) (*Daemon, error) {
	ln, err := transport.Listen(opts.Endpoint)
	if err != nil {
		return nil, err
	}
	log := opts.Log
	if log == nil {
		log = io.Discard
	}
	return &Daemon{
		opts:     opts,
		ln:       ln,
		log:      log,
		draining: make(chan struct{}),
		conns:    make(map[*clientConn]struct{}),
	}, nil
}

// requestDrain asks Serve to stop. It is safe to call from more than one connection
// and more than once.
func (d *Daemon) requestDrain() {
	d.drainOnce.Do(func() { close(d.draining) })
}

// drain stops accepting, tells the clients that are still attached, and waits a
// bounded time for running sessions.
//
// Connected clients are told so they can reconnect to the replacement rather than
// failing on their next request; a session is waited for because it may be halfway
// through writing configuration, and stopping it there would leave the user with a
// half-applied change and nobody to tell them.
func (d *Daemon) drain() {
	d.ln.Close()

	d.mu.Lock()
	attached := make([]*clientConn, 0, len(d.conns))
	for c := range d.conns {
		attached = append(attached, c)
	}
	d.mu.Unlock()
	for _, c := range attached {
		_ = c.write(protocol.NewNotification("server/draining", nil))
	}

	done := make(chan struct{})
	go func() {
		d.inflight.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(d.opts.DrainGrace):
		d.logf("drain: a session did not finish within %s", d.opts.DrainGrace)
	}
}

func (d *Daemon) registerConn(c *clientConn) {
	d.mu.Lock()
	d.conns[c] = struct{}{}
	d.idleSince = time.Time{}
	d.mu.Unlock()
}

func (d *Daemon) releaseConn(c *clientConn) {
	d.mu.Lock()
	delete(d.conns, c)
	if len(d.conns) == 0 {
		d.idleSince = time.Now()
	}
	d.mu.Unlock()
}

// idleFor is how long nobody has been connected. A daemon that has never had a client
// is idle from the moment it started, so a spawn that never gets a connection does not
// stay resident forever.
func (d *Daemon) idleFor() time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.idleSince.IsZero() {
		return 0
	}
	return time.Since(d.idleSince)
}

// Serve accepts clients until the context is cancelled, a client asks core to stop,
// or the daemon sits idle for the configured period.
//
// All three return nil, because all three are ordinary ways for a daemon to stop and
// the caller has nothing different to do about them.
func (d *Daemon) Serve(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()     // tell the connection handlers to stop
		d.ln.Close() // unblock Accept, which is how the accept goroutine learns
		d.wg.Wait()  // let them finish before this returns
	}()

	d.mu.Lock()
	d.idleSince = time.Now()
	d.mu.Unlock()

	conns := make(chan net.Conn)
	acceptErr := make(chan error, 1)
	go d.accept(ctx, conns, acceptErr)

	// Poll rather than arm and re-arm a timer on every connection change. The check is
	// a comparison, and while it matters the daemon is idle by definition, so the cost
	// is one wake-up per interval and nothing else.
	poll := time.Second
	if d.opts.IdleTimeout > 0 {
		poll = d.opts.IdleTimeout / 4
		if poll < 10*time.Millisecond {
			poll = 10 * time.Millisecond
		}
	}
	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-d.draining:
			d.drain()
			return nil
		case err := <-acceptErr:
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		case <-ticker.C:
			if d.opts.IdleTimeout > 0 && d.idleFor() >= d.opts.IdleTimeout {
				d.logf("idle for %s; exiting", d.opts.IdleTimeout)
				return nil
			}
		case conn := <-conns:
			d.wg.Add(1)
			go func() {
				defer d.wg.Done()
				d.serveConn(ctx, conn)
			}()
		}
	}
}

func (d *Daemon) accept(ctx context.Context, conns chan<- net.Conn, failed chan<- error) {
	for {
		conn, err := d.ln.Accept()
		if err != nil {
			select {
			case failed <- err:
			default:
				// Serve has already returned; nobody is listening.
			}
			return
		}
		select {
		case conns <- conn:
		case <-ctx.Done():
			conn.Close()
			return
		}
	}
}

// serveConn answers requests on one client connection until it ends.
func (d *Daemon) serveConn(ctx context.Context, raw net.Conn) {
	defer raw.Close()

	conn := newClientConn(raw)
	d.registerConn(conn)
	defer d.releaseConn(conn)
	state := &connState{conn: conn}

	sessCtx, cancelSessions := context.WithCancel(ctx)
	var sessions sync.WaitGroup
	defer func() {
		// A session must not outlive the connection that asked for it: its handler may
		// be halfway through writing configuration and the user is no longer watching.
		cancelSessions()
		sessions.Wait()
	}()

	done := make(chan struct{})
	defer close(done)
	go func() {
		// A read does not observe the context, so the only way to stop a connection
		// whose client has gone quiet is to close it underneath the read.
		select {
		case <-sessCtx.Done():
			raw.Close()
		case <-done:
		}
	}()

	r := protocol.NewReader(raw)
	for {
		msg, err := r.Read()
		if err != nil {
			if ctx.Err() == nil && !errors.Is(err, io.EOF) {
				d.logf("client connection ended: %v", err)
			}
			return
		}

		if msg.Kind() != protocol.KindRequest {
			// Only a request expects an answer, and a response or notification arriving
			// on a client connection is a bug worth seeing rather than answering.
			d.logf("ignoring an unexpected %s from a client", msg.Kind())
			continue
		}

		result, rpcErr := d.dispatch(sessCtx, state, &sessions, msg)
		reply := protocol.NewResult(*msg.ID, result)
		if rpcErr != nil {
			reply = protocol.NewErrorResponse(*msg.ID, rpcErr.Code, rpcErr.Message)
		}
		if err := conn.write(reply); err != nil {
			return
		}

		if msg.Method == "server/shutdown" {
			// Only after the reply is on the wire. Draining closes this connection, and
			// the client that asked for the shutdown is waiting for an answer on it.
			d.requestDrain()
		}
	}
}

// connState is what one connection has established about itself.
type connState struct {
	conn         *clientConn
	env          Env
	capabilities clientCapabilities
	initialized  bool

	mu       sync.Mutex
	sessions map[string]*session
}

func (s *connState) addSession(sess *session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions == nil {
		s.sessions = make(map[string]*session)
	}
	s.sessions[sess.id] = sess
}

func (s *connState) dropSession(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

func (s *connState) session(id string) (*session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	return sess, ok
}

// methodNeedsSession lists the methods that depend on the session environment, which
// arrives with the handshake.
var methodNeedsSession = map[string]bool{
	"capabilities/list": true,
	"command/run":       true,
	"session/respond":   true,
	"session/cancel":    true,
}

// methodIsKnown reports whether a method exists at all.
//
// It is checked before the session guard so that an unknown method is reported as
// unknown rather than as needing a handshake: a typo and a missing handshake send a
// caller in different directions.
func methodIsKnown(method string) bool {
	switch method {
	case "initialize", "server/shutdown":
		return true
	}
	return methodNeedsSession[method]
}

func (d *Daemon) dispatch(
	ctx context.Context, state *connState, sessions *sync.WaitGroup, msg protocol.Incoming,
) (json.RawMessage, *protocol.Error) {
	if !methodIsKnown(msg.Method) {
		return nil, &protocol.Error{
			Code:    protocol.CodeMethodNotFound,
			Message: fmt.Sprintf("unknown method %q", msg.Method),
		}
	}
	if methodNeedsSession[msg.Method] && !state.initialized {
		return nil, &protocol.Error{
			Code:    protocol.CodeInvalidParams,
			Message: msg.Method + " needs a session: send initialize first",
		}
	}

	switch msg.Method {
	case "initialize":
		return d.initialize(state, msg.Params)
	case "server/shutdown":
		return json.RawMessage(`{}`), nil
	case "capabilities/list":
		return d.capabilities(state)
	case "command/run":
		return d.startSession(ctx, state, sessions, msg.Params)
	case "session/respond":
		return d.respond(state, msg.Params)
	case "session/cancel":
		return d.cancelSession(state, msg.Params)
	}
	return nil, &protocol.Error{
		Code:    protocol.CodeInternalError,
		Message: "method " + msg.Method + " is known but was not routed",
	}
}

func (d *Daemon) respond(state *connState, params json.RawMessage) (json.RawMessage, *protocol.Error) {
	var p struct {
		SessionID string `json:"session_id"`
		RequestID int64  `json:"request_id"`

		// Answer is whatever the question asked for: a form's values, or an action
		// request's outcome. It is opaque here because the session knew what it asked,
		// and the client's answer is only meaningful to the question that waits for it.
		Answer json.RawMessage `json:"answer"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &protocol.Error{
			Code:    protocol.CodeInvalidParams,
			Message: fmt.Sprintf("session/respond params: %v", err),
		}
	}

	sess, ok := state.session(p.SessionID)
	if !ok || !sess.deliver(p.RequestID, p.Answer) {
		// A late answer is ordinary: the user filled a form and the session was
		// cancelled first. Reported, because a client answering the wrong question is
		// worth seeing, but it must not take the connection down.
		return nil, &protocol.Error{
			Code: protocol.CodeInvalidParams,
			Message: fmt.Sprintf("no question %d is waiting in session %s",
				p.RequestID, p.SessionID),
		}
	}
	return json.RawMessage(`{}`), nil
}

func (d *Daemon) cancelSession(state *connState, params json.RawMessage) (json.RawMessage, *protocol.Error) {
	var p struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &protocol.Error{
			Code:    protocol.CodeInvalidParams,
			Message: fmt.Sprintf("session/cancel params: %v", err),
		}
	}

	sess, ok := state.session(p.SessionID)
	if !ok {
		return nil, &protocol.Error{
			Code:    protocol.CodeInvalidParams,
			Message: "session " + p.SessionID + " is not running",
		}
	}
	// Cancelling the context is the whole mechanism: a handler blocked in Ask returns,
	// one blocked on a child process sees it, and the daemon reports the end of the
	// session through the same path as any other failure.
	sess.cancel()
	return json.RawMessage(`{}`), nil
}

// startSession answers immediately with a session id and runs the command in the
// background.
//
// The id goes back before the command finishes so that the client can cancel it. A
// client that had to wait for the result before learning the session's identity could
// never cancel anything, which is exactly the situation a form or a long install
// creates.
func (d *Daemon) startSession(
	ctx context.Context, state *connState, sessions *sync.WaitGroup, params json.RawMessage,
) (json.RawMessage, *protocol.Error) {
	var p struct {
		Util string   `json:"util"`
		Args []string `json:"args"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &protocol.Error{
				Code:    protocol.CodeInvalidParams,
				Message: fmt.Sprintf("command/run params: %v", err),
			}
		}
	}
	if p.Util == "" {
		return nil, &protocol.Error{
			Code:    protocol.CodeInvalidParams,
			Message: "command/run needs a util",
		}
	}

	sctx, cancel := context.WithCancel(ctx)
	s := &session{
		id:           fmt.Sprintf("s%d", d.sessionSeq.Add(1)),
		cancel:       cancel,
		conn:         state.conn,
		capabilities: state.capabilities,
	}
	state.addSession(s)

	sessions.Add(1)
	d.inflight.Add(1)
	go func() {
		defer d.inflight.Done()
		defer sessions.Done()
		defer state.dropSession(s.id)
		defer cancel()

		err := d.opts.Handler.Run(sctx, Command{Util: p.Util, Args: p.Args}, s)
		if err == nil && s.hasEnded() {
			return
		}
		if err == nil {
			err = errors.New("the command ended without a result")
		}
		d.logf("session %s: %v", s.id, err)
		if failErr := s.fail(err); failErr != nil && sctx.Err() == nil {
			d.logf("session %s: reporting the failure: %v", s.id, failErr)
		}
	}()

	out, err := json.Marshal(map[string]string{"session_id": s.id})
	if err != nil {
		return nil, &protocol.Error{Code: protocol.CodeInternalError, Message: err.Error()}
	}
	return out, nil
}

type initializeParams struct {
	Client struct {
		Kind             string             `json:"kind"`
		Version          string             `json:"version"`
		ProtocolVersions []int              `json:"protocol_versions"`
		Capabilities     clientCapabilities `json:"capabilities"`
		Env              Env                `json:"env"`
	} `json:"client"`
}

type coreInfo struct {
	Version string `json:"version"`
}

type initializeResult struct {
	ProtocolVersion int               `json:"protocol_version"`
	Core            coreInfo          `json:"core"`
	Capabilities    []registry.Report `json:"capabilities,omitempty"`
}

func (d *Daemon) initialize(state *connState, params json.RawMessage) (json.RawMessage, *protocol.Error) {
	var p initializeParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &protocol.Error{
				Code:    protocol.CodeInvalidParams,
				Message: fmt.Sprintf("initialize params: %v", err),
			}
		}
	}

	if !supports(p.Client.ProtocolVersions, protocol.Version) {
		return nil, &protocol.Error{
			Code: protocol.CodeProtocolMismatch,
			Message: fmt.Sprintf("no shared protocol version: core speaks %d, client offered %v",
				protocol.Version, p.Client.ProtocolVersions),
		}
	}

	if p.Client.Env.GOOS == "" {
		return nil, &protocol.Error{
			Code: protocol.CodeInvalidParams,
			Message: "initialize needs the client's environment: capabilities are judged " +
				"against it, not against the daemon's",
		}
	}
	state.env = p.Client.Env
	state.capabilities = p.Client.Capabilities
	state.initialized = true

	out, err := json.Marshal(initializeResult{
		ProtocolVersion: protocol.Version,
		Core:            coreInfo{Version: d.opts.Version},
		Capabilities:    d.snapshot(state.env),
	})
	if err != nil {
		return nil, &protocol.Error{Code: protocol.CodeInternalError, Message: err.Error()}
	}
	return out, nil
}

// snapshot judges every capability against one session's environment.
//
// It is computed per request rather than cached here. The daemon is the only place
// that knows which session is asking, and a cache keyed by anything coarser would
// answer one client with another client's machine.
func (d *Daemon) snapshot(env Env) []registry.Report {
	if d.opts.Registry == nil {
		return nil
	}
	return d.opts.Registry.Snapshot(env.host()).Capabilities
}

func (d *Daemon) capabilities(state *connState) (json.RawMessage, *protocol.Error) {
	out, err := json.Marshal(registry.Snapshot{Capabilities: d.snapshot(state.env)})
	if err != nil {
		return nil, &protocol.Error{Code: protocol.CodeInternalError, Message: err.Error()}
	}
	return out, nil
}

func supports(versions []int, want int) bool {
	for _, v := range versions {
		if v == want {
			return true
		}
	}
	return false
}

func (d *Daemon) logf(format string, args ...any) {
	fmt.Fprintf(d.log, "core: "+format+"\n", args...)
}
