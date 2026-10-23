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
	project := filepath.Join(root, "AdviserHub")
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
