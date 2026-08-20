package help

import (
	"strings"

	"github.com/homiakus/agctl/internal/tui/theme"
)

type HelpSection struct {
	Title string
	Items [][2]string // [key, description]
}

type Model struct {
	Visible  bool
	Sections []HelpSection
	Width    int
}

func New() Model {
	return Model{
		Visible: false,
		Width:   60,
		Sections: []HelpSection{
			{
				Title: "NAVIGATION",
				Items: [][2]string{
					{"1..5", "Switch top-level section"},
					{"↑ ↓ / j k", "Move cursor in active list"},
					{"Tab / S-Tab", "Cycle focus between panels"},
					{"PageUp/Down", "Quick scroll"},
				},
			},
			{
				Title: "ACTIONS",
				Items: [][2]string{
					{"Enter", "Open / Execute / Select item"},
					{"Space", "Toggle option / Checkbox"},
					{"d", "Inspect details / Raw JSON"},
					{"r", "Refresh / Live probe"},
					{"x", "Cancel / Terminate running task"},
				},
			},
			{
				Title: "GLOBAL",
				Items: [][2]string{
					{"Ctrl+K / :", "Command palette & quick launcher"},
					{"/", "Fuzzy search & filter active list"},
					{"?", "Toggle this help cheatsheet"},
					{"Esc / q", "Back / Close modal / Quit"},
				},
			},
		},
	}
}

func (m *Model) Toggle() {
	m.Visible = !m.Visible
}

func (m Model) View() string {
	if !m.Visible {
		return ""
	}
	t := theme.Current()

	var sb strings.Builder
	sb.WriteString(t.Title.Render("Keyboard Shortcuts & Help") + "\n\n")

	for _, sec := range m.Sections {
		sb.WriteString(t.MicroLabel.Render(sec.Title) + "\n")
		for _, item := range sec.Items {
			k := t.Key.Render(item[0])
			d := t.KeyDesc.Render(item[1])
			sb.WriteString("  " + k + "  " + d + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(t.Muted.Render("Press '?' or 'Esc' to dismiss"))

	boxWidth := m.Width
	if boxWidth > 65 {
		boxWidth = 65
	} else if boxWidth < 40 {
		boxWidth = 40
	}

	return t.ModalBox.Width(boxWidth).Render(sb.String())
}
