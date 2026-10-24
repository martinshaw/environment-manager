package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"martinshaw.co/environment-manager/internal/dotenv"
	"martinshaw.co/environment-manager/internal/pending"
	"martinshaw.co/environment-manager/internal/store"
	"martinshaw.co/environment-manager/internal/watch"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		if m.mode == modeEdit {
			m.layoutEditScreen()
		} else {
			m.layoutEditor()
		}
		return m, nil
	case pendingMsg:
		m.applyPending(msg.events)
		m.refreshDisk()
		return m, m.listenPending()
	}
	switch m.mode {
	case modeUnlock, modeCreatePass, modeConfirmPass:
		return m.updatePass(msg)
	case modeShowRecovery:
		return m.updateRecovery(msg)
	case modeAddRoot, modeRename, modePath, modeNewEnv:
		return m.updateText(msg)
	case modeFilter:
		return m.updateFilter(msg)
	case modeSearch:
		return m.updateSearch(msg)
	case modeEdit:
		return m.updateEdit(msg)
	case modeDiff:
		return m.updateDiff(msg)
	case modeDrift:
		return m.updateDrift(msg)
	case modeSettings:
		return m.updateSettings(msg)
	case modeHelp, modeLint, modeCopy, modeConfirm:
		return m.updateOverlay(msg)
	default:
		return m.updateBrowse(msg)
	}
}

func (m *Model) layoutEditor() {
	w, h := m.width-4, m.height-3
	if w < 20 {
		w = 20
	}
	if h < 5 {
		h = 5
	}
	m.editor.SetWidth(w)
	m.editor.SetHeight(h)
	m.viewport.Width = w
	m.viewport.Height = h
}

func (m *Model) layoutEditScreen() {
	// One row is the action line. Leave a column free: the cursor line in
	// the textarea is one cell wider than the width it was given, and that
	// extra cell wraps in the terminal and scrolls the action line off.
	w := m.width - 1
	h := m.height - 1
	if w < 20 {
		w = 20
	}
	if h < 5 {
		h = 5
	}
	m.editor.SetWidth(w)
	m.editor.SetHeight(h)
}

