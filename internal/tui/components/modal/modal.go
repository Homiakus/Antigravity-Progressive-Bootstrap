package modal

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/homiakus/agctl/internal/tui/theme"
)

type Model struct {
	Title         string
	Message       string
	ConfirmLabel  string
	CancelLabel   string
	IsDestructive bool
	SelectedBtn   int // 0: Cancel, 1: Confirm
	Visible       bool
	Width         int
	ActionID      string
}

func New() Model {
	return Model{
		ConfirmLabel: "Confirm",
		CancelLabel:  "Cancel",
		SelectedBtn:  0,
		Visible:      false,
		Width:        50,
	}
}

func (m *Model) Show(actionID, title, msg string, destructive bool) {
	m.ActionID = actionID
	m.Title = title
	m.Message = msg
	m.IsDestructive = destructive
	m.SelectedBtn = 0 // Default to safe Cancel
	m.Visible = true
}

func (m *Model) Close() {
	m.Visible = false
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.Visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.Close()
			return m, nil
		case "left", "h", "right", "l", "tab":
			if m.SelectedBtn == 0 {
				m.SelectedBtn = 1
			} else {
				m.SelectedBtn = 0
			}
			return m, nil
		}
	}
	return m, nil
}

func (m Model) View() string {
	if !m.Visible {
		return ""
	}
	t := theme.Current()

	var sb strings.Builder
	titleStyle := t.Title
	if m.IsDestructive {
		titleStyle = t.BadgeError
	}
	sb.WriteString(titleStyle.Render(m.Title) + "\n\n")
	sb.WriteString(t.Body.Render(m.Message) + "\n\n")

	cancelStyle := t.KeyDesc
	confirmStyle := t.KeyDesc
	if m.SelectedBtn == 0 {
		cancelStyle = t.Key
	} else {
		if m.IsDestructive {
			confirmStyle = t.BadgeError
		} else {
			confirmStyle = t.BadgeSuccess
		}
	}

	btnCancel := cancelStyle.Render("[ " + m.CancelLabel + " ]")
	btnConfirm := confirmStyle.Render("[ " + m.ConfirmLabel + " ]")

	sb.WriteString("  " + btnCancel + "    " + btnConfirm)

	boxWidth := m.Width
	if boxWidth > 60 {
		boxWidth = 60
	} else if boxWidth < 35 {
		boxWidth = 35
	}

	return t.ModalBox.Width(boxWidth).Render(sb.String())
}
