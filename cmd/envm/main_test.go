package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestMainVersionAndUnknown(t *testing.T) {
	if arg := os.Getenv("ENVM_MAIN_TEST"); arg != "" {
		os.Args = []string{"envm", arg}
		main()
		os.Exit(0)
	}

	out := runMain(t, "version")
	if !strings.Contains(out, version) || version != "1.0.1" {
		t.Fatalf("version %q output %q", version, out)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestMainVersionAndUnknown$")
	cmd.Env = append(os.Environ(), "ENVM_MAIN_TEST=not-a-command")
	raw, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("unknown command exited 0: %s", raw)
	}
	if !strings.Contains(string(raw), "unknown command") {
		t.Fatalf("%s", raw)
	}
}

func runMain(t *testing.T, arg string) string {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestMainVersionAndUnknown$")
	cmd.Env = append(os.Environ(), "ENVM_MAIN_TEST="+arg)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, raw)
	}
	return string(raw)
}
