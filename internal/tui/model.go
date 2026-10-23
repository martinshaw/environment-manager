package tui

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"martinshaw.co/environment-manager/internal/dotenv"
	"martinshaw.co/environment-manager/internal/pending"
	"martinshaw.co/environment-manager/internal/store"
)

type mode int

const (
	modeUnlock mode = iota
	modeCreatePass
	modeConfirmPass
	modeShowRecovery
	modeAddRoot
	modeBrowse
	modeFilter
	modeSearch
	modeEdit
	modeDiff
	modeLint
	modeCopy
	modeConfirm
	modeRename
	modePath
	modeNewEnv
	modeDrift
	modeHelp
	modeSettings
)

type focus int

const (
	focusProjects focus = iota
	focusEnvs
	focusVersions
	focusMain
)

// Options configures a TUI session.
type Options struct {
	Dir     string
	CWD     string
	Version string
}

type Model struct {
	dir         string
	cwd         string
	version     string
	ctx         context.Context
	store       *store.Store
	data        store.Data
	width       int
	height      int
	mode        mode
	focus       focus
	projectID   string
	envID       string
	versionN    int
	projectCur  int
	envCur      int
	versionCur  int
	projectQ    string
	envQ        string
	versionQ    string
	lineCur     int
	lineOff     int
	content     []byte
	contentKey  string
	diskHash    string
	input       textinput.Model
	editor      textarea.Model
	viewport    viewport.Model
	help        help.Model
	keys        keyMap
	errMsg      string
	status      string
	pass1       string
	recoveryKey string
	useRecovery bool
	wizard      bool
	returnMode  mode
	searchQ     string
	hits        []int
	hitAt       int
	events      []pending.Event
	eventIdx    int
	snooze      map[string]bool
	confirmText string
	deleteKind  string
	lintIssues  []dotenv.Issue
	settingCur  int
	ready       bool
}

func newModel(dir, cwd, version string, ctx context.Context, st *store.Store) Model {
	ed := textarea.New()
	ed.ShowLineNumbers = true
	ed.CharLimit = 0
	m := Model{
		dir:        dir,
		cwd:        cwd,
		version:    version,
		ctx:        ctx,
		store:      st,
		input:      textinput.New(),
		editor:     ed,
		viewport:   viewport.New(40, 20),
		help:       help.New(),
		keys:       newKeys(),
		snooze:     map[string]bool{},
		mode:       modeUnlock,
		returnMode: modeBrowse,
	}
	if !store.Initialized(dir) {
		m.mode = modeCreatePass
	}
	m.preparePrompt()
	m.input.Focus()
	return m
}

func (m *Model) preparePrompt() {
	secret := false
	placeholder := ""
	switch m.mode {
	case modeUnlock:
		secret = true
		placeholder = "Passphrase"
		if m.useRecovery {
			placeholder = "Recovery key"
		}
	case modeCreatePass:
		secret = true
		placeholder = "New passphrase"
	case modeConfirmPass:
		secret = true
		placeholder = "Confirm passphrase"
	case modeAddRoot:
		placeholder = "Project root directory"
	case modeRename:
		placeholder = "Name"
	case modePath:
		placeholder = "Path"
	case modeNewEnv:
		placeholder = "Name for the new env file"
	case modeFilter:
		placeholder = "Filter"
	case modeSearch:
		placeholder = "Search"
	}
	m.input.Placeholder = placeholder
	m.input.Prompt = "> "
	m.input.CharLimit = 0
	m.input.Width = 64
	if secret {
		m.input.EchoMode = textinput.EchoPassword
		m.input.EchoCharacter = '•'
	} else {
		m.input.EchoMode = textinput.EchoNormal
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *Model) projectRows() []Row {
	cwdID, _ := store.Resolve(m.data.Projects, m.cwd)
	rows := make([]Row, 0, len(m.data.Projects))
	for _, p := range m.data.Projects {
		if p.Hidden {
			continue
		}
		note := ""
		if p.Missing {
			note = "missing"
		}
		if p.ID == cwdID {
			note = "cwd"
		}
		rows = append(rows, Row{ID: p.ID, Label: p.Name, Pinned: p.ID == cwdID, Note: note})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].Label) < strings.ToLower(rows[j].Label)
	})
	return FilterRows(rows, m.projectQ)
}

func (m *Model) currentProject() *store.Project {
	return m.data.Project(m.projectID)
}

func (m *Model) envRows() []Row {
	p := m.currentProject()
	if p == nil {
		return nil
	}
	rows := make([]Row, 0, len(p.Envs))
	for _, env := range p.Envs {
		note := ""
		if env.ID == p.SelectedEnvID {
			note = "selected"
		}
		rows = append(rows, Row{
			ID:     env.ID,
			Label:  env.Name,
			Pinned: env.ID == p.SelectedEnvID,
			Note:   note,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].Label) < strings.ToLower(rows[j].Label)
	})
	return FilterRows(rows, m.envQ)
}

func (m *Model) currentEnv() *store.EnvFile {
	p := m.currentProject()
	if p == nil {
		return nil
	}
	return p.Env(m.envID)
}

