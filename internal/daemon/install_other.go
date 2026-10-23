//go:build unix && !darwin && !linux

package daemon

import "errors"

func ServiceInstalled() bool { return false }

func Install(dir string) error {
	return errors.New("installing a login service is not supported on this OS; run envm daemon run")
}

func Uninstall() error { return nil }

func StartService() error {
	return errors.New("no login service is installed")
}

func StopService() error { return nil }
