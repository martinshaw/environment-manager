package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestHasPrefixPath(t *testing.T) {
	root := filepath.Join("proj", "root")
	child := filepath.Join(root, "app")
	sibling := filepath.Join("proj", "root-extra", "app")
	if !hasPrefixPath(child, root) {
		t.Fatalf("%s should be inside %s", child, root)
	}
	if hasPrefixPath(sibling, root) || hasPrefixPath(root, root) || hasPrefixPath(child, "") {
		t.Fatal("prefix matched a path that is not a child")
	}
}

func TestVersionHelpAndRejectedArgs(t *testing.T) {
	out, err := execute(t, "version")
	if err != nil {
		t.Fatal(err, out)
	}
	if strings.TrimSpace(out) != "9.9.9" {
		t.Fatalf("%q", out)
	}
	out, err = execute(t, "--version")
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "9.9.9") {
		t.Fatalf("%q", out)
	}
	out, err = execute(t, "--help")
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "upgrade") || !strings.Contains(out, "pwd") {
		t.Fatalf("%q", out)
	}
	if _, err := execute(t, "use"); err == nil {
		t.Fatal("use without a name succeeded")
	}
	if _, err := execute(t, "new", "one", "two"); err == nil {
		t.Fatal("new with two names succeeded")
	}
	if _, err := execute(t, "nope"); err == nil {
		t.Fatal("unknown command succeeded")
	}
}

func TestCommandsRequireATerminal(t *testing.T) {
	t.Setenv("ENVM_HOME", t.TempDir())
	for _, args := range [][]string{
		{"use", "Local"},
		{"update"},
		{"new", "Local"},
		{"revert"},
		{"diff"},
	} {
		out, err := execute(t, args...)
		if err == nil || !strings.Contains(out, "terminal is required") {
			t.Fatalf("%v: %v %q", args, err, out)
		}
	}
}

func TestRoots(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ENVM_HOME", home)
	out, err := execute(t, "roots", "list")
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "No roots.") {
		t.Fatalf("%q", out)
	}

	root := t.TempDir()
	project := filepath.Join(root, "Todo App")
	nested := filepath.Join(project, "packages", "web")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".env"), []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, ".env"), []byte("B=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err = execute(t, "roots", "add", file)
	if err == nil || !strings.Contains(out, "not a directory") {
		t.Fatalf("%v %q", err, out)
	}
	out, err = execute(t, "roots", "add", root)
	if err != nil || !strings.Contains(out, "Added root.") {
		t.Fatalf("%v %q", err, out)
	}
	out, err = execute(t, "roots", "add", root)
	if err == nil || !strings.Contains(out, "already a root") {
		t.Fatalf("%v %q", err, out)
	}
	out, err = execute(t, "roots", "list")
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "(2 projects)") {
		t.Fatalf("%q", out)
	}
	out, err = execute(t, "roots", "remove", root)
	if err != nil {
		t.Fatal(err, out)
	}
	out, err = execute(t, "roots", "list")
	if err != nil || !strings.Contains(out, "No roots.") {
		t.Fatalf("%v %q", err, out)
	}
	out, err = execute(t, "roots", "remove", root)
	if err == nil || !strings.Contains(out, "root not found") {
		t.Fatalf("%v %q", err, out)
	}
}

func TestDriftWithNothingPending(t *testing.T) {
	t.Setenv("ENVM_HOME", t.TempDir())
	out, err := execute(t, "drift")
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "No pending changes.") {
		t.Fatalf("%q", out)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err = execute(t, "roots", "add", root); err != nil {
		t.Fatal(err, out)
	}
	out, err = execute(t, "drift")
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "No pending changes.") {
		t.Fatalf("%q", out)
	}
}

func TestDaemonStatus(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ENVM_HOME", home)
	pid := filepath.Join(home, "daemon.pid")
	if err := os.WriteFile(pid, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := execute(t, "daemon", "status")
	if err != nil {
		t.Fatal(err, out)
	}
	want := "running (pid " + strconv.Itoa(os.Getpid()) + ")"
	if !strings.Contains(out, want) {
		t.Fatalf("%q", out)
	}
}

func TestUpgradeRejectsExtraArgs(t *testing.T) {
	cmd := upgradeCmd("9.9.9")
	if cmd.Flags().Lookup("check") == nil {
		t.Fatal("missing --check")
	}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"extra"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("upgrade accepted an extra argument")
	}
}

func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newRoot("9.9.9")
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr := os.Stdout, os.Stderr
	os.Stdout = outW
	os.Stderr = errW
	defer func() {
		os.Stdout = stdout
		os.Stderr = stderr
	}()
	cmd.SetOut(outW)
	cmd.SetErr(errW)
	cmd.SetArgs(args)
	runErr := cmd.Execute()
	outW.Close()
	errW.Close()
	var out, errOut bytes.Buffer
	if _, err := io.Copy(&out, outR); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(&errOut, errR); err != nil {
		t.Fatal(err)
	}
	return out.String() + errOut.String(), runErr
}
