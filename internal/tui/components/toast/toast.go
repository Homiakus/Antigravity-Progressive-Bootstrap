package toast

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/homiakus/agctl/internal/tui/theme"
)

type ToastType int

const (
	TypeInfo ToastType = iota
	TypeSuccess
	TypeWarning
	TypeError
)

type Model struct {
	Message   string
	Type      ToastType
	ExpiresAt time.Time
}

type ClearToastMsg struct{}

func New() Model {
	return Model{}
}

func (m *Model) Show(msg string, t ToastType, duration time.Duration) tea.Cmd {
	m.Message = msg
	m.Type = t
	m.ExpiresAt = time.Now().Add(duration)
	return tea.Tick(duration, func(t time.Time) tea.Msg {
		return ClearToastMsg{}
	})
}

func (m *Model) Clear() {
	m.Message = ""
}

func (m Model) View() string {
	if m.Message == "" || time.Now().After(m.ExpiresAt) {
		return ""
	}
	t := theme.Current()
	switch m.Type {
	case TypeSuccess:
		return t.BadgeSuccess.Render(t.Symbols.Checkmark + " " + m.Message)
	case TypeWarning:
		return t.BadgeWarning.Render(t.Symbols.Warning + " " + m.Message)
	case TypeError:
		return t.BadgeError.Render(t.Symbols.Cross + " " + m.Message)
	default:
		return t.BadgeInfo.Render(t.Symbols.Bullet + " " + m.Message)
	}
}
