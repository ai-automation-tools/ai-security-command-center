//go:build unix

package main

import (
	"io"
	"syscall"
	"testing"
	"time"
)

// `docker stop` sends the terminate signal, not an interrupt. run must treat it as a stop and return 0;
// before this it didn't listen for it, so the process was killed with requests in flight.
func TestTerminateSignalStopsCleanly(t *testing.T) {
	s, _ := sandbox(t)
	_, exited := started(t, func(stderr io.Writer) int {
		return run([]string{"--listen", "127.0.0.1:0", "--registry", s.registry, "--policies", s.policies}, io.Discard, stderr)
	})
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-exited:
		if code != 0 {
			t.Fatalf("exit %d on the terminate signal", code)
		}
	case <-time.After(drainFor + time.Second):
		t.Fatal("the service ignored the terminate signal")
	}
}
