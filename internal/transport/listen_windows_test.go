//go:build windows

package transport

import "testing"

// This runs on the Windows CI lane. It is written to fail the moment someone
// implements pipes without revisiting the test, so the gap cannot quietly turn into
// a working-looking build.
func TestWindowsTransportReportsThatPipesAreNotImplemented(t *testing.T) {
	ep, err := Resolve("windows", `C:\Users\u`, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if _, err := Listen(ep); err == nil {
		t.Fatal("Listen succeeded on Windows, want an error until pipes are implemented")
	}
	if _, err := Dial(ep); err == nil {
		t.Fatal("Dial succeeded on Windows, want an error until pipes are implemented")
	}
}