func (m Model) updatePass(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if ok {
		switch keyMsg.String() {
		case "enter":
			return m.submitPass()
		case "ctrl+r":
			if m.mode == modeUnlock {
				m.useRecovery = !m.useRecovery
				m.preparePrompt()
				m.input.SetValue("")
				return m, m.input.Focus()
			}
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) submitPass() (tea.Model, tea.Cmd) {
	value := m.input.Value()
	switch m.mode {
	case modeCreatePass:
		if len(value) < 10 {
			m.errMsg = "Passphrase must be at least 10 characters."
			return m, nil
		}
		m.pass1 = value
		m.input.SetValue("")
		m.mode = modeConfirmPass
		m.errMsg = ""
		m.preparePrompt()
		return m, m.input.Focus()
	case modeConfirmPass:
		if value != m.pass1 {
			m.errMsg = "Passphrases did not match."
			m.pass1 = ""
			m.input.SetValue("")
			m.mode = modeCreatePass
			m.preparePrompt()
			return m, m.input.Focus()
		}
		recovery, err := m.store.CreateVault(value, 0)
		m.pass1 = ""
		m.input.SetValue("")
		if err != nil {
			m.errMsg = err.Error()
			m.mode = modeCreatePass
			m.preparePrompt()
			return m, m.input.Focus()
		}
		m.recoveryKey = strings.TrimSpace(recovery)
		m.recoveryCopied = clipboard.WriteAll(m.recoveryKey) == nil
		m.mode = modeShowRecovery
		m.errMsg = ""
		return m, nil
	default:
		var err error
		if m.useRecovery {
			err = m.store.UnlockRecovery(value)
		} else {
			err = m.store.Unlock(value)
		}
		m.input.SetValue("")
		if err != nil {
			if m.useRecovery {
				m.errMsg = "That recovery key does not open the vault."
			} else {
				m.errMsg = fmt.Sprintf("That passphrase does not open the vault (%d characters). Press ctrl+r to enter the recovery key.", len(value))
			}
			m.preparePrompt()
			return m, m.input.Focus()
		}
		return m.finishUnlock()
	}
}

func (m Model) finishUnlock() (tea.Model, tea.Cmd) {
	if err := m.reload(); err != nil {
		m.errMsg = err.Error()
	}
	if id, err := store.Resolve(m.data.Projects, m.cwd); err == nil {
		m.projectID = id
		m.projectCur = indexOf(m.projectRows(), id)
		m.syncFromCursors()
	}
	m.errMsg = ""
	m.setup = !m.data.Config.SetupDone
	if len(m.data.Config.Roots) == 0 {
		m.mode = modeAddRoot
		m.wizard = true
		m.returnMode = modeBrowse
		m.preparePrompt()
		return m, tea.Batch(m.input.Focus(), m.listenPending())
	}
	if m.setup {
		m.finishSetup()
	} else {
		m.events, _ = pending.Load(m.dir)
		m.events = watch.OmitGitignoreWithoutEnv(m.events)
	}
	if len(m.events) > 0 {
		m.mode = modeDrift
		m.eventIdx = 0
		return m, m.listenPending()
	}
	m.mode = modeBrowse
	return m, m.listenPending()
}

func (m Model) updateRecovery(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && (keyMsg.String() == "enter" || keyMsg.String() == "esc") {
		m.recoveryKey = ""
		if err := m.reload(); err != nil {
			m.errMsg = err.Error()
		}
		if len(m.data.Config.Roots) == 0 {
			m.mode = modeAddRoot
			m.wizard = true
			m.setup = m.setup || !m.data.Config.SetupDone
			m.preparePrompt()
			return m, tea.Batch(m.input.Focus(), m.listenPending())
		}
		return m.finishUnlock()
	}
	return m, nil
}

func (m Model) updateText(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.input.SetValue("")
			m.errMsg = ""
			m.mode = m.returnMode
			if m.wizard {
				m.mode = modeBrowse
				m.wizard = false
			}
			return m, nil
		case "enter":
			return m.submitText()
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) submitText() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())
	switch m.mode {
	case modeAddRoot:
		if value == "" {
			m.wizard = false
			m.input.SetValue("")
			if m.setup {
				m.finishSetup()
			}
			if len(m.events) > 0 {
				m.mode = modeDrift
				m.eventIdx = 0
			} else {
				m.mode = modeBrowse
			}
			return m, m.listenPending()
		}
		if err := m.store.AddRoot(value); err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		m.errMsg = ""
		m.status = "Added root."
		m.input.SetValue("")
		_ = m.reload()
		if !m.wizard {
			m.mode = modeSettings
		}
		return m, nil
	case modeRename:
		if m.focus == focusEnvs {
			env := m.currentEnv()
			p := m.currentProject()
			if env == nil || p == nil {
				m.errMsg = "Select an env file first."
				return m, nil
			}
			if err := m.store.RenameEnv(p.ID, env.ID, value); err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
		} else {
			p := m.currentProject()
			if p == nil {
				m.errMsg = "Select a project first."
				return m, nil
			}
			if err := m.store.RenameProject(p.ID, value); err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
		}
		m.input.SetValue("")
		m.status = "Renamed."
		_ = m.reload()
		m.mode = modeBrowse
		return m, nil
	case modePath:
		p := m.currentProject()
		if p == nil {
			m.errMsg = "Select a project first."
			return m, nil
		}
		missing, err := m.store.SetPath(p.ID, value)
		if err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		m.input.SetValue("")
		_ = m.reload()
		m.mode = modeBrowse
		if missing {
			m.status = "Path saved. Directory was not found."
		} else {
			m.status = "Path saved."
		}
		return m, nil
	case modeNewEnv:
		if m.draft != nil {
			m.newFromDraft(value)
		} else {
			m.newFromDisk(value)
		}
		if m.errMsg != "" {
			return m, nil
		}
		m.input.SetValue("")
		if m.returnMode == modeDrift && len(m.events) > 0 {
			m.mode = modeDrift
		} else {
			m.mode = modeBrowse
		}
		return m, nil
	}
	return m, nil
}

