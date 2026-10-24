package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"martinshaw.co/environment-manager/internal/notify"
	"martinshaw.co/environment-manager/internal/pending"
	"martinshaw.co/environment-manager/internal/watch"
)

// Running reports whether a daemon process recorded in dir is still alive.
func Running(dir string) bool {
	pid, err := readPID(dir)
	if err != nil {
		return false
	}
	return Alive(pid)
}

// Run is the foreground watcher. It listens on localhost for UI clients and
// never receives the vault passphrase.
func Run(ctx context.Context, dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if Running(dir) {
		return errors.New("daemon is already running")
	}
	if err := writePID(dir); err != nil {
		return err
	}
	defer clearPID(dir)

	token, err := newToken()
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "daemon.token"), []byte(token), 0o600); err != nil {
		return err
	}
	defer os.Remove(filepath.Join(dir, "daemon.token"))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	if err := os.WriteFile(filepath.Join(dir, "daemon.port"), []byte(strconv.Itoa(port)), 0o600); err != nil {
		return err
	}
	defer os.Remove(filepath.Join(dir, "daemon.port"))

	h := newHub()
	go serve(ctx, ln, token, h)
	err = watch.Loop(ctx, dir, watch.Hooks{
		Notify: func(title, body string) {
			if err := notify.Send(title, body); err != nil {
				appendLog(dir, "notify: "+err.Error())
			}
		},
		Updated: func(events []pending.Event) {
			h.broadcast(events)
		},
	})
	h.close()
	return err
}

// Status prints whether the daemon is running.
func Status(dir string) string {
	if Running(dir) {
		pid, _ := readPID(dir)
		return fmt.Sprintf("running (pid %d)", pid)
	}
	if ServiceInstalled() {
		return "stopped (login service installed)"
	}
	return "stopped"
}

// Install registers and starts the login service.
func InstallService(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := Install(dir); err != nil {
		return err
	}
	return Start(dir)
}

// Start starts the login service, or a background process when none is installed.
func Start(dir string) error {
	if ServiceInstalled() {
		return StartService()
	}
	if Running(dir) {
		return nil
	}
	return startBackground(dir)
}

// Stop stops the login service and any recorded daemon process.
func Stop(dir string) error {
	_ = StopService()
	pid, err := readPID(dir)
	if err != nil {
		return nil
	}
	if Alive(pid) {
		stopPID(pid)
	}
	clearPID(dir)
	return nil
}

// UninstallService stops the daemon and removes the login service.
func UninstallService(dir string) error {
	if err := Stop(dir); err != nil {
		return err
	}
	return Uninstall()
}

func startBackground(dir string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	logPath := filepath.Join(dir, "daemon.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "daemon", "run")
	cmd.Stdout = f
	cmd.Stderr = f
	configureCmd(cmd)
	if err := cmd.Start(); err != nil {
		f.Close()
		return err
	}
	_ = f.Close()
	return nil
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func pidPath(dir string) string { return filepath.Join(dir, "daemon.pid") }

func writePID(dir string) error {
	return os.WriteFile(pidPath(dir), []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600)
}

func readPID(dir string) (int, error) {
	b, err := os.ReadFile(pidPath(dir))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}

func clearPID(dir string) { _ = os.Remove(pidPath(dir)) }

func appendLog(dir, msg string) {
	f, err := os.OpenFile(filepath.Join(dir, "daemon.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), msg)
}

type hub struct {
	mu   sync.Mutex
	subs map[chan []pending.Event]struct{}
}

func newHub() *hub {
	return &hub{subs: map[chan []pending.Event]struct{}{}}
}

func (h *hub) add(ch chan []pending.Event) {
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
}

func (h *hub) remove(ch chan []pending.Event) {
	h.mu.Lock()
	delete(h.subs, ch)
	h.mu.Unlock()
}

func (h *hub) broadcast(events []pending.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- events:
		default:
		}
	}
}

func (h *hub) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		close(ch)
		delete(h.subs, ch)
	}
}

type envelope struct {
	Op     string          `json:"op"`
	Token  string          `json:"token,omitempty"`
	Events []pending.Event `json:"events,omitempty"`
}

func serve(ctx context.Context, ln net.Listener, token string, h *hub) {
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go handleConn(ctx, conn, token, h)
	}
}

func handleConn(ctx context.Context, conn net.Conn, token string, h *hub) {
	defer conn.Close()
	dec := json.NewDecoder(conn)
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var auth envelope
	if err := dec.Decode(&auth); err != nil {
		return
	}
	_ = conn.SetReadDeadline(time.Time{})
	if auth.Op != "auth" || !tokenOK(auth.Token, token) {
		return
	}
	ch := make(chan []pending.Event, 1)
	h.add(ch)
	defer h.remove(ch)
	enc := json.NewEncoder(conn)
	for {
		select {
		case <-ctx.Done():
			return
		case events, ok := <-ch:
			if !ok {
				return
			}
			if err := enc.Encode(envelope{Op: "pending", Events: events}); err != nil {
				return
			}
		}
	}
}

func tokenOK(got, want string) bool {
	if len(got) != len(want) || want == "" {
		return false
	}
	// Lengths match, so a straightforward compare is fine for a local socket token.
	diff := 0
	for i := 0; i < len(got); i++ {
		diff |= int(got[i] ^ want[i])
	}
	return diff == 0
}

// NextEvents waits for a daemon broadcast or, when the daemon is not
// reachable, returns the pending file after a short pause.
func NextEvents(ctx context.Context, dir string) []pending.Event {
	conn, token, err := dial(dir)
	if err != nil {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(200 * time.Millisecond):
		}
		ev, _ := pending.Load(dir)
		return ev
	}
	defer conn.Close()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-done:
		}
	}()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if err := json.NewEncoder(conn).Encode(envelope{Op: "auth", Token: token}); err != nil {
		ev, _ := pending.Load(dir)
		return ev
	}
	var msg envelope
	_ = json.NewDecoder(conn).Decode(&msg)
	ev, _ := pending.Load(dir)
	return ev
}

func dial(dir string) (net.Conn, string, error) {
	portB, err := os.ReadFile(filepath.Join(dir, "daemon.port"))
	if err != nil {
		return nil, "", err
	}
	tokenB, err := os.ReadFile(filepath.Join(dir, "daemon.token"))
	if err != nil {
		return nil, "", err
	}
	port := strings.TrimSpace(string(portB))
	if _, err := strconv.Atoi(port); err != nil {
		return nil, "", err
	}
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, time.Second)
	if err != nil {
		return nil, "", err
	}
	return conn, strings.TrimSpace(string(tokenB)), nil
}
