package ignore

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPatMatch(t *testing.T) {
	cases := []struct {
		pattern string
		want    bool
	}{
		{".env", true},
		{".env*", true},
		{"*", true},
		{"*.log", false},
		{".env/", false},
		{"foo/.env", false},
		{"/.env", true},
	}
	for _, tc := range cases {
		if got := patMatch(tc.pattern, ".env"); got != tc.want {
			t.Errorf("patMatch(%q) = %v, want %v", tc.pattern, got, tc.want)
		}
	}
}

func TestCheckFile(t *testing.T) {
	dir := t.TempDir()
	st := checkFile(dir)
	if !st.NeedsAttention() {
		t.Fatal(st)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("# keep\n.env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st = checkFile(dir)
	if st.Code != "ignored" {
		t.Fatal(st)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".env\n!.env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st = checkFile(dir)
	if st.Code != "unignored" {
		t.Fatal(st)
	}
}

func TestCheckGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(dir, "global"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(dir, "system"))
	if err := os.WriteFile(filepath.Join(dir, "global"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "system"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "app")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-b", "master")
	st, err := Check(repo)
	if err != nil {
		t.Fatal(err)
	}
	if st.Code != "unignored" {
		t.Fatalf("expected unignored, got %+v", st)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(".env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err = Check(repo)
	if err != nil {
		t.Fatal(err)
	}
	if st.Code != "ignored" {
		t.Fatalf("expected ignored, got %+v", st)
	}
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git("add", "-f", ".env")
	st, err = Check(repo)
	if err != nil {
		t.Fatal(err)
	}
	if st.Code != "tracked" {
		t.Fatalf("expected tracked, got %+v", st)
	}
}
