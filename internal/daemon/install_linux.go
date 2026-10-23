//go:build linux

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func unitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user", "envm.service"), nil
}

func ServiceInstalled() bool {
	path, err := unitPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func Install(dir string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	path, err := unitPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body := fmt.Sprintf(`[Unit]
Description=envm project .env watcher
After=default.target

[Service]
ExecStart=%s daemon run
Restart=on-failure
RestartSec=2

[Install]
WantedBy=default.target
`, exe)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return err
	}
	if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %s: %s", err, out)
	}
	return nil
}

func Uninstall() error {
	_ = StopService()
	path, err := unitPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

func StartService() error {
	out, err := exec.Command("systemctl", "--user", "enable", "--now", "envm.service").CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl enable --now: %s: %s", err, out)
	}
	return nil
}

func StopService() error {
	_ = exec.Command("systemctl", "--user", "disable", "--now", "envm.service").Run()
	return nil
}
