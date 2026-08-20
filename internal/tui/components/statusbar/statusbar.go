package statusbar

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/homiakus/agctl/internal/tui/i18n"
	"github.com/homiakus/agctl/internal/tui/theme"
)

type KeyHint struct {
	Key  string
	Desc string
}

type Model struct {
	Mode    string // "NORMAL", "SEARCH", "COMMAND", "RUNNING"
	Message string
	Hints   []KeyHint
	Width   int
}

func New() Model {
	return Model{
		Mode: "NORMAL",
		Hints: []KeyHint{
			{Key: "↑↓", Desc: i18n.T("hint_move")},
			{Key: "enter", Desc: i18n.T("hint_select")},
			{Key: "tab", Desc: i18n.T("hint_panel")},
			{Key: "ctrl+k", Desc: i18n.T("hint_commands")},
			{Key: "?", Desc: i18n.T("hint_help")},
			{Key: "q", Desc: i18n.T("hint_quit")},
		},
	}
}

func (m Model) View() string {
	t := theme.Current()

	// Mode badge with translation
	modeDisplay := m.Mode
	if m.Mode == "NORMAL" {
		modeDisplay = i18n.T("mode_normal")
	} else if m.Mode == "RUNNING" {
		modeDisplay = i18n.T("status_running")
	}

	modeStyle := t.Key
	if m.Mode == "RUNNING" {
		modeStyle = t.BadgeInfo
	} else if m.Mode == "COMMAND" {
		modeStyle = t.BadgeWarning
	}
	left := modeStyle.Render(" " + modeDisplay + " ")

	if m.Message != "" {
		left += "  " + t.Body.Render(m.Message)
	}

	// Right hints
	var hintParts []string
	for _, h := range m.Hints {
		hintParts = append(hintParts, t.Key.Render(h.Key)+" "+t.KeyDesc.Render(h.Desc))
	}
	right := strings.Join(hintParts, "  ")

	innerW := m.Width - 2
	if innerW < 10 {
		innerW = 10
	}

	leftLen := lipgloss.Width(left)
	rightLen := lipgloss.Width(right)
	gap := innerW - leftLen - rightLen
	if gap < 1 {
		gap = 1
		if innerW < 75 {
			right = ""
		}
	}

	content := left + strings.Repeat(" ", gap) + right
	return t.FooterBox.Width(m.Width).Render(content)
}
