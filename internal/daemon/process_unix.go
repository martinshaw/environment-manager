//go:build unix

package daemon

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// Alive reports whether pid is a running process.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}

func stopPID(pid int) {
	_ = syscall.Kill(pid, syscall.SIGTERM)
}

func configureCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// WithSignals cancels when the process is interrupted or asked to terminate.
func WithSignals() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}
