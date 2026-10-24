package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPassphraseArtSitsBehindModal(t *testing.T) {
	m := newModel(t.TempDir(), t.TempDir(), "1.1.0", nil, nil)
	m.ready = true
	m.width = 120
	m.height = 40
	m.mode = modeUnlock
	view := m.View()
	assertArtScreen(t, view, m.width, m.height, "Unlock envm")
	plain := stripANSI(view)
	if !strings.Contains(plain, "█") || !strings.Contains(plain, "~") || !strings.Contains(plain, "@") {
		t.Fatalf("backdrop missing:\n%s", plain)
	}
	if strings.Count(plain, "Unlock envm") != 1 {
		t.Fatal("modal title was lost")
	}

	m.mode = modeCreatePass
	create := m.View()
	assertArtScreen(t, create, m.width, m.height, "Create a passphrase")

	m.mode = modeConfirmPass
	confirm := m.View()
	assertArtScreen(t, confirm, m.width, m.height, "Confirm the passphrase")

	narrow := m
	narrow.width = 80
	narrow.height = 24
	narrow.mode = modeUnlock
	assertArtScreen(t, narrow.View(), 80, 24, "Unlock envm")
}

func assertArtScreen(t *testing.T, view string, width, height int, title string) {
	t.Helper()
	if !strings.Contains(stripANSI(view), title) {
		t.Fatalf("missing %q\n%s", title, stripANSI(view))
	}
	lines := strings.Split(view, "\n")
	if len(lines) > height {
		t.Fatalf("height %d > %d", len(lines), height)
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w > width {
			t.Fatalf("line %d width %d\n%s", i, w, stripANSI(line))
		}
	}
	if !strings.Contains(view, "\x1b[") {
		t.Fatal("backdrop has no color")
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			_, size := readANSI(s[i:])
			if size == 0 {
				size = 1
			}
			i += size
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
