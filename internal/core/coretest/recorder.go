// Package coretest provides a Responder that answers the way a test tells it to.
//
// A command asks through a Responder, so a test that wants to drive one needs something
// on the other end. This is that, in one place rather than once per util: three
// near-identical doubles is where the third copy stops paying for itself.
package coretest

import (
	"context"
	"encoding/json"

	"strings"

	"github.com/soppylzz/soppy-utils/internal/action"
	"github.com/soppylzz/soppy-utils/internal/core"
	"github.com/soppylzz/soppy-utils/internal/form"
)

// Recorder stands in for a client. It remembers what a command reported and answers
// whatever the test told it to.
//
// The zero value is a client with no terminal: it cannot ask, it declines work, and it
// says no to every question. That is the state a scripted run is in, and it is the safe
// reading of every field nobody set.
type Recorder struct {
	// Askable is what CanAsk reports.
	Askable bool

	// Answers is what Ask returns. Nil makes Ask fail, which is what a test wants when
	// it believes a form should not be shown at all.
	Answers map[string]any

	// Confirms is consumed one per question. Running out means no.
	Confirms []bool

	// ActionOutcome, when set, is what AskActions answers. Left nil it declines, which is
	// what a client with nobody to ask answers.
	ActionOutcome *action.Outcome

	// What the command did, recorded.
	Asked     []form.Spec
	Questions []string
	Notices   []string
	Actions   []action.Request
	Finished  json.RawMessage
}

func (r *Recorder) CanAsk() bool { return r.Askable }

func (r *Recorder) Notify(message string) error {
	r.Notices = append(r.Notices, message)
	return nil
}

func (r *Recorder) Ask(_ context.Context, spec form.Spec) (map[string]any, error) {
	r.Asked = append(r.Asked, spec)
	if r.Answers == nil {
		return nil, ErrNoAnswers
	}
	return r.Answers, nil
}

func (r *Recorder) AskActions(_ context.Context, req action.Request) (action.Outcome, error) {
	r.Actions = append(r.Actions, req)
	if r.ActionOutcome != nil {
		return *r.ActionOutcome, nil
	}
	return action.Outcome{Declined: true}, nil
}

func (r *Recorder) Confirm(_ context.Context, question string) (bool, error) {
	r.Questions = append(r.Questions, question)
	if len(r.Confirms) == 0 {
		return false, nil
	}
	answer := r.Confirms[0]
	r.Confirms = r.Confirms[1:]
	return answer, nil
}

// Finish is the Responder method. It records into Finished, because a field and a
// method cannot share a name.
func (r *Recorder) Finish(result json.RawMessage) error {
	r.Finished = result
	return nil
}

// Reporter is the part of a testing.T this package needs, so that a caller's failure is
// reported against the caller.
type Reporter interface {
	Errorf(format string, args ...any)
}

// Answer reads back what a command handed to Finish.
func (r *Recorder) Answer(t Reporter) core.Result {
	var got core.Result
	if err := json.Unmarshal(r.Finished, &got); err != nil {
		t.Errorf("the command did not finish with a result: %v", err)
	}
	return got
}

// NoticesContain reports whether any notice mentions want.
func (r *Recorder) NoticesContain(want string) bool {
	for _, n := range r.Notices {
		if strings.Contains(n, want) {
			return true
		}
	}
	return false
}

// Session is a session on the machine running the test. A test that cares which platform
// a command is judged as builds its own.
func Session(home, goos string) core.Session {
	return core.Session{Home: home, GOOS: goos}
}

// ErrNoAnswers is what Ask returns when a test gave it nothing to say.
var ErrNoAnswers = errNoAnswers{}

type errNoAnswers struct{}

func (errNoAnswers) Error() string { return "this test's client has nothing to answer with" }

// LastAction is the most recent offer of work, if any was made.
func (r *Recorder) LastAction() (action.Request, bool) {
	if len(r.Actions) == 0 {
		return action.Request{}, false
	}
	return r.Actions[len(r.Actions)-1], true
}

// LastAsked is the most recent form shown, if any was.
func (r *Recorder) LastAsked() (form.Spec, bool) {
	if len(r.Asked) == 0 {
		return form.Spec{}, false
	}
	return r.Asked[len(r.Asked)-1], true
}
