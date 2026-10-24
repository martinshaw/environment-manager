package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"martinshaw.co/environment-manager/internal/pending"
	"martinshaw.co/environment-manager/internal/store"
)

func TestVersionLabelShowsCreatedAt(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateVault("test-passphrase", 12); err != nil {
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
	if err := st.AddRoot(root); err != nil {
		t.Fatal(err)
	}
	if err := st.Scan(); err != nil {
		t.Fatal(err)
	}
	data, err := st.View()
	if err != nil {
		t.Fatal(err)
	}
	id := data.Projects[0].ID
	if _, err := st.NewFromDisk(id, "Local"); err != nil {
		t.Fatal(err)
	}
	data, err = st.View()
	if err != nil {
		t.Fatal(err)
	}
	ver := data.Project(id).EnvByName("Local").Latest()
	if ver == nil || ver.CreatedAt.IsZero() {
		t.Fatal("version was stored without a creation time")
	}
	m := newModel(dir, project, "0.1.0", context.Background(), st)
	m.ready = true
	m.width = 120
	m.height = 40
	m.mode = modeBrowse
	m.projectID = id
	if err := m.reload(); err != nil {
		t.Fatal(err)
	}
	want := "v1  " + ver.CreatedAt.Local().Format("2 Jan 15:04")
	rows := m.versionRows()
	if len(rows) != 1 || rows[0].Label != want {
		t.Fatalf("version row %v, want %q", rows, want)
	}
	if !strings.Contains(m.View(), want) {
		t.Fatal("open file title omits the version time")
	}
}

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
	project := filepath.Join(root, "Todo App")
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
	if !strings.Contains(view, "Todo App") {
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

func TestNewEnvErrorDoesNotStick(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateVault("test-passphrase", 12); err != nil {
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
	if err := st.AddRoot(root); err != nil {
		t.Fatal(err)
	}
	if err := st.Scan(); err != nil {
		t.Fatal(err)
	}
	data, err := st.View()
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(dir, project, "0.1.0", context.Background(), st)
	m.ready = true
	m.width = 100
	m.height = 30
	m.data = data
	m.projectID = data.Projects[0].ID
	m.mode = modeNewEnv
	m.returnMode = modeDrift
	m.errMsg = "no selected env file"
	m.events = []pending.Event{{
		ID: "e1", Kind: pending.KindDrift, ProjectID: m.projectID, Project: "Todo App", Path: project,
		Message: "Todo App: .env is not stored yet. Save it as a new file.",
	}}
	m.input.SetValue(store.DefaultEnvName)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	if got.mode == modeNewEnv {
		t.Fatalf("stuck on name screen:\n%s", got.View())
	}
	got.mode = modeNewEnv
	got.returnMode = modeDrift
	got.errMsg = "an env file with that name already exists"
	got.events = []pending.Event{{
		ID: "e2", Kind: pending.KindDrift, ProjectID: got.projectID, Project: "Todo App", Path: project,
		Message: "still pending",
	}}
	got.input.SetValue(store.DefaultEnvName)
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = next.(Model)
	if got.mode == modeNewEnv {
		t.Fatalf("duplicate name stuck:\n%s", got.View())
	}
}

func TestGitignoreActionAdvances(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	other := t.TempDir()
	m := newModel(dir, project, "0.1.0", context.Background(), st)
	m.ready = true
	m.width = 90
	m.height = 28
	m.mode = modeDrift
	m.events = []pending.Event{
		{ID: "1", Kind: pending.KindGitignore, ProjectID: "p", Project: "Demo", Path: project, Message: "Demo: .env is not covered by .gitignore"},
		{ID: "2", Kind: pending.KindGitignore, ProjectID: "q", Project: "Other", Path: other, Message: "Other: .env is not covered by .gitignore"},
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	got := next.(Model)
	view := got.View()
	if !strings.Contains(view, "Pending 1 of 1") || strings.Contains(view, "Demo:") {
		t.Fatalf("after first add:\n%s", view)
	}
	rule, err := os.ReadFile(filepath.Join(project, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(rule) != ".env\n" {
		t.Fatalf("gitignore %q", rule)
	}
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	got = next.(Model)
	if got.mode == modeDrift {
		t.Fatalf("still pending:\n%s", got.View())
	}
}

func TestRecoveryKeyIsOneLine(t *testing.T) {
	m := newModel(t.TempDir(), t.TempDir(), "0.1.0", context.Background(), nil)
	m.ready = true
	m.width = 80
	m.height = 24
	m.mode = modeShowRecovery
	m.recoveryKey = "AGE-SECRET-KEY-1EXAMPLEKEYEXAMPLEKEYEXAMPLEKEYEXAMPLEKEYEXAMPLEKEYEXAMP"
	m.recoveryCopied = true
	view := m.View()
	var keyLine string
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "AGE-SECRET-KEY-") {
			if keyLine != "" {
				t.Fatalf("key split across lines:\n%s", view)
			}
			keyLine = line
		}
	}
	if keyLine != m.recoveryKey {
		t.Fatalf("key line %q", keyLine)
	}
}

func TestSetupNameDefault(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateVault("test-passphrase", 12); err != nil {
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
	if err := st.AddRoot(root); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetupDone(); err != nil {
		t.Fatal(err)
	}
	if err := st.Scan(); err != nil {
		t.Fatal(err)
	}
	data, err := st.View()
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(dir, project, "0.1.0", context.Background(), st)
	m.ready = true
	m.width = 100
	m.height = 30
	m.data = data
	m.projectID = data.Projects[0].ID
	m.mode = modeBrowse
	m.setup = true
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	got := next.(Model)
	if got.input.Value() != store.DefaultEnvName {
		t.Fatalf("setup default %q", got.input.Value())
	}
	got.setup = false
	got.mode = modeBrowse
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	got = next.(Model)
	if got.input.Value() != "" {
		t.Fatalf("later default %q", got.input.Value())
	}
}

func TestPendingGitignoreWithoutEnvIsHidden(t *testing.T) {
	project := t.TempDir()
	m := newModel(t.TempDir(), project, "0.1.0", context.Background(), nil)
	m.ready = true
	m.width = 80
	m.height = 24
	m.mode = modeDrift
	m.applyPending([]pending.Event{{
		ID: "1", Kind: pending.KindGitignore, ProjectID: "p", Project: "marnoir", Path: project,
		Message: "marnoir: no .gitignore in the project, and .env is not ignored",
	}})
	if len(m.events) != 0 || m.mode == modeDrift {
		t.Fatalf("still asking to ignore a project with no .env: mode %v events %#v", m.mode, m.events)
	}
	if err := os.WriteFile(filepath.Join(project, ".env"), []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m.mode = modeBrowse
	m.applyPending([]pending.Event{{
		ID: "2", Kind: pending.KindGitignore, ProjectID: "p", Project: "marnoir", Path: project,
		Message: "marnoir: no .gitignore in the project, and .env is not ignored",
	}})
	if len(m.events) != 1 || m.mode != modeDrift {
		t.Fatalf("finding with a .env was hidden: mode %v events %#v", m.mode, m.events)
	}
}

func TestSelectFileAndVersion(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateVault("test-passphrase", 12); err != nil {
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
	if err := st.AddRoot(root); err != nil {
		t.Fatal(err)
	}
	if err := st.Scan(); err != nil {
		t.Fatal(err)
	}
	data, err := st.View()
	if err != nil {
		t.Fatal(err)
	}
	id := data.Projects[0].ID
	if _, err := st.NewFromDisk(id, "Local"); err != nil {
		t.Fatal(err)
	}
	data, err = st.View()
	if err != nil {
		t.Fatal(err)
	}
	localID := data.Project(id).EnvByName("Local").ID
	if _, _, err := st.SaveContent(id, localID, []byte("A=2\n"), true); err != nil {
		t.Fatal(err)
	}
	if _, err := st.NewFromContent(id, "Staging", []byte("S=1\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Use(id, "Local"); err != nil {
		t.Fatal(err)
	}
	data, err = st.View()
	if err != nil {
		t.Fatal(err)
	}
	stagingID := data.Project(id).EnvByName("Staging").ID
	m := newModel(dir, project, "0.1.0", context.Background(), st)
	m.ready = true
	m.width = 120
	m.height = 40
	m.mode = modeBrowse
	m.projectID = id
	m.envID = stagingID
	m.versionN = 1
	m.focus = focusEnvs
	if err := m.reload(); err != nil {
		t.Fatal(err)
	}
	got := send(m, runeKey("s")).(Model)
	data, err = st.View()
	if err != nil {
		t.Fatal(err)
	}
	if data.Project(id).SelectedEnvID != stagingID {
		t.Fatal("env file was not selected")
	}
	disk, err := os.ReadFile(filepath.Join(project, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(disk) != "S=1\n" {
		t.Fatalf("disk after selecting file %q", disk)
	}
	got.envID = localID
	got.versionN = 1
	got.focus = focusVersions
	if err := got.reload(); err != nil {
		t.Fatal(err)
	}
	got = send(got, runeKey("s")).(Model)
	if got.versionN != 3 {
		t.Fatalf("promoted version %d", got.versionN)
	}
	if n := versionCount(t, &got, "Local"); n != 3 {
		t.Fatalf("Local versions %d", n)
	}
	data, err = st.View()
	if err != nil {
		t.Fatal(err)
	}
	if data.Project(id).SelectedEnvID != localID {
		t.Fatal("promoting a version did not select its env file")
	}
	disk, err = os.ReadFile(filepath.Join(project, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(disk) != "A=1\n" {
		t.Fatalf("disk after selecting v1 %q", disk)
	}
}

func TestUpdateWithoutChangeKeepsVersion(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateVault("test-passphrase", 12); err != nil {
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
	if err := st.AddRoot(root); err != nil {
		t.Fatal(err)
	}
	if err := st.Scan(); err != nil {
		t.Fatal(err)
	}
	data, err := st.View()
	if err != nil {
		t.Fatal(err)
	}
	id := data.Projects[0].ID
	if _, err := st.NewFromDisk(id, "Local"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.NewFromContent(id, "Staging", []byte("S=1\n")); err != nil {
		t.Fatal(err)
	}
	data, err = st.View()
	if err != nil {
		t.Fatal(err)
	}
	proj := data.Project(id)
	staging := proj.EnvByName("Staging")
	if _, _, err := st.SaveContent(id, staging.ID, []byte("S=2\n"), false); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Use(id, "Local"); err != nil {
		t.Fatal(err)
	}
	m := newModel(dir, project, "0.1.0", context.Background(), st)
	m.ready = true
	m.width = 120
	m.height = 40
	m.mode = modeBrowse
	m.projectID = id
	m.envID = staging.ID
	m.versionN = 2
	if err := m.reload(); err != nil {
		t.Fatal(err)
	}
	if m.versionN != 2 || m.envID != staging.ID {
		t.Fatalf("setup version=%d env=%s", m.versionN, m.envID)
	}
	got := send(m, runeKey("u")).(Model)
	if got.versionN != 2 || got.envID != staging.ID {
		t.Fatalf("u moved to version %d env %s", got.versionN, got.envID)
	}
	if !strings.Contains(got.status, "already matches") {
		t.Fatalf("status %q", got.status)
	}
}

func TestCommentChangesWait(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateVault("test-passphrase", 12); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	project := filepath.Join(root, "Todo App")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".env"), []byte("A=1\nB=2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := st.AddRoot(root); err != nil {
		t.Fatal(err)
	}
	if err := st.Scan(); err != nil {
		t.Fatal(err)
	}
	data, err := st.View()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.NewFromDisk(data.Projects[0].ID, "Local"); err != nil {
		t.Fatal(err)
	}
	m := newModel(dir, project, "0.1.0", context.Background(), st)
	m.ready = true
	m.width = 120
	m.height = 40
	m.mode = modeBrowse
	if err := m.reload(); err != nil {
		t.Fatal(err)
	}
	m.focus = focusMain
	if n := versionCount(t, &m, "Local"); n != 1 {
		t.Fatalf("start versions %d", n)
	}
	got := send(m, runeKey("c")).(Model)
	if n := versionCount(t, &got, "Local"); n != 1 {
		t.Fatalf("comment stored a version: %d", n)
	}
	view := got.View()
	if !strings.Contains(view, "# A=1") || !strings.Contains(view, "u update") {
		t.Fatalf("draft not shown:\n%s", view)
	}
	got = send(got, runeKey("j")).(Model)
	got = send(got, runeKey("c")).(Model)
	if n := versionCount(t, &got, "Local"); n != 1 {
		t.Fatalf("second comment stored a version: %d", n)
	}
	view = got.View()
	if !strings.Contains(view, "# A=1") || !strings.Contains(view, "# B=2") {
		t.Fatalf("both comments:\n%s", view)
	}
	got.focus = focusProjects
	cur := got.projectCur
	got = send(got, runeKey("j")).(Model)
	if got.projectCur != cur || got.draft == nil {
		t.Fatal("left the file while comment changes were unsaved")
	}
	got.focus = focusMain
	got = send(got, runeKey("R")).(Model)
	view = got.View()
	if strings.Contains(view, "# A=1") || strings.Contains(view, "Comment changes") {
		t.Fatalf("revert kept the draft:\n%s", view)
	}
	got = send(got, runeKey("k")).(Model)
	got = send(got, runeKey("c")).(Model)
	got = send(got, runeKey("j")).(Model)
	got = send(got, runeKey("c")).(Model)
	got = send(got, runeKey("u")).(Model)
	if n := versionCount(t, &got, "Local"); n != 2 {
		t.Fatalf("update versions %d", n)
	}
	disk, err := os.ReadFile(filepath.Join(project, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(disk) != "# A=1\n# B=2\n" {
		t.Fatalf("disk after update %q", disk)
	}
	if strings.Contains(got.View(), "Comment changes") {
		t.Fatal("banner remained after update")
	}
	got = send(got, runeKey("c")).(Model)
	got = send(got, runeKey("a")).(Model)
	for _, r := range "Copy" {
		got = send(got, runeKey(string(r))).(Model)
	}
	got = send(got, tea.KeyMsg{Type: tea.KeyEnter}).(Model)
	if got.mode == modeNewEnv {
		t.Fatalf("stuck naming the new env:\n%s", got.View())
	}
	if n := versionCount(t, &got, "Local"); n != 2 {
		t.Fatalf("saving as new changed Local: %d versions", n)
	}
	if n := versionCount(t, &got, "Copy"); n != 1 {
		t.Fatalf("Copy versions %d", n)
	}
	disk, err = os.ReadFile(filepath.Join(project, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(disk) != "A=1\n# B=2\n" {
		t.Fatalf("disk after new %q", disk)
	}
}

func versionCount(t *testing.T, m *Model, name string) int {
	t.Helper()
	data, err := m.store.View()
	if err != nil {
		t.Fatal(err)
	}
	p := data.Project(m.projectID)
	if p == nil {
		t.Fatal("missing project")
	}
	env := p.EnvByName(name)
	if env == nil {
		t.Fatalf("missing env %s", name)
	}
	return len(env.Versions)
}

func runeKey(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestVimChoiceAfterExit(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateVault("test-passphrase", 12); err != nil {
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
	if err := st.AddRoot(root); err != nil {
		t.Fatal(err)
	}
	if err := st.Scan(); err != nil {
		t.Fatal(err)
	}
	data, err := st.View()
	if err != nil {
		t.Fatal(err)
	}
	id := data.Projects[0].ID
	if _, err := st.NewFromDisk(id, "Local"); err != nil {
		t.Fatal(err)
	}
	m := newModel(dir, project, "1.0.1", context.Background(), st)
	m.ready = true
	m.width = 80
	m.height = 24
	m.mode = modeBrowse
	if err := m.reload(); err != nil {
		t.Fatal(err)
	}
	m.draft = []byte("A=1\n")
	blocked := send(m, runeKey("v")).(Model)
	if blocked.mode != modeBrowse || !strings.Contains(blocked.errMsg, "not saved") {
		t.Fatalf("vim started during a draft: %s", blocked.errMsg)
	}
	m.draft = nil
	m.refreshDisk()
	m.vimBefore = m.diskHash
	m.vimProject = id
	unchanged := send(m, vimDoneMsg{}).(Model)
	if unchanged.mode != modeBrowse || unchanged.status != "No change." {
		t.Fatalf("unchanged vim: mode %d status %q", unchanged.mode, unchanged.status)
	}

	m.refreshDisk()
	m.vimBefore = m.diskHash
	m.vimProject = id
	if err := os.WriteFile(envPath, []byte("A=2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m.events = []pending.Event{{ID: "drift", Kind: pending.KindDrift, ProjectID: id, Project: "Todo App", Message: "changed"}}
	m.applyPending(m.events)
	if m.mode == modeDrift {
		t.Fatal("drift opened while vim was in progress")
	}
	got := send(m, vimDoneMsg{}).(Model)
	if got.mode != modeVimChoice {
		t.Fatalf("mode %d", got.mode)
	}
	if !strings.Contains(got.View(), "u  update") || !strings.Contains(got.View(), "Local v1") {
		t.Fatalf("choice screen:\n%s", got.View())
	}
	got = send(got, runeKey("u")).(Model)
	if got.mode != modeBrowse {
		t.Fatalf("update stayed in %d: %s", got.mode, got.errMsg)
	}
	if n := versionCount(t, &got, "Local"); n != 2 {
		t.Fatalf("versions %d", n)
	}
	if got.versionN != 2 || got.vimProject != "" {
		t.Fatalf("version %d vim %q", got.versionN, got.vimProject)
	}
	disk, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(disk) != "A=2\n" {
		t.Fatalf("disk %q", disk)
	}

	if err := os.WriteFile(envPath, []byte("A=3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got.vimBefore = "previous"
	got.vimProject = id
	got = send(got, vimDoneMsg{}).(Model)
	got = send(got, runeKey("R")).(Model)
	disk, err = os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(disk) != "A=2\n" || got.mode != modeBrowse {
		t.Fatalf("revert disk %q mode %d err %s", disk, got.mode, got.errMsg)
	}

	if err := os.WriteFile(envPath, []byte("A=4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got.vimBefore = "previous"
	got.vimProject = id
	got = send(got, vimDoneMsg{}).(Model)
	got = send(got, runeKey("a")).(Model)
	for _, r := range "Copy" {
		got = send(got, runeKey(string(r))).(Model)
	}
	got = send(got, tea.KeyMsg{Type: tea.KeyEnter}).(Model)
	if n := versionCount(t, &got, "Copy"); n != 1 {
		t.Fatalf("Copy versions %d err %s", n, got.errMsg)
	}
	if got.vimProject != "" {
		t.Fatal("vim pause remained after new")
	}
	disk, err = os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(disk) != "A=4\n" {
		t.Fatalf("disk after new %q", disk)
	}
}

func TestEditActionsStayVisible(t *testing.T) {
	m := newModel(t.TempDir(), t.TempDir(), "0.1.0", context.Background(), nil)
	m.ready = true
	m.width = 80
	m.height = 24
	m.mode = modeEdit
	var body strings.Builder
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&body, "KEY_%02d=%s\n", i, strings.Repeat("v", 40))
	}
	m.editor.SetValue(body.String())
	m.layoutEditScreen()
	m.editor.Focus()
	view := m.View()
	if !strings.Contains(visibleTail(view, m.height), "ctrl+s") {
		t.Fatalf("actions clipped before edit:\n%s", view)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	got := next.(Model)
	view = got.View()
	first, _, _ := strings.Cut(view, "\n")
	if !strings.Contains(first, "ctrl+s") || !strings.Contains(first, "esc cancel") {
		t.Fatalf("action line is not the first row:\n%s", first)
	}
	if !strings.Contains(visibleTail(view, got.height), "ctrl+s") {
		t.Fatalf("actions clipped after edit (%d lines, height %d):\n%s", strings.Count(view, "\n")+1, got.height, view)
	}
	for i, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > got.width {
			t.Fatalf("line %d width %d > %d: %q", i, w, got.width, line)
		}
	}
	if n := strings.Count(view, "\n") + 1; n > got.height {
		t.Fatalf("edit view is %d lines, screen is %d", n, got.height)
	}
}

func TestLongStatusDoesNotGrowScreen(t *testing.T) {
	m := newModel(t.TempDir(), t.TempDir(), "0.1.0", context.Background(), nil)
	m.ready = true
	m.width = 70
	m.height = 20
	m.mode = modeBrowse
	m.status = strings.Repeat("Stored Local Development v1. ", 6)
	view := m.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) > m.height {
		t.Fatalf("view is %d lines, screen is %d", len(lines), m.height)
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w > m.width {
			t.Fatalf("line %d width %d > %d", i, w, m.width)
		}
	}
	if !strings.Contains(lines[len(lines)-1], "quit") {
		t.Fatalf("status pushed the actions off the footer: %q", lines[len(lines)-1])
	}
}

func visibleTail(view string, height int) string {
	lines := strings.Split(view, "\n")
	if len(lines) > height {
		lines = lines[len(lines)-height:]
	}
	return strings.Join(lines, "\n")
}

func send(m tea.Model, msg tea.Msg) tea.Model {
	next, _ := m.Update(msg)
	return next
}
