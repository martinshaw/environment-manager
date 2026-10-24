package daemon

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"martinshaw.co/environment-manager/internal/pending"
)

func TestPIDAndStatus(t *testing.T) {
	dir := t.TempDir()
	if Running(dir) {
		t.Fatal("missing pid file reported running")
	}
	if _, err := readPID(dir); err == nil {
		t.Fatal("expected missing pid file to fail")
	}
	if err := os.WriteFile(pidPath(dir), []byte("nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readPID(dir); err == nil {
		t.Fatal("expected a non-numeric pid to fail")
	}
	if Running(dir) {
		t.Fatal("invalid pid reported running")
	}

	if err := writePID(dir); err != nil {
		t.Fatal(err)
	}
	pid, err := readPID(dir)
	if err != nil {
		t.Fatal(err)
	}
	if pid != os.Getpid() {
		t.Fatalf("pid %d", pid)
	}
	if !Running(dir) {
		t.Fatal("current process was not running")
	}
	got := Status(dir)
	want := "running (pid " + strconv.Itoa(os.Getpid()) + ")"
	if got != want {
		t.Fatalf("status %q", got)
	}

	clearPID(dir)
	if Running(dir) {
		t.Fatal("pid file remained")
	}
	stopped := Status(dir)
	if ServiceInstalled() {
		if stopped != "stopped (login service installed)" {
			t.Fatalf("status %q", stopped)
		}
	} else if stopped != "stopped" {
		t.Fatalf("status %q", stopped)
	}
}

func TestAlive(t *testing.T) {
	if Alive(0) || Alive(-1) {
		t.Fatal("non-positive pid was alive")
	}
	if !Alive(os.Getpid()) {
		t.Fatal("current process was not alive")
	}
	dead := os.Getpid()
	for i := 0; i < 1000; i++ {
		dead++
		if !Alive(dead) {
			return
		}
	}
	t.Fatal("could not find a pid that is not running")
}

func TestRunRejectsSecondDaemon(t *testing.T) {
	dir := t.TempDir()
	if err := writePID(dir); err != nil {
		t.Fatal(err)
	}
	err := Run(context.Background(), dir)
	if err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("err %v", err)
	}
	pid, err := readPID(dir)
	if err != nil {
		t.Fatal(err)
	}
	if pid != os.Getpid() {
		t.Fatal("second start replaced the pid file")
	}
}

func TestRunStopsAndClearsFiles(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, dir)
	}()
	portFile := filepath.Join(dir, "daemon.port")
	tokenFile := filepath.Join(dir, "daemon.token")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(portFile); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(portFile); err != nil {
		cancel()
		t.Fatalf("daemon did not listen: %v", <-errCh)
	}
	token, err := os.ReadFile(tokenFile)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if len(strings.TrimSpace(string(token))) != 64 {
		cancel()
		t.Fatalf("token %q", token)
	}
	if !Running(dir) {
		cancel()
		t.Fatal("pid was not recorded")
	}
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("daemon did not stop")
	}
	for _, name := range []string{"daemon.port", "daemon.token", "daemon.pid"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s remained: %v", name, err)
		}
	}
}

func TestTokenAndLog(t *testing.T) {
	a, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 64 || a == b {
		t.Fatalf("tokens %q %q", a, b)
	}
	if !tokenOK(a, a) || tokenOK(a, b) || tokenOK(a, "") || tokenOK("abc", "abcd") {
		t.Fatal("token compare mismatch")
	}
	dir := t.TempDir()
	appendLog(dir, "notify: failed")
	raw, err := os.ReadFile(filepath.Join(dir, "daemon.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "notify: failed") {
		t.Fatalf("%q", raw)
	}
}

func TestHubBroadcastAndClose(t *testing.T) {
	h := newHub()
	ch := make(chan []pending.Event, 1)
	full := make(chan []pending.Event)
	h.add(ch)
	h.add(full)
	h.broadcast([]pending.Event{{ID: "1", Message: "one"}})
	h.broadcast([]pending.Event{{ID: "2", Message: "two"}})
	got := <-ch
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("%#v", got)
	}
	select {
	case extra := <-ch:
		t.Fatalf("full buffer should drop the second broadcast, got %#v", extra)
	default:
	}
	h.remove(ch)
	h.close()
	select {
	case _, ok := <-ch:
		if !ok {
			t.Fatal("removed subscription was closed")
		}
		t.Fatal("removed subscription received a late event")
	default:
	}
	if _, ok := <-full; ok {
		t.Fatal("close left a subscription open")
	}
}

func TestServeAuthAndNextEvents(t *testing.T) {
	dir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := newHub()
	token := "secret-token"
	go serve(ctx, ln, token, h)

	bad, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(bad).Encode(envelope{Op: "auth", Token: "nope"}); err != nil {
		t.Fatal(err)
	}
	_ = bad.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var ignored envelope
	if err := json.NewDecoder(bad).Decode(&ignored); err == nil {
		t.Fatal("rejected token still received a broadcast")
	}
	bad.Close()

	want := pending.Event{ID: "e1", Kind: pending.KindDrift, ProjectID: "p", Project: "Todo App", Message: ".env differs"}
	if err := pending.Save(dir, []pending.Event{want}); err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := os.WriteFile(filepath.Join(dir, "daemon.port"), []byte(strconv.Itoa(port)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "daemon.token"), []byte(token), 0o600); err != nil {
		t.Fatal(err)
	}
	go func() {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			h.broadcast([]pending.Event{want})
			time.Sleep(10 * time.Millisecond)
		}
	}()
	got := NextEvents(context.Background(), dir)
	if len(got) != 1 || got[0].ID != want.ID || got[0].Message != want.Message {
		t.Fatalf("%#v", got)
	}
}

func TestNextEventsWithoutDaemon(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if got := NextEvents(ctx, dir); got != nil {
		t.Fatalf("cancelled: %#v", got)
	}
	if time.Since(start) > 150*time.Millisecond {
		t.Fatal("cancelled wait did not return immediately")
	}

	want := pending.Event{ID: "file", Kind: pending.KindGitignore, ProjectID: "p", Message: "not ignored"}
	if err := pending.Save(dir, []pending.Event{want}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "daemon.port"), []byte("not-a-port"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "daemon.token"), []byte("token"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := NextEvents(context.Background(), dir)
	if len(got) != 1 || got[0].ID != "file" {
		t.Fatalf("%#v", got)
	}
}