func (m Model) updateFilter(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.setFilter("")
			m.input.SetValue("")
			m.mode = modeBrowse
			m.resetActiveCursor()
			m.syncFromCursors()
			return m, nil
		case "enter":
			m.setFilter(m.input.Value())
			m.mode = modeBrowse
			m.resetActiveCursor()
			m.syncFromCursors()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.setFilter(m.input.Value())
	m.resetActiveCursor()
	m.syncFromCursors()
	return m, cmd
}

func (m *Model) setFilter(q string) {
	switch m.focus {
	case focusEnvs:
		m.envQ = q
	case focusVersions:
		m.versionQ = q
	default:
		m.projectQ = q
	}
}

func (m *Model) activeFilter() string {
	switch m.focus {
	case focusEnvs:
		return m.envQ
	case focusVersions:
		return m.versionQ
	default:
		return m.projectQ
	}
}

func (m *Model) resetActiveCursor() {
	switch m.focus {
	case focusEnvs:
		m.envCur = 0
	case focusVersions:
		m.versionCur = 0
	case focusMain:
	default:
		m.projectCur = 0
		m.envCur = 0
		m.versionCur = 0
	}
}

func (m Model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.searchQ = ""
			m.hits = nil
			m.input.SetValue("")
			m.mode = modeBrowse
			return m, nil
		case "enter":
			m.searchQ = m.input.Value()
			m.recomputeHits()
			m.mode = modeBrowse
			if len(m.hits) > 0 {
				m.lineCur = m.hits[0]
				m.ensureLine(m.visibleLines())
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.searchQ = m.input.Value()
	m.recomputeHits()
	if len(m.hits) > 0 {
		m.lineCur = m.hits[0]
		m.ensureLine(m.visibleLines())
	}
	return m, cmd
}

func (m Model) updateEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.mode = modeBrowse
			return m, nil
		case "ctrl+s":
			m.saveContent([]byte(m.editor.Value()))
			if m.errMsg == "" {
				m.mode = modeBrowse
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	return m, cmd
}

func (m Model) updateDiff(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc", "q":
			m.mode = modeBrowse
			return m, nil
		case "j", "down":
			m.viewport.ScrollDown(1)
			return m, nil
		case "k", "up":
			m.viewport.ScrollUp(1)
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) updateDrift(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(m.events) == 0 {
		m.mode = modeBrowse
		return m, nil
	}
	m.eventIdx = clamp(m.eventIdx, len(m.events))
	ev := m.events[m.eventIdx]
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.snoozeAll()
			return m, nil
		case "j", "down":
			m.eventIdx = (m.eventIdx + 1) % len(m.events)
			return m, nil
		case "k", "up":
			m.eventIdx = (m.eventIdx - 1 + len(m.events)) % len(m.events)
			return m, nil
		case "x":
			m.errMsg = ""
			m.dismissEvent()
			return m, nil
		case "u":
			if ev.Kind != pending.KindDrift || m.driftUnsaved(ev) {
				return m, nil
			}
			m.errMsg = ""
			m.focusProject(ev.ProjectID)
			m.updateFromDisk()
			if m.errMsg == "" && len(m.events) == 0 {
				m.mode = modeBrowse
			}
			return m, nil
		case "a":
			if ev.Kind != pending.KindDrift {
				return m, nil
			}
			m.errMsg = ""
			m.focusProject(ev.ProjectID)
			m.returnMode = modeDrift
			m.mode = modeNewEnv
			m.preparePrompt()
			m.input.SetValue("")
			if m.setup {
				m.input.SetValue(store.DefaultEnvName)
				m.input.CursorEnd()
			}
			return m, m.input.Focus()
		case "R":
			if ev.Kind != pending.KindDrift || m.driftUnsaved(ev) {
				return m, nil
			}
			m.errMsg = ""
			m.focusProject(ev.ProjectID)
			m.revertDisk()
			if m.errMsg == "" && len(m.events) == 0 {
				m.mode = modeBrowse
			}
			return m, nil
		case "c":
			if ev.Kind == pending.KindGitignore {
				m.addGitignore(ev)
			}
			return m, nil
		}
	}
	return m, nil
}

