package palette

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/homiakus/agctl/internal/tui/theme"
)

type CommandItem struct {
	ID          string
	Title       string
	Category    string
	Description string
	Shortcut    string
}

type Model struct {
	Input    textinput.Model
	Commands []CommandItem
	Filtered []CommandItem
	Cursor   int
	Visible  bool
	Width    int
	Height   int
}

func New(commands []CommandItem) Model {
	ti := textinput.New()
	ti.Placeholder = "Type a command or search..."
	ti.Prompt = " > "
	ti.Focus()

	m := Model{
		Input:    ti,
		Commands: commands,
		Filtered: commands,
		Cursor:   0,
		Visible:  false,
		Width:    60,
		Height:   15,
	}
	return m
}

func (m *Model) Open() {
	m.Visible = true
	m.Input.SetValue("")
	m.Input.Focus()
	m.filter()
	m.Cursor = 0
}

func (m *Model) Close() {
	m.Visible = false
	m.Input.Blur()
}

func (m *Model) filter() {
	q := strings.ToLower(strings.TrimSpace(m.Input.Value()))
	if q == "" {
		m.Filtered = m.Commands
		return
	}
	var res []CommandItem
	for _, c := range m.Commands {
		if strings.Contains(strings.ToLower(c.Title), q) ||
			strings.Contains(strings.ToLower(c.Category), q) ||
			strings.Contains(strings.ToLower(c.Description), q) {
			res = append(res, c)
		}
	}
	m.Filtered = res
	if m.Cursor >= len(m.Filtered) {
		m.Cursor = 0
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.Visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.Close()
			return m, nil
		case "up", "ctrl+p":
			if m.Cursor > 0 {
				m.Cursor--
			}
			return m, nil
		case "down", "ctrl+n":
			if m.Cursor < len(m.Filtered)-1 {
				m.Cursor++
			}
			return m, nil
		case "enter":
			// Handled by parent
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.Input, cmd = m.Input.Update(msg)
	m.filter()
	return m, cmd
}

func (m Model) SelectedCommand() *CommandItem {
	if len(m.Filtered) == 0 || m.Cursor >= len(m.Filtered) {
		return nil
	}
	return &m.Filtered[m.Cursor]
}

func (m Model) View() string {
	if !m.Visible {
		return ""
	}
	t := theme.Current()

	var sb strings.Builder
	sb.WriteString(t.Title.Render("Command Palette") + "\n\n")
	sb.WriteString(m.Input.View() + "\n\n")

	maxItems := 7
	if len(m.Filtered) == 0 {
		sb.WriteString(t.Muted.Render("  No matching commands found\n"))
	} else {
		start := 0
		if m.Cursor >= maxItems {
			start = m.Cursor - maxItems + 1
		}
		end := start + maxItems
		if end > len(m.Filtered) {
			end = len(m.Filtered)
		}

		for i := start; i < end; i++ {
			item := m.Filtered[i]
			isSel := i == m.Cursor

			prefix := "  "
			titleStyle := t.ItemNormal
			if isSel {
				prefix = t.Symbols.ArrowRight + " "
				titleStyle = t.ItemActive
			}

			cat := t.MicroLabel.Render("[" + item.Category + "]")
			title := titleStyle.Render(item.Title)
			desc := t.Muted.Render(" — " + item.Description)

			sb.WriteString(prefix + cat + " " + title + desc + "\n")
		}
	}

	boxWidth := m.Width
	if boxWidth > 70 {
		boxWidth = 70
	} else if boxWidth < 40 {
		boxWidth = 40
	}

	return t.ModalBox.Width(boxWidth).Render(sb.String())
}
