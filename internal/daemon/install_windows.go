//go:build windows

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const taskName = "envm"

func ServiceInstalled() bool {
	err := exec.Command("schtasks", "/Query", "/TN", taskName).Run()
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
	tr := fmt.Sprintf(`"%s" daemon run`, exe)
	out, err := exec.Command("schtasks", "/Create", "/F", "/TN", taskName, "/SC", "ONLOGON", "/RL", "LIMITED", "/TR", tr).CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks: %s: %s", err, out)
	}
	return nil
}

func Uninstall() error {
	_ = StopService()
	out, err := exec.Command("schtasks", "/Delete", "/F", "/TN", taskName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks delete: %s: %s", err, out)
	}
	return nil
}

func StartService() error {
	out, err := exec.Command("schtasks", "/Run", "/TN", taskName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks run: %s: %s", err, out)
	}
	return nil
}

func StopService() error {
	_ = exec.Command("schtasks", "/End", "/TN", taskName).Run()
	return nil
}