func (m Model) driftUnsaved(ev pending.Event) bool {
	if ev.Kind != pending.KindDrift {
		return false
	}
	p := m.data.Project(ev.ProjectID)
	return p == nil || p.Selected() == nil
}

func (m *Model) focusProject(id string) {
	m.projectID = id
	if p := m.data.Project(id); p != nil && p.SelectedEnvID != "" {
		m.envID = p.SelectedEnvID
	}
	rows := m.projectRows()
	idx := indexOf(rows, id)
	if len(rows) > 0 && rows[idx].ID == id {
		m.projectCur = idx
		m.envCur = indexOf(m.envRows(), m.envID)
		m.versionCur = 0
		m.syncFromCursors()
	}
}

func (m Model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc", "q":
			m.mode = modeBrowse
			return m, nil
		case "j", "down":
			m.moveSetting(1)
			return m, nil
		case "k", "up":
			m.moveSetting(-1)
			return m, nil
		case "a":
			m.returnMode = modeSettings
			m.wizard = false
			m.mode = modeAddRoot
			m.preparePrompt()
			return m, m.input.Focus()
		case "x":
			items := m.settingItems()
			m.settingCur = clamp(m.settingCur, len(items))
			if len(items) == 0 {
				return m, nil
			}
			item := items[m.settingCur]
			if item.kind == "root" {
				if err := m.store.RemoveRoot(item.id); err != nil {
					m.errMsg = err.Error()
					return m, nil
				}
				_ = m.reload()
				m.status = "Removed root."
			}
			return m, nil
		case "enter":
			items := m.settingItems()
			m.settingCur = clamp(m.settingCur, len(items))
			if len(items) == 0 {
				return m, nil
			}
			item := items[m.settingCur]
			if item.kind == "hidden" {
				if err := m.store.SetHidden(item.id, false); err != nil {
					m.errMsg = err.Error()
					return m, nil
				}
				_ = m.reload()
				m.status = "Project is visible again."
			}
			return m, nil
		}
	}
	return m, nil
}

func (m Model) updateOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch m.mode {
	case modeCopy:
		switch keyMsg.String() {
		case "m":
			m.copy("markdown")
			return m, nil
		case "t":
			m.copy("text")
			return m, nil
		case "d":
			m.copy("diff")
			return m, nil
		case "esc", "q":
			m.mode = modeBrowse
			return m, nil
		}
	case modeConfirm:
		switch keyMsg.String() {
		case "y":
			m.doDelete()
			m.mode = modeBrowse
			return m, nil
		case "n", "esc", "q":
			m.mode = modeBrowse
			return m, nil
		}
	default:
		if keyMsg.String() == "esc" || keyMsg.String() == "q" || keyMsg.String() == "enter" {
			m.mode = modeBrowse
			return m, nil
		}
	}
	return m, nil
}

