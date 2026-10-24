//go:build !darwin && !linux && !windows

package notify

import "testing"

func TestSendUnsupported(t *testing.T) {
	err := Send("Title", "Body")
	if err == nil {
		t.Fatal("expected an unsupported-OS error")
	}
}
