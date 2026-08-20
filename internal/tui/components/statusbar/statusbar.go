package statusbar

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/homiakus/agctl/internal/tui/theme"
)

type KeyHint struct {
	Key  string
	Desc string
}

type Model struct {
	Mode    string // "NORMAL", "SEARCH", "COMMAND", "RUNNING"
	Message string // Ephemeral status/toast message
	Hints   []KeyHint
	Width   int
}

func New() Model {
	return Model{
		Mode: "NORMAL",
		Hints: []KeyHint{
			{Key: "↑↓", Desc: "move"},
			{Key: "enter", Desc: "select"},
			{Key: "tab", Desc: "panel"},
			{Key: "ctrl+k", Desc: "commands"},
			{Key: "?", Desc: "help"},
			{Key: "q", Desc: "quit"},
		},
	}
}

func (m Model) View() string {
	t := theme.Current()

	// Mode badge
	modeStyle := t.Key
	if m.Mode == "RUNNING" {
		modeStyle = t.BadgeInfo
	} else if m.Mode == "COMMAND" {
		modeStyle = t.BadgeWarning
	}
	left := modeStyle.Render(" " + m.Mode + " ")

	if m.Message != "" {
		left += "  " + t.Body.Render(m.Message)
	}

	// Right hints
	var hintParts []string
	for _, h := range m.Hints {
		hintParts = append(hintParts, t.Key.Render(h.Key)+" "+t.KeyDesc.Render(h.Desc))
	}
	right := strings.Join(hintParts, "  ")

	// FooterBox has Padding(0, 1), so inner width is m.Width - 2
	innerW := m.Width - 2
	if innerW < 10 {
		innerW = 10
	}

	leftLen := lipgloss.Width(left)
	rightLen := lipgloss.Width(right)
	gap := innerW - leftLen - rightLen
	if gap < 1 {
		gap = 1
		// If narrow, drop hints
		if innerW < 70 {
			right = ""
		}
	}

	content := left + strings.Repeat(" ", gap) + right
	return t.FooterBox.Width(m.Width).Render(content)
}
