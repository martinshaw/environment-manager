//go:build windows

package notify

import "testing"

func TestPSQuote(t *testing.T) {
	got := psQuote("a'b\nc")
	want := "'a''b c'"
	if got != want {
		t.Fatalf("%q", got)
	}
}

func TestSendWithoutPowerShell(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := Send("Title", "Body"); err == nil {
		t.Fatal("expected powershell to be missing")
	}
}
