// Package tui hosts the Bubble Tea models that drive the interactive UI.
//
// Phase 1 (this file): a placeholder shell that proves the loop works and
// quits cleanly. Future phases add real screens (inventory, apply, status).
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// New returns the root Bubble Tea model for the interactive TUI.
func New(version string) tea.Model {
	return placeholderModel{version: version}
}

type placeholderModel struct {
	version string
	width   int
	height  int
}

func (m placeholderModel) Init() tea.Cmd { return nil }

func (m placeholderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFD166")).
			Padding(0, 1)

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888")).
			Italic(true)

	bodyStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7d56f4")).
			Padding(1, 2).
			Margin(1, 2)
)

func (m placeholderModel) View() string {
	title := titleStyle.Render(fmt.Sprintf("homelab-nut %s", m.version))
	body := bodyStyle.Render(
		"Welcome.\n\n" +
			"This is the Phase 1 TUI shell. Real screens — inventory,\n" +
			"apply, status — land in Phase 2 and 3.\n\n" +
			"Track progress: https://github.com/rtorcato/homelab-nut/issues",
	)
	hint := hintStyle.Render("press q or esc to quit")
	return fmt.Sprintf("%s\n%s\n%s\n", title, body, hint)
}
