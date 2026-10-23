//go:build windows

package notify

import (
	"os/exec"
	"strings"
)

// Send shows a balloon notification through PowerShell.
func Send(title, body string) error {
	script := `
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$n = New-Object System.Windows.Forms.NotifyIcon
$n.Icon = [System.Drawing.SystemIcons]::Information
$n.Visible = $true
$n.BalloonTipTitle = ` + psQuote(title) + `
$n.BalloonTipText = ` + psQuote(body) + `
$n.ShowBalloonTip(8000)
Start-Sleep -Seconds 8
$n.Dispose()
`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	return cmd.Start()
}

func psQuote(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "'", "''")
	return "'" + s + "'"
}
