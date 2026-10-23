package store

import (
	"os"
	"path/filepath"
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

func TestVersionsAndCWD(t *testing.T) {
	s := testStore(t)
	root := t.TempDir()
	project := filepath.Join(root, "AdviserHub")
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
	if len(data.Projects) != 1 || data.Projects[0].Name != "AdviserHub" {
		t.Fatalf("projects: %#v", data.Projects)
	}
	id := data.Projects[0].ID
	if _, err := s.NewFromDisk(id, "Staging AdviserHub"); err != nil {
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
	if !created || n != 2 || name != "Staging AdviserHub" {
		t.Fatalf("update name=%s n=%d created=%v", name, n, created)
	}
	if _, err := s.NewFromDisk(id, "Production AHub"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Use(id, "Staging AdviserHub"); err != nil {
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

func mustProjects(t *testing.T, s *Store) []Project {
	t.Helper()
	data, err := s.View()
	if err != nil {
		t.Fatal(err)
	}
	return data.Projects
}
