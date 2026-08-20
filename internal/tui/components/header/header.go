package header

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/homiakus/agctl/internal/tui/theme"
)

type Model struct {
	Title       string
	Version     string
	Breadcrumbs []string
	StatusText  string
	StatusType  string // "ok", "warn", "err", "busy"
	Width       int
}

func New(version string) Model {
	return Model{
		Title:       "agctl",
		Version:     version,
		Breadcrumbs: []string{"DASHBOARD"},
		StatusText:  "READY",
		StatusType:  "ok",
	}
}

func (m Model) View() string {
	t := theme.Current()

	// Left: App Name + Version + Breadcrumbs
	appName := t.Bold.Render(m.Title + " " + m.Version)
	crumbs := strings.Join(m.Breadcrumbs, " > ")
	crumbsStyled := t.Subtitle.Render(crumbs)
	left := appName + "  " + crumbsStyled

	// Right: Status indicator
	var statusBadge string
	switch m.StatusType {
	case "ok":
		statusBadge = t.BadgeSuccess.Render("● " + m.StatusText)
	case "warn":
		statusBadge = t.BadgeWarning.Render("! " + m.StatusText)
	case "err":
		statusBadge = t.BadgeError.Render("✕ " + m.StatusText)
	case "busy":
		statusBadge = t.BadgeInfo.Render("◌ " + m.StatusText)
	default:
		statusBadge = t.BadgeNeutral.Render("○ " + m.StatusText)
	}

	leftLen := lipgloss.Width(left)
	rightLen := lipgloss.Width(statusBadge)
	// HeaderBox has Padding(0, 1), so inner width is m.Width - 2
	innerW := m.Width - 2
	if innerW < 10 {
		innerW = 10
	}

	gap := innerW - leftLen - rightLen
	if gap < 1 {
		gap = 1
	}

	content := left + strings.Repeat(" ", gap) + statusBadge
	return t.HeaderBox.Width(m.Width).Render(content)
}