func (m Model) updateBrowse(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	m.errMsg = ""
	switch {
	case key.Matches(keyMsg, m.keys.Quit):
		return m, tea.Quit
	case keyMsg.String() == "tab":
		m.cycle(false)
		return m, nil
	case keyMsg.String() == "shift+tab":
		m.cycle(true)
		return m, nil
	case keyMsg.String() == "enter":
		if m.focus != focusMain {
			m.cycle(false)
		}
		return m, nil
	case keyMsg.String() == "esc":
		if m.focus != focusProjects {
			m.cycle(true)
		}
		return m, nil
	case key.Matches(keyMsg, m.keys.Up):
		m.move(-1)
		return m, nil
	case key.Matches(keyMsg, m.keys.Down):
		m.move(1)
		return m, nil
	case keyMsg.String() == "n" && m.searchQ != "" && m.focus == focusMain:
		m.nextHit(1)
		return m, nil
	case keyMsg.String() == "N" && m.searchQ != "" && m.focus == focusMain:
		m.nextHit(-1)
		return m, nil
	case key.Matches(keyMsg, m.keys.Filter):
		if m.draft != nil && m.focus != focusMain {
			m.errMsg = draftHint
			return m, nil
		}
		if m.focus == focusMain {
			m.mode = modeSearch
			m.input.SetValue(m.searchQ)
			m.preparePrompt()
			return m, m.input.Focus()
		}
		m.mode = modeFilter
		m.input.SetValue(m.activeFilter())
		m.preparePrompt()
		return m, m.input.Focus()
	case key.Matches(keyMsg, m.keys.Help):
		m.mode = modeHelp
		return m, nil
	case key.Matches(keyMsg, m.keys.Settings):
		m.mode = modeSettings
		m.settingCur = 0
		m.moveSetting(1)
		return m, nil
	case key.Matches(keyMsg, m.keys.Pending):
		if m.draft != nil {
			m.errMsg = draftHint
			return m, nil
		}
		if len(m.events) == 0 {
			m.status = "No pending changes."
			return m, nil
		}
		m.mode = modeDrift
		m.eventIdx = 0
		return m, nil
	case key.Matches(keyMsg, m.keys.Edit):
		if m.currentVersion() == nil {
			m.errMsg = "Select a version first."
			return m, nil
		}
		m.editor.SetValue(string(m.shown()))
		m.mode = modeEdit
		m.layoutEditScreen()
		return m, m.editor.Focus()
	case key.Matches(keyMsg, m.keys.Comment):
		m.toggleComment()
		return m, nil
	case key.Matches(keyMsg, m.keys.Diff):
		m.openDiff(true)
		return m, nil
	case key.Matches(keyMsg, m.keys.DiffDisk):
		m.openDiff(false)
		return m, nil
	case key.Matches(keyMsg, m.keys.Lint):
		m.lintIssues = dotenv.Lint(string(m.shown()))
		m.mode = modeLint
		return m, nil
	case key.Matches(keyMsg, m.keys.Copy):
		if m.currentVersion() == nil {
			m.errMsg = "Select a version first."
			return m, nil
		}
		m.mode = modeCopy
		return m, nil
	case key.Matches(keyMsg, m.keys.Delete):
		if m.draft != nil {
			m.errMsg = draftHint
			return m, nil
		}
		m.armDelete()
		return m, nil
	case key.Matches(keyMsg, m.keys.Rename):
		if m.draft != nil {
			m.errMsg = draftHint
			return m, nil
		}
		if m.focus == focusVersions || m.focus == focusMain {
			m.errMsg = "Versions keep their numbers. Rename a project or env file."
			return m, nil
		}
		m.returnMode = modeBrowse
		m.mode = modeRename
		m.preparePrompt()
		if m.focus == focusEnvs && m.currentEnv() != nil {
			m.input.SetValue(m.currentEnv().Name)
		} else if p := m.currentProject(); p != nil {
			m.input.SetValue(p.Name)
		}
		return m, m.input.Focus()
	case key.Matches(keyMsg, m.keys.Path):
		if m.draft != nil {
			m.errMsg = draftHint
			return m, nil
		}
		p := m.currentProject()
		if p == nil {
			m.errMsg = "Select a project first."
			return m, nil
		}
		m.returnMode = modeBrowse
		m.mode = modePath
		m.preparePrompt()
		m.input.SetValue(p.Path)
		return m, m.input.Focus()
	case key.Matches(keyMsg, m.keys.Select):
		if m.draft != nil {
			m.errMsg = draftHint
			return m, nil
		}
		m.selectFocused()
		return m, nil
	case key.Matches(keyMsg, m.keys.Update):
		if m.draft != nil {
			m.saveContent(m.draft)
			return m, nil
		}
		m.updateFromDisk()
		return m, nil
	case key.Matches(keyMsg, m.keys.New):
		if m.currentProject() == nil {
			m.errMsg = "Select a project first."
			return m, nil
		}
		m.returnMode = modeBrowse
		m.mode = modeNewEnv
		m.errMsg = ""
		m.preparePrompt()
		m.input.SetValue("")
		if m.setup && m.draft == nil {
			m.input.SetValue(store.DefaultEnvName)
			m.input.CursorEnd()
		}
		return m, m.input.Focus()
	case key.Matches(keyMsg, m.keys.Revert):
		if m.draft != nil {
			m.draft = nil
			m.status = "Reverted comment changes."
			return m, nil
		}
		m.revertDisk()
		return m, nil
	case key.Matches(keyMsg, m.keys.Hide):
		if m.draft != nil {
			m.errMsg = draftHint
			return m, nil
		}
		p := m.currentProject()
		if p == nil || m.focus != focusProjects {
			return m, nil
		}
		if err := m.store.SetHidden(p.ID, !p.Hidden); err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		m.projectID = ""
		_ = m.reload()
		m.status = "Project hidden. Press g to show it again."
		return m, nil
	}
	return m, nil
}

