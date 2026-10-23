package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"martinshaw.co/environment-manager/internal/dotenv"
	"martinshaw.co/environment-manager/internal/pending"
)

func (m Model) View() string {
	if !m.ready || m.width == 0 || m.height == 0 {
		return "Starting envm…\n"
	}
	switch m.mode {
	case modeUnlock, modeCreatePass, modeConfirmPass:
		return m.centered(m.passBody())
	case modeShowRecovery:
		return m.centered(m.recoveryBody())
	case modeAddRoot, modeRename, modePath, modeNewEnv:
		return m.centered(m.promptBody())
	case modeHelp:
		return m.helpBody()
	case modeLint:
		return m.lintBody()
	case modeCopy:
		return m.copyBody()
	case modeConfirm:
		return m.centered(m.confirmText + "\n\ny confirm    n cancel")
	case modeDiff:
		return m.diffBody()
	case modeEdit:
		return m.editBody()
	case modeDrift:
		return m.driftBody()
	case modeSettings:
		return m.settingsBody()
	default:
		return m.browserBody()
	}
}

func (m Model) centered(body string) string {
	box := boxStyle.Width(min(m.width-4, 72)).Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) passBody() string {
	title := "Unlock envm"
	help := "ctrl+r enters the recovery key. ctrl+c quits."
	switch m.mode {
	case modeCreatePass:
		title = "Create a passphrase"
		help = "At least 10 characters. It cannot be recovered without the recovery key."
	case modeConfirmPass:
		title = "Confirm the passphrase"
		help = "Enter the same passphrase again."
	}
	body := titleStyle.Render(title) + "\n\n" + m.input.View() + "\n\n" + dimStyle.Render(help)
	if m.errMsg != "" {
		body += "\n\n" + errStyle.Render(m.errMsg)
	}
	return body
}

func (m Model) recoveryBody() string {
	return titleStyle.Render("Recovery key") + "\n\n" +
		errStyle.Render("Shown once. Store it somewhere safe. A forgotten passphrase cannot be undone without this key.") +
		"\n\n" + m.recoveryKey + "\n\n" + dimStyle.Render("Press enter to continue.")
}

func (m Model) promptBody() string {
	title := "envm"
	help := "enter confirms. esc cancels. An empty root finishes setup."
	switch m.mode {
	case modeAddRoot:
		title = "Add a project root"
		if m.wizard {
			help = "Add a directory whose child folders are projects. Leave the field empty and press enter when you are done."
		}
	case modeRename:
		title = "Rename"
	case modePath:
		title = "Project path"
	case modeNewEnv:
		title = "New env file"
		help = "The name is for this stored .env, for example Staging AdviserHub. The current .env becomes v1 and is selected."
	}
	body := titleStyle.Render(title) + "\n\n" + m.input.View() + "\n\n" + dimStyle.Render(help)
	if m.errMsg != "" {
		body += "\n\n" + errStyle.Render(m.errMsg)
	}
	return body
}

func (m Model) browserBody() string {
	footer := m.footerLine()
	bodyH := m.height - 1
	if bodyH < 8 {
		bodyH = 8
	}
	narrow := m.width < 100
	showSide := !narrow || m.focus != focusMain
	showMain := !narrow || m.focus == focusMain
	sideW := 36
	mainW := m.width - sideW
	if narrow {
		if showMain {
			mainW = m.width
		}
		sideW = m.width
	}
	if mainW < 10 {
		mainW = 10
	}
	var parts []string
	if showSide {
		h1 := bodyH / 3
		h2 := bodyH / 3
		h3 := bodyH - h1 - h2
		side := lipgloss.JoinVertical(lipgloss.Left,
			m.listPanel("Projects", m.projectRows(), m.projectCur, sideW, h1, m.sectionActive(focusProjects)),
			m.listPanel("Env files", m.envRows(), m.envCur, sideW, h2, m.sectionActive(focusEnvs)),
			m.listPanel("Versions", m.versionRows(), m.versionCur, sideW, h3, m.sectionActive(focusVersions)),
		)
		parts = append(parts, side)
	}
	if showMain {
		parts = append(parts, m.mainPanel(mainW, bodyH))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...) + "\n" + footer
}

