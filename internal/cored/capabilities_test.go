package cored

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/soppylzz/soppy-utils/internal/protocol"
	"github.com/soppylzz/soppy-utils/internal/registry"
)

// probeRegistry holds one capability that depends on a command being installed, which
// is the shape most real capabilities have.
func probeRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	r := registry.New()
	if err := r.Register(registry.Capability{
		ID:    "welcome",
		Title: "Welcome banner",
		Judge: func(h registry.Host) registry.Assessment {
			if _, err := h.LookPath("probe"); err != nil {
				return registry.Assessment{
					State: registry.Degraded, Reason: registry.MissingDependency,
					Missing: []string{"probe"}, Detail: "the banner will be plain text",
				}
			}
			return registry.Assessment{State: registry.Available}
		},
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return r
}

func initParams(t *testing.T, extra map[string]any) string {
	t.Helper()
	client := map[string]any{"protocol_versions": []int{protocol.Version}}
	if extra != nil {
		client["env"] = extra
	}
	body, err := json.Marshal(map[string]any{"client": client})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(body)
}

func TestTheHandshakeCarriesTheJudgedSnapshot(t *testing.T) {
	h := newHarness(t, Options{Registry: probeRegistry(t)})
	c := h.dial()

	reply := c.call("initialize", initParams(t, map[string]any{"os": "linux", "path": ""}))
	if reply.Error != nil {
		t.Fatalf("initialize failed: %+v", reply.Error)
	}

	var got struct {
		Capabilities []registry.Report `json:"capabilities"`
	}
	if err := json.Unmarshal(reply.Result, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(got.Capabilities) != 1 {
		t.Fatalf("handshake carried %d capabilities, want 1", len(got.Capabilities))
	}
	if got.Capabilities[0].ID != "welcome" {
		t.Errorf("capability = %q, want welcome", got.Capabilities[0].ID)
	}
	if got.Capabilities[0].State != registry.Degraded {
		t.Errorf("state = %q, want degraded: the probe command is not on this path",
			got.Capabilities[0].State)
	}
}

func TestTwoClientsWithDifferentPathsGetDifferentAnswers(t *testing.T) {
	// The whole reason evaluation is per session. One daemon, one registry, two
	// clients: the answer has to follow the client, not the process that happens to be
	// serving both of them.
	installed := t.TempDir()
	if err := os.WriteFile(filepath.Join(installed, "probe"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write probe: %v", err)
	}

	h := newHarness(t, Options{Registry: probeRegistry(t)})

	withProbe := h.dial()
	withProbe.call("initialize", initParams(t, map[string]any{"os": "linux", "path": installed}))
	withoutProbe := h.dial()
	withoutProbe.call("initialize", initParams(t, map[string]any{"os": "linux", "path": t.TempDir()}))

	first := capabilitiesList(t, withProbe)
	second := capabilitiesList(t, withoutProbe)

	if first[0].State != registry.Available {
		t.Errorf("the client with the command on its path saw %q, want available", first[0].State)
	}
	if second[0].State != registry.Degraded {
		t.Errorf("the client without it saw %q, want degraded", second[0].State)
	}
}

func capabilitiesList(t *testing.T, c *client) []registry.Report {
	t.Helper()
	reply := c.call("capabilities/list", `{}`)
	if reply.Error != nil {
		t.Fatalf("capabilities/list failed: %+v", reply.Error)
	}
	var got struct {
		Capabilities []registry.Report `json:"capabilities"`
	}
	if err := json.Unmarshal(reply.Result, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return got.Capabilities
}

func TestCapabilitiesListRescansOnRequest(t *testing.T) {
	// A daemon lives for hours, so a snapshot taken at the handshake goes stale: the
	// user installs the command, and the client has to be able to ask again.
	installed := t.TempDir()
	h := newHarness(t, Options{Registry: probeRegistry(t)})

	c := h.dial()
	c.call("initialize", initParams(t, map[string]any{"os": "linux", "path": installed}))
	if got := capabilitiesList(t, c)[0].State; got != registry.Degraded {
		t.Fatalf("before installing: state = %q, want degraded", got)
	}

	if err := os.WriteFile(filepath.Join(installed, "probe"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write probe: %v", err)
	}

	if got := capabilitiesList(t, c)[0].State; got != registry.Available {
		t.Errorf("after installing: state = %q, want available", got)
	}
}

func TestInitializeRefusesAClientThatDoesNotSayWhatItIsRunningOn(t *testing.T) {
	// Platform is an input to judging, so a client that withholds it cannot be
	// evaluated, and guessing the daemon's own platform is how a Git Bash client gets
	// answers about macOS.
	h := newHarness(t, Options{Registry: probeRegistry(t)})
	reply := h.dial().call("initialize", initParams(t, nil))

	if reply.Error == nil {
		t.Fatal("initialize accepted a client with no environment, want an error")
	}
	if reply.Error.Code != protocol.CodeInvalidParams {
		t.Errorf("code = %d, want %d", reply.Error.Code, protocol.CodeInvalidParams)
	}
}

func TestASessionMethodIsRefusedBeforeInitialize(t *testing.T) {
	// Judging needs the environment, and the environment arrives with the handshake.
	h := newHarness(t, Options{Registry: probeRegistry(t)})
	c := h.dial()

	for _, method := range []string{"capabilities/list", "command/run"} {
		reply := c.call(method, `{}`)
		if reply.Error == nil {
			t.Errorf("%s succeeded before initialize, want an error", method)
			continue
		}
		if reply.Error.Code == protocol.CodeMethodNotFound {
			t.Errorf("%s is not implemented at all", method)
		}
	}
}
