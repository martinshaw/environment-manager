package tui

import "github.com/charmbracelet/lipgloss"

var (
	accent = lipgloss.AdaptiveColor{Light: "#1d4ed8", Dark: "#7dd3fc"}
	muted  = lipgloss.AdaptiveColor{Light: "#57534e", Dark: "#a8a29e"}
	bad    = lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "#fca5a5"}
	good   = lipgloss.AdaptiveColor{Light: "#166534", Dark: "#86efac"}

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)
	dimStyle   = lipgloss.NewStyle().Foreground(muted)
	errStyle   = lipgloss.NewStyle().Foreground(bad)
	okStyle    = lipgloss.NewStyle().Foreground(good)
	boxStyle   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(muted).
			Padding(1, 2)
	activeStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)
)

func panelStyle(active bool, width, height int) lipgloss.Style {
	st := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(muted)
	if active {
		st = st.BorderForeground(accent)
	}
	innerW := width - 2
	innerH := height - 2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}
	return st.Width(innerW).Height(innerH)
}
