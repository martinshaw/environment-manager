package tui

import (
	"fmt"
	"strconv"

	"github.com/atotto/clipboard"
	"martinshaw.co/environment-manager/internal/dotenv"
	"martinshaw.co/environment-manager/internal/ignore"
	"martinshaw.co/environment-manager/internal/pending"
	"martinshaw.co/environment-manager/internal/store"
	"martinshaw.co/environment-manager/internal/watch"
)

func (m *Model) saveContent(content []byte) {
	env := m.currentEnv()
	p := m.currentProject()
	if env == nil || p == nil {
		m.errMsg = "Select an env file first."
		return
	}
	writeDisk := p.SelectedEnvID == env.ID
	n, created, err := m.store.SaveContent(p.ID, env.ID, content, writeDisk)
	if err != nil {
		m.errMsg = err.Error()
		return
	}
	m.versionN = n
	if err := m.reload(); err != nil {
		m.errMsg = err.Error()
		return
	}
	if !created {
		m.status = "No change from the latest version."
		return
	}
	m.status = fmt.Sprintf("Saved v%d.", n)
	if writeDisk {
		m.status = fmt.Sprintf("Saved v%d and updated .env.", n)
	}
}

func (m *Model) toggleComment() {
	if m.currentVersion() == nil {
		m.errMsg = "Select a version first."
		return
	}
	next, err := dotenv.ToggleComment(string(m.content), m.lineCur)
	if err != nil {
		m.errMsg = err.Error()
		return
	}
	if next == string(m.content) {
		m.status = "No change."
		return
	}
	m.saveContent([]byte(next))
}

func (m *Model) updateFromDisk() {
	p := m.currentProject()
	if p == nil {
		m.errMsg = "Select a project first."
		return
	}
	name, n, created, err := m.store.UpdateFromDisk(p.ID)
	if err != nil {
		m.errMsg = err.Error()
		return
	}
	m.versionN = n
	_ = m.reload()
	if !created {
		m.status = fmt.Sprintf("%s v%d already matches .env.", name, n)
		return
	}
	m.status = fmt.Sprintf("Stored %s v%d.", name, n)
	m.clearProjectDrift(p.ID)
}

func (m *Model) newFromDisk(name string) {
	p := m.currentProject()
	if p == nil {
		m.errMsg = "Select a project first."
		return
	}
	n, err := m.store.NewFromDisk(p.ID, name)
	if err != nil {
		m.errMsg = err.Error()
		return
	}
	m.versionN = n
	if data, err := m.store.View(); err == nil {
		if proj := data.Project(p.ID); proj != nil {
			m.envID = proj.SelectedEnvID
		}
	}
	_ = m.reload()
	m.status = fmt.Sprintf("Stored %s v%d and selected it.", name, n)
	m.clearProjectDrift(p.ID)
}

func (m *Model) revertDisk() {
	p := m.currentProject()
	if p == nil {
		m.errMsg = "Select a project first."
		return
	}
	name, n, err := m.store.Revert(p.ID)
	if err != nil {
		m.errMsg = err.Error()
		return
	}
	m.versionN = n
	_ = m.reload()
	m.status = fmt.Sprintf("Restored .env to %s v%d.", name, n)
	m.clearProjectDrift(p.ID)
}

func (m *Model) clearProjectDrift(projectID string) {
	var keep []pending.Event
	for _, ev := range m.events {
		if ev.ProjectID == projectID && ev.Kind == pending.KindDrift {
			_ = watch.Dismiss(m.dir, ev.ID)
			continue
		}
		keep = append(keep, ev)
	}
	m.events = keep
}

func (m *Model) doDelete() {
	p := m.currentProject()
	if p == nil {
		m.errMsg = "Nothing to delete."
		return
	}
	var err error
	switch m.deleteKind {
	case "project":
		err = m.store.DeleteProject(p.ID)
		m.projectID = ""
	case "env":
		env := m.currentEnv()
		if env == nil {
			err = store.ErrNotFound
			break
		}
		err = m.store.DeleteEnv(p.ID, env.ID)
		m.envID = ""
	case "version":
		env := m.currentEnv()
		if env == nil {
			err = store.ErrNotFound
			break
		}
		if len(env.Versions) <= 1 {
			err = m.store.DeleteEnv(p.ID, env.ID)
			m.envID = ""
			break
		}
		err = m.store.DeleteVersion(p.ID, env.ID, m.versionN)
	default:
		err = fmt.Errorf("nothing to delete")
	}
	if err != nil {
		m.errMsg = err.Error()
		return
	}
	m.status = "Deleted."
	_ = m.reload()
}

