package header

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/homiakus/agctl/internal/tui/i18n"
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
		Breadcrumbs: []string{i18n.T("sec_dashboard")},
		StatusText:  "status_ready",
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

	// Right: Status indicator with i18n
	statusTranslated := i18n.T(m.StatusText)
	if statusTranslated == m.StatusText && !strings.HasPrefix(m.StatusText, "status_") {
		// Already custom translated string
		statusTranslated = m.StatusText
	}

	var statusBadge string
	switch m.StatusType {
	case "ok":
		statusBadge = t.BadgeSuccess.Render("● " + statusTranslated)
	case "warn":
		statusBadge = t.BadgeWarning.Render("! " + statusTranslated)
	case "err":
		statusBadge = t.BadgeError.Render("✕ " + statusTranslated)
	case "busy":
		statusBadge = t.BadgeInfo.Render("◌ " + statusTranslated)
	default:
		statusBadge = t.BadgeNeutral.Render("○ " + statusTranslated)
	}

	leftLen := lipgloss.Width(left)
	rightLen := lipgloss.Width(statusBadge)
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