func (m Model) sectionActive(f focus) bool {
	if m.mode != modeBrowse && m.mode != modeFilter && m.mode != modeSearch {
		return false
	}
	return m.focus == f
}

func (m Model) listPanel(title string, rows []Row, cursor, width, height int, active bool) string {
	innerH := height - 2
	if innerH < 1 {
		innerH = 1
	}
	body := renderItems(rows, cursor, innerH-1)
	return panelStyle(active, width, height).Render(titleStyle.Render(title) + "\n" + body)
}

func renderItems(rows []Row, cursor, height int) string {
	if height < 1 {
		height = 1
	}
	if len(rows) == 0 {
		return dimStyle.Render("none")
	}
	start := 0
	if cursor >= height {
		start = cursor - height + 1
	}
	var b strings.Builder
	for i := start; i < len(rows) && i < start+height; i++ {
		row := rows[i]
		marker := "  "
		label := row.Label
		if row.Note != "" {
			label += "  " + row.Note
		}
		line := marker + label
		if i == cursor {
			line = "> " + label
			line = activeStyle.Render(line)
		}
		if i > start {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	return b.String()
}

func (m Model) mainPanel(width, height int) string {
	p := m.currentProject()
	env := m.currentEnv()
	ver := m.currentVersion()
	title := "envm"
	sub := ""
	if p != nil {
		title = p.Name
		sub = p.Path
		if env != nil {
			title += " / " + env.Name
		}
		if ver != nil {
			title += " / v" + fmt.Sprint(ver.N)
		}
	}
	banner := ""
	if m.drifted() {
		banner = okStyle.Render("On-disk .env differs. u update   a save as new   R revert") + "\n"
	}
	body := banner + m.renderFile(width-4, height-5)
	if sub != "" {
		body = dimStyle.Render(sub) + "\n" + body
	}
	return panelStyle(m.sectionActive(focusMain), width, height).Render(titleStyle.Render(title) + "\n" + body)
}

func (m Model) renderFile(width, height int) string {
	if m.currentVersion() == nil {
		if m.currentProject() == nil {
			return dimStyle.Render("Add a root with g, or choose a project.")
		}
		if m.currentEnv() == nil {
			return dimStyle.Render("No stored env files. Press a to store the current .env.")
		}
		return dimStyle.Render("No versions.")
	}
	lines := m.contentLines()
	if len(lines) == 0 {
		return dimStyle.Render("(empty)")
	}
	if height < 1 {
		height = 1
	}
	if width < 8 {
		width = 8
	}
	start := m.lineOff
	var b strings.Builder
	hits := map[int]bool{}
	for _, hit := range m.hits {
		hits[hit] = true
	}
	for i := start; i < len(lines) && i < start+height; i++ {
		marker := "  "
		if i == m.lineCur {
			marker = "> "
		} else if hits[i] {
			marker = "* "
		}
		line := lines[i]
		if len([]rune(line)) > width-6 {
			runes := []rune(line)
			line = string(runes[:width-6])
		}
		text := fmt.Sprintf("%s%4d  %s", marker, i+1, styleLine(line))
		if i == m.lineCur {
			text = activeStyle.Render(fmt.Sprintf("%s%4d  %s", marker, i+1, line))
		}
		if i > start {
			b.WriteByte('\n')
		}
		b.WriteString(text)
	}
	return b.String()
}

func styleLine(line string) string {
	trim := strings.TrimSpace(line)
	if strings.HasPrefix(trim, "#") {
		return dimStyle.Render(line)
	}
	if i := strings.IndexByte(line, '='); i > 0 {
		return titleStyle.Render(line[:i]) + line[i:]
	}
	return line
}

func (m Model) footerLine() string {
	switch m.mode {
	case modeFilter:
		return "filter " + m.input.View() + "  " + dimStyle.Render("enter keeps the filter, esc clears it")
	case modeSearch:
		label := "search " + m.input.View()
		if m.searchQ != "" {
			label += fmt.Sprintf("  %d matches", len(m.hits))
		}
		return label
	}
	if m.errMsg != "" {
		return errStyle.Render(m.errMsg)
	}
	if m.status != "" {
		return m.status + "  " + dimStyle.Render(m.help.View(m.keys))
	}
	right := dimStyle.Render("envm " + m.version)
	left := m.help.View(m.keys)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return left
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) editBody() string {
	title := "Edit    ctrl+s save    esc cancel"
	return titleStyle.Render(title) + "\n" + m.editor.View()
}

func (m Model) diffBody() string {
	return titleStyle.Render("Diff  "+m.status) + "\n" + m.viewport.View() + "\n" + dimStyle.Render("j/k scroll    esc back")
}

func (m Model) lintBody() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Lint"))
	b.WriteString("\n\n")
	if len(m.lintIssues) == 0 {
		b.WriteString("No problems found.\n")
	}
	for _, issue := range m.lintIssues {
		line := fmt.Sprintf("%s  line %d  %s\n", issue.Severity, issue.Line, issue.Message)
		if issue.Severity == dotenv.Error {
			b.WriteString(errStyle.Render(strings.TrimRight(line, "\n")))
		} else {
			b.WriteString(line)
		}
		b.WriteByte('\n')
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("esc back"))
	return b.String()
}

func (m Model) copyBody() string {
	return titleStyle.Render("Copy") + `

m  Markdown fence for Teams
t  Plain text
d  Unified diff against the previous version

esc cancel`
}

func (m Model) helpBody() string {
	h := m.help
	h.ShowAll = true
	h.Width = m.width
	return titleStyle.Render("Keys") + "\n\n" + h.View(m.keys) + "\n\n" +
		dimStyle.Render("While a file search is active, n and N move between matches. a stores the current .env as a new named file.") +
		"\n\n" + dimStyle.Render("esc back")
}

func (m Model) driftBody() string {
	if len(m.events) == 0 {
		return m.centered("No pending changes.\n\nesc back")
	}
	m.eventIdx = clamp(m.eventIdx, len(m.events))
	ev := m.events[m.eventIdx]
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n%s\n%s\n\n", titleStyle.Render(fmt.Sprintf("Pending %d of %d", m.eventIdx+1, len(m.events))), ev.Project, ev.Message)
	if ev.Kind == pending.KindDrift {
		b.WriteString("u  Update the selected env file\na  Save .env as a new file and select it\nR  Revert .env to the latest stored version\n")
	} else {
		b.WriteString("c  Copy a .env gitignore rule\n")
	}
	b.WriteString("x  Dismiss\nesc  Back\n")
	if len(m.events) > 1 {
		b.WriteString("j/k  Other findings\n")
	}
	if m.errMsg != "" {
		b.WriteString("\n" + errStyle.Render(m.errMsg))
	}
	return m.centered(b.String())
}

func (m Model) settingsBody() string {
	items := m.settingItems()
	var b strings.Builder
	b.WriteString(titleStyle.Render("Roots and hidden projects"))
	b.WriteString("\n\n")
	for i, item := range items {
		switch item.kind {
		case "header":
			b.WriteString("\n" + titleStyle.Render(item.label) + "\n")
		case "note":
			b.WriteString(dimStyle.Render("  "+item.label) + "\n")
		default:
			prefix := "  "
			line := prefix + item.label
			if i == m.settingCur {
				line = activeStyle.Render("> " + item.label)
			}
			b.WriteString(line + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("a add root    x remove root    enter unhide    esc back"))
	if m.errMsg != "" {
		b.WriteString("\n" + errStyle.Render(m.errMsg))
	}
	if m.status != "" {
		b.WriteString("\n" + m.status)
	}
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
