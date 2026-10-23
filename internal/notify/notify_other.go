//go:build !darwin && !linux && !windows

package notify

import "errors"

// Send is unavailable on this operating system.
func Send(title, body string) error {
	return errors.New("desktop notifications are not supported on this OS")
}