func (m *Model) cycle(back bool) {
	if back {
		if m.focus == focusProjects {
			m.focus = focusMain
			return
		}
		m.focus--
		return
	}
	if m.focus == focusMain {
		m.focus = focusProjects
		return
	}
	m.focus++
}

func (m *Model) move(delta int) {
	if m.draft != nil && m.focus != focusMain {
		m.errMsg = draftHint
		return
	}
	switch m.focus {
	case focusProjects:
		m.projectCur += delta
		m.envCur = 0
		m.versionCur = 0
	case focusEnvs:
		m.envCur += delta
		m.versionCur = 0
	case focusVersions:
		m.versionCur += delta
	default:
		m.lineCur += delta
		m.ensureLine(m.visibleLines())
		m.syncFromCursors()
		return
	}
	m.syncFromCursors()
}

func (m *Model) nextHit(delta int) {
	if len(m.hits) == 0 {
		m.status = "No matches."
		return
	}
	m.hitAt = (m.hitAt + delta + len(m.hits)) % len(m.hits)
	m.lineCur = m.hits[m.hitAt]
	m.ensureLine(m.visibleLines())
}

func (m Model) visibleLines() int {
	h := m.height - 6
	if h < 1 {
		return 1
	}
	return h
}

func (m *Model) armDelete() {
	p := m.currentProject()
	if p == nil {
		m.errMsg = "Nothing to delete."
		return
	}
	switch m.focus {
	case focusProjects:
		m.deleteKind = "project"
		m.confirmText = "Remove " + p.Name + " from envm? The project directory and its .env stay on disk."
	case focusEnvs:
		env := m.currentEnv()
		if env == nil {
			m.errMsg = "Select an env file first."
			return
		}
		m.deleteKind = "env"
		m.confirmText = "Delete stored env file " + env.Name + " and all of its versions? The on-disk .env stays."
	default:
		env := m.currentEnv()
		ver := m.currentVersion()
		if env == nil || ver == nil {
			m.errMsg = "Select a version first."
			return
		}
		if len(env.Versions) <= 1 {
			m.deleteKind = "env"
			m.confirmText = "This is the only version of " + env.Name + ". Delete the stored env file? The on-disk .env stays."
		} else {
			m.deleteKind = "version"
			m.confirmText = "Delete v" + strconv.Itoa(ver.N) + " of " + env.Name + "?"
		}
	}
	m.mode = modeConfirm
}
