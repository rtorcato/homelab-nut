package tui

import "github.com/charmbracelet/lipgloss"

// Palette mirrors the brand established by cover.png: deep navy field,
// emerald primary, amber for the daemon/automation accent. Lipgloss adapts
// these to whatever the user's terminal can render.
var palette = struct {
	primary lipgloss.Color // emerald — accents, active tab
	mascot  lipgloss.Color // amber — daemon / automation callouts (unused for now)
	muted   lipgloss.Color // dim text
	border  lipgloss.Color // subtle borders
	heading lipgloss.Color // headings + titles
}{
	primary: lipgloss.Color("#10b981"),
	mascot:  lipgloss.Color("#fb923c"),
	muted:   lipgloss.Color("#666"),
	border:  lipgloss.Color("#3a3a3a"),
	heading: lipgloss.Color("#FFD166"),
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(palette.heading).
			Padding(0, 1)

	tabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(palette.muted)

	activeTabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Bold(true).
			Foreground(lipgloss.Color("#000")).
			Background(palette.primary)

	tabBarStyle = lipgloss.NewStyle().
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(palette.border).
			MarginBottom(1)

	hintStyle = lipgloss.NewStyle().
			Foreground(palette.muted).
			Italic(true)

	bodyStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(palette.border).
			Padding(1, 2).
			Margin(0, 1)

	labelStyle = lipgloss.NewStyle().
			Foreground(palette.muted).
			Width(10)

	hostItemStyle = lipgloss.NewStyle().
			Padding(0, 1)

	activeHostItemStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Bold(true).
				Foreground(palette.primary)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(palette.muted).
			MarginTop(1).
			Padding(0, 1)

	emptyStateStyle = lipgloss.NewStyle().
			Foreground(palette.muted).
			Italic(true).
			Padding(1, 2)
)
