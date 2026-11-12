package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// started runs the service on a port the kernel picks and returns its address and a channel that
// carries its exit code. start is run or serve with everything but stderr filled in.
func started(t *testing.T, start func(stderr io.Writer) int) (addr string, exited <-chan int) {
	t.Helper()
	pr, pw := io.Pipe()
	done := make(chan int, 1)
	go func() {
		code := start(pw)
		pw.Close()
		done <- code
	}()
	lines := bufio.NewReader(pr)
	for {
		line, err := lines.ReadString('\n')
		if err != nil {
			t.Fatalf("the service stopped before it listened: %v", err)
		}
		if after, ok := strings.CutPrefix(line, "ascc-pdp: listening on "); ok {
			go io.Copy(io.Discard, lines)
			return strings.TrimSpace(after), done
		}
	}
}

// A stop lets the request already in flight finish, and only then does the service return. Before
// this the process ended the moment it was told to stop, and a caller mid-request got a reset: to an
// enforcement point that is an error, so a deny, but it was a deny nobody decided.
func TestStopDrainsRequestsInFlight(t *testing.T) {
	s, _ := sandbox(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	addr, exited := started(t, func(stderr io.Writer) int {
		return serve(ctx, []string{"--listen", "127.0.0.1:0", "--registry", s.registry, "--policies", s.policies}, io.Discard, stderr)
	})

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	reqBody := ask(t, "read-repo-file.json")
	half := len(reqBody) / 2
	fmt.Fprintf(conn, "POST /pdp/v1/decide HTTP/1.1\r\nHost: %s\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s", addr, len(reqBody), reqBody[:half])

	time.Sleep(100 * time.Millisecond) // let the server start reading it
	stop()
	select {
	case code := <-exited:
		t.Fatalf("the service returned %d with a request in flight", code)
	case <-time.After(300 * time.Millisecond):
	}
	// It has stopped listening, so nothing new gets in while it drains.
	if c, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		c.Close()
		t.Error("the service still takes connections after a stop")
	}

	fmt.Fprint(conn, reqBody[half:])
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("the request in flight got no answer: %v", err)
	}
	answer, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(answer), `"decision":"allow"`) {
		t.Fatalf("the request in flight: HTTP %d %s", resp.StatusCode, answer)
	}
	select {
	case code := <-exited:
		if code != 0 {
			t.Fatalf("exit %d after a clean stop", code)
		}
	case <-time.After(drainFor + time.Second):
		t.Fatal("the service didn't return after its last request finished")
	}
}
