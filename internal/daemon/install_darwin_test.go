//go:build darwin

package daemon

import (
	"os"
	"testing"
)

func TestXMLEscape(t *testing.T) {
	got := xmlEscape(`a&b<c>"`)
	want := "a&amp;b&lt;c&gt;&quot;"
	if got != want {
		t.Fatalf("%q", got)
	}
}

func TestExecutableResolves(t *testing.T) {
	exe, err := executable()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(exe)
	if err != nil {
		t.Fatal(err)
	}
	if info.IsDir() {
		t.Fatalf("executable is a directory: %s", exe)
	}
}
