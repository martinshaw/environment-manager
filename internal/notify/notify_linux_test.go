//go:build linux

package notify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSendReportsMissingNotifySend(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := Send("Title", "Body"); err == nil {
		t.Fatal("expected notify-send to be missing")
	}
}

func TestSendStartsNotifySend(t *testing.T) {
	bin := t.TempDir()
	record := filepath.Join(bin, "record")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> '" + record + "'\n"
	if err := os.WriteFile(filepath.Join(bin, "notify-send"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if err := Send("Title", "Body line"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	var got string
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(record)
		if err == nil {
			got = string(raw)
		}
		if strings.Count(got, "Title") >= 2 && strings.Count(got, "Body line") >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if strings.Count(got, "--app-name=envm") < 2 || strings.Count(got, "Title") < 2 {
		t.Fatalf("%q", got)
	}
}
