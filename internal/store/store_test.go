package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateVault("test-passphrase", 12); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestTerminalDirUsesPwd(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	link := filepath.Join(root, "link")
	other := filepath.Join(root, "other")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	t.Chdir(link)
	t.Setenv("PWD", link)
	got, err := TerminalDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != link {
		t.Fatalf("terminal dir %q, pwd is %q", got, link)
	}
	t.Setenv("PWD", other)
	got, err = TerminalDir()
	if err != nil {
		t.Fatal(err)
	}
	if got == other {
		t.Fatalf("used a PWD that is not the terminal directory: %s", got)
	}
	dot, err := os.Stat(".")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(got)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(dot, info) {
		t.Fatalf("terminal dir %s is not the process directory", got)
	}
}

func TestVersionsAndCWD(t *testing.T) {
	s := testStore(t)
	root := t.TempDir()
	project := filepath.Join(root, "Todo App")
	sub := filepath.Join(project, "app")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".env"), []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.AddRoot(root); err != nil {
		t.Fatal(err)
	}
	if err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	data, err := s.View()
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Projects) != 1 || data.Projects[0].Name != "Todo App" {
		t.Fatalf("projects: %#v", data.Projects)
	}
	id := data.Projects[0].ID
	if _, err := s.NewFromDisk(id, "Staging Todo App"); err != nil {
		t.Fatal(err)
	}
	if _, _, created, err := s.UpdateFromDisk(id); err != nil || created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if err := os.WriteFile(filepath.Join(project, ".env"), []byte("A=2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	name, n, created, err := s.UpdateFromDisk(id)
	if err != nil {
		t.Fatal(err)
	}
	if !created || n != 2 || name != "Staging Todo App" {
		t.Fatalf("update name=%s n=%d created=%v", name, n, created)
	}
	if _, err := s.NewFromDisk(id, "Production App"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Use(id, "Staging Todo App"); err != nil {
		t.Fatal(err)
	}
	disk, err := os.ReadFile(filepath.Join(project, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(disk) != "A=2\n" {
		t.Fatalf("use wrote %q", disk)
	}
	if err := os.WriteFile(filepath.Join(project, ".env"), []byte("A=9\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Revert(id); err != nil {
		t.Fatal(err)
	}
	disk, err = os.ReadFile(filepath.Join(project, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(disk) != "A=2\n" {
		t.Fatalf("revert wrote %q", disk)
	}
	got, err := Resolve(mustProjects(t, s), sub)
	if err != nil || got != id {
		t.Fatalf("resolve sub: %s %v", got, err)
	}
	if _, err := Resolve(mustProjects(t, s), root); err != ErrNoProject {
		t.Fatalf("root should not resolve, got %v", err)
	}
	if err := s.RenameProject(id, "Hub"); err != nil {
		t.Fatal(err)
	}
	data, err = s.View()
	if err != nil {
		t.Fatal(err)
	}
	if data.Projects[0].Name != "Hub" {
		t.Fatal(data.Projects[0].Name)
	}
	plain, err := s.ReadBlob(data.Projects[0].Selected().Latest().Blob)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != "A=2\n" {
		t.Fatalf("decrypted %q", plain)
	}
}

func TestScanMonorepo(t *testing.T) {
	s := testStore(t)
	root := t.TempDir()
	repo := filepath.Join(root, "packagevault")
	web := filepath.Join(repo, "packages", "web")
	mod := filepath.Join(repo, "node_modules", "leftpad")
	if err := os.MkdirAll(web, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(web, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mod, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("ROOT=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, ".env"), []byte("WEB=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mod, ".env"), []byte("NO=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.AddRoot(root); err != nil {
		t.Fatal(err)
	}
	if err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	data, err := s.View()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]string{}
	for _, p := range data.Projects {
		byName[p.Name] = p.ID
	}
	if _, ok := byName["packagevault"]; !ok {
		t.Fatalf("missing repository project: %#v", data.Projects)
	}
	webID, ok := byName["packagevault/packages/web"]
	if !ok {
		t.Fatalf("missing nested .env project: %#v", data.Projects)
	}
	for _, p := range data.Projects {
		if strings.Contains(p.Path, "node_modules") {
			t.Fatalf("dependency .env became a project: %#v", p)
		}
	}
	got, err := Resolve(data.Projects, filepath.Join(web, "src"))
	if err != nil || got != webID {
		t.Fatalf("resolve package: %s %v", got, err)
	}
	got, err = Resolve(data.Projects, repo)
	if err != nil || got != byName["packagevault"] {
		t.Fatalf("resolve repository root: %s %v", got, err)
	}
	if err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	again, err := s.View()
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Projects) != len(data.Projects) {
		t.Fatalf("scan duplicated projects: %d then %d", len(data.Projects), len(again.Projects))
	}
}

func TestImportUnsaved(t *testing.T) {
	s := testStore(t)
	root := t.TempDir()
	withEnv := filepath.Join(root, "With")
	without := filepath.Join(root, "Without")
	if err := os.MkdirAll(withEnv, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(without, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(withEnv, ".env"), []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.AddRoot(root); err != nil {
		t.Fatal(err)
	}
	if err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	n, err := s.ImportUnsaved(DefaultEnvName)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("imported %d", n)
	}
	data, err := s.View()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, p := range data.Projects {
		if p.Name != "With" {
			if p.Selected() != nil {
				t.Fatalf("stored an env for %s", p.Name)
			}
			continue
		}
		found = true
		env := p.Selected()
		if env == nil || env.Name != DefaultEnvName || env.Latest() == nil {
			t.Fatalf("%+v", p)
		}
	}
	if !found {
		t.Fatal("missing project")
	}
	again, err := s.ImportUnsaved(DefaultEnvName)
	if err != nil {
		t.Fatal(err)
	}
	if again != 0 {
		t.Fatalf("second import stored %d", again)
	}
}

func mustProjects(t *testing.T, s *Store) []Project {
	t.Helper()
	data, err := s.View()
	if err != nil {
		t.Fatal(err)
	}
	return data.Projects
}
