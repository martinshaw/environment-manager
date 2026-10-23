//go:build darwin

package notify

import (
	"os/exec"
	"strings"
)

// Send displays a macOS notification. Failure to notify does not affect the watcher.
func Send(title, body string) error {
	script := "display notification " + appleString(body) + " with title " + appleString(title)
	return exec.Command("osascript", "-e", script).Run()
}

func appleString(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
