//go:build windows

package transport

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// This runs on the Windows CI lane, which is the only place the pipe implementation is
// executed. It replaces a test that asserted the opposite — that pipes were not
// implemented — so that the gap could not quietly become a working-looking build. It is
// still the case that a broken pipe would look fine on a machine that never runs this.
func TestAPipeCarriesBytesBothWays(t *testing.T) {
	ep, err := Resolve("windows", t.TempDir(), "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	ln, err := Listen(ep)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 5)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return
		}
		_, _ = conn.Write([]byte("pong!"))
	}()

	conn, err := Dial(ep)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("ping!")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := make([]byte, 5)
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != "pong!" {
		t.Errorf("read %q, want %q", got, "pong!")
	}
}

func TestAListenerRefusesToTakeASecondOne(t *testing.T) {
	// The endpoint is the daemon's; a second daemon on the same pipe name would be two
	// answers to one question.
	ep, err := Resolve("windows", t.TempDir(), "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	ln, err := Listen(ep)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	if _, err := Listen(ep); !errors.Is(err, ErrInUse) {
		t.Errorf("a second Listen returned %v, want ErrInUse", err)
	}
}

func TestAReadWithADeadlineThatPasses(t *testing.T) {
	// A read with nothing coming has to end, or the handshake's budget has nothing to
	// enforce it with. This is the part of the implementation that is not a straight
	// translation of a socket.
	ep, err := Resolve("windows", t.TempDir(), "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	ln, err := Listen(ep)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			accepted <- conn
		}
	}()

	conn, err := Dial(ep)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	select {
	case server := <-accepted:
		defer server.Close()
	case <-time.After(5 * time.Second):
		t.Fatal("Accept never returned")
	}

	if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	start := time.Now()
	_, err = conn.Read(make([]byte, 1))
	if err == nil {
		t.Fatal("a read with a passed deadline returned data")
	}
	var timeout interface{ Timeout() bool }
	if !errors.As(err, &timeout) || !timeout.Timeout() {
		t.Fatalf("read returned %v, want a timeout", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("the read took %s to time out", elapsed)
	}
}

func TestAClosedPipeEndsARead(t *testing.T) {
	ep, err := Resolve("windows", t.TempDir(), "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	ln, err := Listen(ep)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err == nil {
			conn.Close()
		}
	}()

	conn, err := Dial(ep)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Errorf("reading a closed pipe returned %v, want io.EOF", err)
	}
}
