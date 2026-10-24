//go:build darwin

package notify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppleString(t *testing.T) {
	got := appleString("line1\nline2\\end \"quoted\"")
	want := `"line1 line2\\end \"quoted\""`
	if got != want {
		t.Fatalf("%q", got)
	}
}

func TestSendUsesOsascript(t *testing.T) {
	bin := t.TempDir()
	record := filepath.Join(t.TempDir(), "args")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + record + "'\n"
	if err := os.WriteFile(filepath.Join(bin, "osascript"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if err := Send("A \"title\"", "line1\nline2\\end"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	want := `display notification "line1 line2\\end" with title "A \"title\""`
	if !strings.Contains(got, "-e") || !strings.Contains(got, want) {
		t.Fatalf("%q", got)
	}
}