func (m *Model) openDiff(previous bool) {
	env := m.currentEnv()
	ver := m.currentVersion()
	if env == nil || ver == nil {
		m.errMsg = "Select a version first."
		return
	}
	cur, err := m.store.ReadBlob(ver.Blob)
	if err != nil {
		m.errMsg = err.Error()
		return
	}
	var text, title string
	if previous {
		prev := env.Previous(ver.N)
		if prev == nil {
			m.errMsg = "No previous version."
			return
		}
		before, err := m.store.ReadBlob(prev.Blob)
		if err != nil {
			m.errMsg = err.Error()
			return
		}
		text, err = dotenv.Diff("v"+strconv.Itoa(prev.N), "v"+strconv.Itoa(ver.N), string(before), string(cur))
		if err != nil {
			m.errMsg = err.Error()
			return
		}
		title = fmt.Sprintf("v%d → v%d", prev.N, ver.N)
	} else {
		p := m.currentProject()
		disk, ok, err := store.ReadDisk(store.EnvPath(p.Path))
		if err != nil {
			m.errMsg = err.Error()
			return
		}
		name := ".env"
		if !ok {
			disk = nil
			name = ".env (missing)"
		}
		text, err = dotenv.Diff(fmt.Sprintf("v%d", ver.N), name, string(cur), string(disk))
		if err != nil {
			m.errMsg = err.Error()
			return
		}
		title = "selected version → .env"
	}
	if text == "" {
		text = "No differences."
	}
	m.viewport.SetContent(text)
	m.viewport.GotoTop()
	m.status = title
	m.mode = modeDiff
}

func (m *Model) copy(kind string) {
	content := string(m.content)
	var text string
	switch kind {
	case "markdown":
		text = dotenv.Markdown(content)
	case "text":
		text = dotenv.Plain(content)
	case "diff":
		env := m.currentEnv()
		ver := m.currentVersion()
		if env == nil || ver == nil {
			m.errMsg = "Select a version first."
			return
		}
		prev := env.Previous(ver.N)
		if prev == nil {
			m.errMsg = "No previous version to diff."
			return
		}
		before, err := m.store.ReadBlob(prev.Blob)
		if err != nil {
			m.errMsg = err.Error()
			return
		}
		text, err = dotenv.Diff("v"+strconv.Itoa(prev.N), "v"+strconv.Itoa(ver.N), string(before), content)
		if err != nil {
			m.errMsg = err.Error()
			return
		}
		if text == "" {
			text = "No differences.\n"
		}
	default:
		return
	}
	if err := clipboard.WriteAll(text); err != nil {
		m.errMsg = "clipboard: " + err.Error()
		return
	}
	m.status = "Copied " + kind + "."
	m.mode = modeBrowse
}

func (m *Model) copyIgnoreRule() {
	if err := clipboard.WriteAll(ignore.IgnoreRule()); err != nil {
		m.errMsg = "clipboard: " + err.Error()
		return
	}
	m.status = "Copied the .env ignore rule."
}

func (m *Model) dismissEvent() {
	if len(m.events) == 0 {
		m.mode = modeBrowse
		return
	}
	m.eventIdx = clamp(m.eventIdx, len(m.events))
	ev := m.events[m.eventIdx]
	if err := watch.Dismiss(m.dir, ev.ID); err != nil {
		m.errMsg = err.Error()
		return
	}
	next := make([]pending.Event, 0, len(m.events)-1)
	next = append(next, m.events[:m.eventIdx]...)
	next = append(next, m.events[m.eventIdx+1:]...)
	m.events = next
	if len(m.events) == 0 {
		m.mode = modeBrowse
		m.status = "Dismissed."
		return
	}
	m.eventIdx = clamp(m.eventIdx, len(m.events))
}

func (m *Model) applyPending(events []pending.Event) {
	m.events = events
	if len(events) == 0 {
		if m.mode == modeDrift {
			m.mode = modeBrowse
		}
		return
	}
	if m.mode != modeBrowse && m.mode != modeFilter && m.mode != modeSearch {
		return
	}
	for _, ev := range events {
		if !m.snooze[ev.ID] {
			m.mode = modeDrift
			m.eventIdx = 0
			return
		}
	}
}

func (m *Model) snoozeAll() {
	for _, ev := range m.events {
		m.snooze[ev.ID] = true
	}
	m.mode = modeBrowse
}

type settingItem struct {
	kind  string
	id    string
	label string
}

func (m Model) settingItems() []settingItem {
	items := []settingItem{{kind: "header", label: "Roots"}}
	for _, root := range m.data.Config.Roots {
		items = append(items, settingItem{kind: "root", id: root, label: root})
	}
	if len(m.data.Config.Roots) == 0 {
		items = append(items, settingItem{kind: "note", label: "No roots yet"})
	}
	items = append(items, settingItem{kind: "header", label: "Hidden projects"})
	hidden := 0
	for _, p := range m.data.Projects {
		if !p.Hidden {
			continue
		}
		hidden++
		items = append(items, settingItem{kind: "hidden", id: p.ID, label: p.Name})
	}
	if hidden == 0 {
		items = append(items, settingItem{kind: "note", label: "None"})
	}
	return items
}

func (m *Model) moveSetting(delta int) {
	items := m.settingItems()
	if len(items) == 0 {
		return
	}
	next := m.settingCur
	for i := 0; i < len(items); i++ {
		next += delta
		if next < 0 {
			next = len(items) - 1
		}
		if next >= len(items) {
			next = 0
		}
		if items[next].kind == "root" || items[next].kind == "hidden" {
			m.settingCur = next
			return
		}
	}
}
