package watch

import (
	"os"
	"path/filepath"
	"testing"

	"martinshaw.co/environment-manager/internal/pending"
	"martinshaw.co/environment-manager/internal/store"
)

func TestCheckDriftAndGitignore(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateVault("test-passphrase", 12); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	project := filepath.Join(root, "Todo App")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(project, ".env")
	if err := os.WriteFile(envPath, []byte("A=1\n"), 0o600); err != nil {
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
	if _, err := s.NewFromDisk(data.Projects[0].ID, "Staging"); err != nil {
		t.Fatal(err)
	}
	if err := Check(dir, Hooks{}); err != nil {
		t.Fatal(err)
	}
	events, err := pending.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if countKind(events, pending.KindDrift) != 0 {
		t.Fatalf("unexpected drift: %#v", events)
	}
	if countKind(events, pending.KindGitignore) != 1 {
		t.Fatalf("expected gitignore notice: %#v", events)
	}
	if err := os.WriteFile(envPath, []byte("A=2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Check(dir, Hooks{}); err != nil {
		t.Fatal(err)
	}
	events, err = pending.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if countKind(events, pending.KindDrift) != 1 {
		t.Fatalf("expected one drift: %#v", events)
	}
	if err := Check(dir, Hooks{}); err != nil {
		t.Fatal(err)
	}
	again, err := pending.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if countKind(again, pending.KindDrift) != 1 || again[driftIndex(again)].ID != events[driftIndex(events)].ID {
		t.Fatalf("drift was re-notified: %#v", again)
	}
	if _, _, _, err := s.UpdateFromDisk(data.Projects[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := Check(dir, Hooks{}); err != nil {
		t.Fatal(err)
	}
	events, err = pending.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if countKind(events, pending.KindDrift) != 0 {
		t.Fatalf("drift should clear after update: %#v", events)
	}
}

func TestCheckSkipsGitignoreWithoutEnv(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateVault("test-passphrase", 12); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	bare := filepath.Join(root, "Bare")
	with := filepath.Join(root, "With")
	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(with, 0o755); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(with, ".env")
	if err := os.WriteFile(envPath, []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.AddRoot(root); err != nil {
		t.Fatal(err)
	}
	if err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	if err := Check(dir, Hooks{}); err != nil {
		t.Fatal(err)
	}
	events, err := pending.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if countKind(events, pending.KindGitignore) != 1 {
		t.Fatalf("expected one gitignore finding, got %#v", events)
	}
	for _, ev := range events {
		if ev.Kind == pending.KindGitignore && ev.Project == "Bare" {
			t.Fatalf("asked to ignore a project with no .env: %#v", ev)
		}
	}
	if err := os.Remove(envPath); err != nil {
		t.Fatal(err)
	}
	if err := Check(dir, Hooks{}); err != nil {
		t.Fatal(err)
	}
	events, err = pending.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if countKind(events, pending.KindGitignore) != 0 {
		t.Fatalf("gitignore finding remained after .env was removed: %#v", events)
	}
	kept := OmitGitignoreWithoutEnv([]pending.Event{
		{ID: "old", Kind: pending.KindGitignore, Project: "Bare", Path: bare, Message: "Bare: no .gitignore"},
		{ID: "keep", Kind: pending.KindDrift, Project: "With", Path: with, Message: "changed"},
	})
	if len(kept) != 1 || kept[0].ID != "keep" {
		t.Fatalf("stale finding was kept: %#v", kept)
	}
}

func TestCheckDoesNotRestoreStoredEnv(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateVault("test-passphrase", 12); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	project := filepath.Join(root, "Todo App")
	if err := os.MkdirAll(project, 0o755); err != nil {
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
	if err := Check(dir, Hooks{}); err != nil {
		t.Fatal(err)
	}
	events, err := pending.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if countKind(events, pending.KindDrift) != 1 {
		t.Fatalf("expected unsaved drift, got %#v", events)
	}
	n, err := s.ImportUnsaved(store.DefaultEnvName)
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
	hashes := map[string]string{}
	for _, p := range data.Projects {
		if env := p.Selected(); env != nil && env.Latest() != nil {
			hashes[p.ID] = env.Latest().SHA256
		}
	}
	if err := SuppressDrift(dir, hashes); err != nil {
		t.Fatal(err)
	}
	if err := Check(dir, Hooks{}); err != nil {
		t.Fatal(err)
	}
	events, err = pending.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if countKind(events, pending.KindDrift) != 0 {
		t.Fatalf("drift returned: %#v", events)
	}
}

func TestCheckIgnoresPausedDrift(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateVault("test-passphrase", 12); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	project := filepath.Join(root, "Todo App")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(project, ".env")
	if err := os.WriteFile(envPath, []byte("A=1\n"), 0o600); err != nil {
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
	id := data.Projects[0].ID
	if _, err := s.NewFromDisk(id, "Local"); err != nil {
		t.Fatal(err)
	}
	if err := PauseDrift(dir, id); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envPath, []byte("A=2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Check(dir, Hooks{}); err != nil {
		t.Fatal(err)
	}
	events, err := pending.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if countKind(events, pending.KindDrift) != 0 {
		t.Fatalf("paused drift was stored: %#v", events)
	}
	if err := ResumeDrift(dir, id); err != nil {
		t.Fatal(err)
	}
	if err := Check(dir, Hooks{}); err != nil {
		t.Fatal(err)
	}
	events, err = pending.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if countKind(events, pending.KindDrift) != 1 {
		t.Fatalf("drift after resume: %#v", events)
	}
}

func countKind(events []pending.Event, kind string) int {
	n := 0
	for _, ev := range events {
		if ev.Kind == kind {
			n++
		}
	}
	return n
}

func driftIndex(events []pending.Event) int {
	for i, ev := range events {
		if ev.Kind == pending.KindDrift {
			return i
		}
	}
	return 0
}