func (m *Model) versionRows() []Row {
	env := m.currentEnv()
	if env == nil {
		return nil
	}
	vers := append([]store.Version(nil), env.Versions...)
	sort.SliceStable(vers, func(i, j int) bool { return vers[i].N > vers[j].N })
	latest := 0
	if len(vers) > 0 {
		latest = vers[0].N
	}
	rows := make([]Row, 0, len(vers))
	for _, v := range vers {
		note := ""
		if v.N == latest {
			note = "latest"
		}
		rows = append(rows, Row{
			ID:     strconv.Itoa(v.N),
			N:      v.N,
			Label:  "v" + strconv.Itoa(v.N),
			Pinned: v.N == latest,
			Note:   note,
		})
	}
	return FilterRows(rows, m.versionQ)
}

func (m *Model) currentVersion() *store.Version {
	env := m.currentEnv()
	if env == nil {
		return nil
	}
	return env.ByN(m.versionN)
}

func (m *Model) syncFromCursors() {
	projects := m.projectRows()
	m.projectCur = clamp(m.projectCur, len(projects))
	if len(projects) == 0 {
		m.projectID = ""
		m.envID = ""
		m.versionN = 0
		m.content = nil
		m.contentKey = ""
		return
	}
	m.projectID = projects[m.projectCur].ID
	envs := m.envRows()
	m.envCur = clamp(m.envCur, len(envs))
	if len(envs) == 0 {
		m.envID = ""
		m.versionN = 0
		m.content = nil
		m.contentKey = ""
		return
	}
	m.envID = envs[m.envCur].ID
	vers := m.versionRows()
	m.versionCur = clamp(m.versionCur, len(vers))
	if len(vers) == 0 {
		m.versionN = 0
		m.content = nil
		m.contentKey = ""
		return
	}
	m.versionN = vers[m.versionCur].N
	m.loadContent()
}

func (m *Model) reload() error {
	if err := m.store.Scan(); err != nil {
		return err
	}
	data, err := m.store.View()
	if err != nil {
		return err
	}
	m.data = data
	m.projectCur = indexOf(m.projectRows(), m.projectID)
	m.envCur = indexOf(m.envRows(), m.envID)
	m.versionCur = indexOf(m.versionRows(), strconv.Itoa(m.versionN))
	m.syncFromCursors()
	m.refreshDisk()
	return nil
}

func (m *Model) loadContent() {
	env := m.currentEnv()
	ver := m.currentVersion()
	if env == nil || ver == nil {
		m.content = nil
		m.contentKey = ""
		return
	}
	key := env.ID + ":" + strconv.Itoa(ver.N)
	if key == m.contentKey {
		return
	}
	b, err := m.store.ReadBlob(ver.Blob)
	if err != nil {
		m.errMsg = err.Error()
		m.content = nil
		m.contentKey = ""
		return
	}
	m.content = b
	m.contentKey = key
	m.lineCur = 0
	m.lineOff = 0
	m.hits = nil
	m.recomputeHits()
}

func (m *Model) refreshDisk() {
	p := m.currentProject()
	if p == nil {
		m.diskHash = ""
		return
	}
	h, err := store.HashDisk(store.EnvPath(p.Path))
	if err != nil {
		return
	}
	m.diskHash = h
}

func (m *Model) recomputeHits() {
	m.hits = nil
	m.hitAt = 0
	q := strings.ToLower(strings.TrimSpace(m.searchQ))
	if q == "" {
		return
	}
	lines, _ := dotenv.Lines(string(m.content))
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), q) {
			m.hits = append(m.hits, i)
		}
	}
}

func (m *Model) contentLines() []string {
	lines, _ := dotenv.Lines(string(m.content))
	return lines
}

func (m *Model) ensureLine(visible int) {
	lines := m.contentLines()
	if len(lines) == 0 {
		m.lineCur = 0
		m.lineOff = 0
		return
	}
	m.lineCur = clamp(m.lineCur, len(lines))
	if visible < 1 {
		visible = 1
	}
	if m.lineCur < m.lineOff {
		m.lineOff = m.lineCur
	}
	if m.lineCur >= m.lineOff+visible {
		m.lineOff = m.lineCur - visible + 1
	}
}

func (m Model) drifted() bool {
	p := m.currentProject()
	env := m.currentEnv()
	if p == nil || env == nil || p.SelectedEnvID != env.ID {
		return false
	}
	lat := env.Latest()
	if lat == nil || m.diskHash == "" {
		return false
	}
	return m.diskHash != lat.SHA256
}

func clamp(i, n int) int {
	if n <= 0 {
		return 0
	}
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

func indexOf(rows []Row, id string) int {
	for i, row := range rows {
		if row.ID == id {
			return i
		}
	}
	return 0
}

type pendingMsg struct{ events []pending.Event }

func (m Model) listenPending() tea.Cmd {
	dir := m.dir
	ctx := m.ctx
	return func() tea.Msg {
		return pendingMsg{events: listenEvents(ctx, dir)}
	}
}
