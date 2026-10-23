//go:build linux

package notify

import (
	"os"
	"os/exec"
	"strings"
)

// Send displays a desktop notification. When notify-send supports actions, the
// default action starts `envm drift`.
func Send(title, body string) error {
	if _, err := exec.LookPath("notify-send"); err != nil {
		return err
	}
	go func() {
		cmd := exec.Command("notify-send", "--app-name=envm", "--wait", "--action=default=Review", title, body)
		out, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(out)) == "default" {
			launchDrift()
			return
		}
		_ = exec.Command("notify-send", "--app-name=envm", title, body).Run()
	}()
	return nil
}

func launchDrift() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	_ = exec.Command(exe, "drift").Start()
}
