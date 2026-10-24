package pending

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLoadMissingOrEmpty(t *testing.T) {
	dir := t.TempDir()
	events, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if events != nil {
		t.Fatalf("missing file: %#v", events)
	}
	if err := os.WriteFile(path(dir), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	events, err = Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if events != nil {
		t.Fatalf("empty file: %#v", events)
	}
}

func TestLoadRejectsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(path(dir), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("expected invalid json to fail")
	}
}

func TestSaveLoadAndDismiss(t *testing.T) {
	dir := t.TempDir()
	when := time.Date(2026, 10, 24, 4, 18, 0, 0, time.UTC)
	in := []Event{{
		ID:        "keep",
		Kind:      KindDrift,
		ProjectID: "p1",
		Project:   "Todo App",
		Path:      "/tmp/todo",
		Message:   ".env differs",
		CreatedAt: when,
	}}
	if err := Save(dir, in); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path(dir))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o", info.Mode().Perm())
	}
	raw, err := os.ReadFile(path(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(raw), "\n") {
		t.Fatal("pending file should end with a newline")
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".pending-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temp files left behind: %v", matches)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "keep" || !got[0].CreatedAt.Equal(when) || got[0].Message != ".env differs" {
		t.Fatalf("%#v", got)
	}
	if err := Dismiss(dir, "missing"); err != nil {
		t.Fatal(err)
	}
	got, err = Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("unknown id removed events: %#v", got)
	}
	if err := Dismiss(dir, "keep"); err != nil {
		t.Fatal(err)
	}
	got, err = Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("dismiss left %#v", got)
	}
}

func TestSaveNilWritesEmptyQueue(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, nil); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("%#v", got)
	}
}

func TestDismissMissingFile(t *testing.T) {
	dir := t.TempDir()
	if err := Dismiss(dir, "none"); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("%#v", got)
	}
}

func TestUpsertAndRemoveKind(t *testing.T) {
	drift := Event{ID: "d1", Kind: KindDrift, ProjectID: "p", Message: "old"}
	ignore := Event{ID: "g1", Kind: KindGitignore, ProjectID: "p", Message: "ignore"}
	other := Event{ID: "d2", Kind: KindDrift, ProjectID: "q", Message: "other"}
	events := []Event{drift, ignore, other, {ID: "dup", Kind: KindDrift, ProjectID: "p", Message: "dup"}}

	next := Upsert(events, Event{Kind: KindDrift, ProjectID: "p", Message: "new"})
	if len(events) != 4 || events[0].Message != "old" {
		t.Fatal("upsert changed the input queue")
	}
	if len(next) != 3 {
		t.Fatalf("len %d: %#v", len(next), next)
	}
	if next[0].Message != "new" || next[0].ID == "" || next[0].CreatedAt.IsZero() {
		t.Fatalf("replacement %#v", next[0])
	}
	if next[1].ID != "g1" || next[2].ID != "d2" {
		t.Fatalf("%#v", next)
	}

	kept := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	again := Upsert(next, Event{ID: "fixed", Kind: KindGitignore, ProjectID: "p", Message: "still", CreatedAt: kept})
	if again[1].ID != "fixed" || !again[1].CreatedAt.Equal(kept) || again[1].Message != "still" {
		t.Fatalf("%#v", again[1])
	}

	fresh := Upsert(nil, Event{Kind: KindDrift, ProjectID: "new", Message: "first"})
	if len(fresh) != 1 || fresh[0].ID == "" || fresh[0].CreatedAt.IsZero() {
		t.Fatalf("%#v", fresh)
	}

	left := RemoveKind(again, "p", KindDrift)
	if len(left) != 2 {
		t.Fatalf("%#v", left)
	}
	for _, ev := range left {
		if ev.ProjectID == "p" && ev.Kind == KindDrift {
			t.Fatalf("drift remained: %#v", left)
		}
	}
}
