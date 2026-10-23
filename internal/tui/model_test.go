package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"martinshaw.co/environment-manager/internal/store"
)

func TestUnlockAndBrowse(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateVault("test-passphrase", 12); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	project := filepath.Join(root, "AdviserHub")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".env"), []byte("APP_KEY=abc\n# note\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := st.AddRoot(root); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var m tea.Model = newModel(dir, project, "0.1.0", ctx, st)
	m = send(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	for _, r := range "test-passphrase" {
		m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	view := m.View()
	if !strings.Contains(view, "AdviserHub") {
		t.Fatalf("browse view:\n%s", view)
	}
	m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	for _, r := range "Staging" {
		m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	view = m.View()
	if !strings.Contains(view, "Staging") {
		t.Fatalf("after new env:\n%s", view)
	}
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("l")},
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune("y")},
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune("e")},
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune{'?'}},
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune("g")},
		{Type: tea.KeyEsc},
		{Type: tea.KeyTab},
	} {
		m = send(m, key)
		_ = m.View()
	}
	m = send(m, tea.WindowSizeMsg{Width: 70, Height: 24})
	if view := m.View(); strings.TrimSpace(view) == "" {
		t.Fatal("empty narrow view")
	}
	m = send(m, tea.KeyMsg{Type: tea.KeyCtrlC})
}

func send(m tea.Model, msg tea.Msg) tea.Model {
	next, _ := m.Update(msg)
	return next
}
