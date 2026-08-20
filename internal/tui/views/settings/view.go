package settings

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/tui/i18n"
	"github.com/homiakus/agctl/internal/tui/theme"
)

type Model struct {
	Paths    paths.Paths
	Settings i18n.Settings
	Cursor   int
	Status   string
	Width    int
	Height   int
}

func New(p paths.Paths) Model {
	s := i18n.LoadSettings(p.AppRoot)
	return Model{
		Paths:    p,
		Settings: s,
		Cursor:   0,
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < 3 {
				m.Cursor++
			}
		case " ", "enter":
			switch m.Cursor {
			case 0: // Toggle Language: RU <-> EN
				if m.Settings.Language == i18n.LangRU {
					m.Settings.Language = i18n.LangEN
				} else {
					m.Settings.Language = i18n.LangRU
				}
				i18n.SetLanguage(m.Settings.Language)
				_ = i18n.SaveSettings(m.Paths.AppRoot, m.Settings)
				if m.Settings.Language == i18n.LangRU {
					m.Status = "Язык переключен на Русский"
				} else {
					m.Status = "Language switched to English"
				}

			case 1: // Toggle Theme: dark <-> light
				if m.Settings.Theme == "dark" {
					m.Settings.Theme = "light"
				} else {
					m.Settings.Theme = "dark"
				}
				_ = i18n.SaveSettings(m.Paths.AppRoot, m.Settings)
				m.Status = "Theme: " + m.Settings.Theme

			case 2: // Toggle Accent: cyan -> purple -> green -> cyan
				switch m.Settings.Accent {
				case "cyan":
					m.Settings.Accent = "purple"
				case "purple":
					m.Settings.Accent = "green"
				default:
					m.Settings.Accent = "cyan"
				}
				_ = i18n.SaveSettings(m.Paths.AppRoot, m.Settings)
				m.Status = "Accent: " + m.Settings.Accent

			case 3: // Toggle Density: comfortable <-> compact
				if m.Settings.Density == "comfortable" {
					m.Settings.Density = "compact"
				} else {
					m.Settings.Density = "comfortable"
				}
				_ = i18n.SaveSettings(m.Paths.AppRoot, m.Settings)
				m.Status = "Density: " + m.Settings.Density
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	t := theme.Current()
	var sb strings.Builder

	sb.WriteString(t.MicroLabel.Render(i18n.T("set_title")) + "\n\n")

	// 1. Language Option
	isSelLang := m.Cursor == 0
	pfxLang := "  "
	lblLangStyle := t.ItemNormal
	if isSelLang {
		pfxLang = t.Symbols.ArrowRight + " "
		lblLangStyle = t.ItemActive
	}
	langVal := "[•] Русский (Russian)  [ ] English"
	if m.Settings.Language == i18n.LangEN {
		langVal = "[ ] Русский (Russian)  [•] English"
	}
	sb.WriteString(pfxLang + lblLangStyle.Render(i18n.T("set_lang")+":") + "\n")
	sb.WriteString("    " + t.Bold.Render(langVal) + "\n\n")

	// 2. Theme Option
	isSelTheme := m.Cursor == 1
	pfxTheme := "  "
	lblThemeStyle := t.ItemNormal
	if isSelTheme {
		pfxTheme = t.Symbols.ArrowRight + " "
		lblThemeStyle = t.ItemActive
	}
	themeVal := "[•] Dark (Тёмная)  [ ] Light (Светлая)"
	if m.Settings.Theme == "light" {
		themeVal = "[ ] Dark (Тёмная)  [•] Light (Светлая)"
	}
	sb.WriteString(pfxTheme + lblThemeStyle.Render(i18n.T("set_theme")+":") + "\n")
	sb.WriteString("    " + t.Bold.Render(themeVal) + "\n\n")

	// 3. Accent Color Option
	isSelAccent := m.Cursor == 2
	pfxAccent := "  "
	lblAccentStyle := t.ItemNormal
	if isSelAccent {
		pfxAccent = t.Symbols.ArrowRight + " "
		lblAccentStyle = t.ItemActive
	}
	accentVal := "● Sky Blue / Cyan"
	if m.Settings.Accent == "purple" {
		accentVal = "● Linear Purple"
	} else if m.Settings.Accent == "green" {
		accentVal = "● Emerald Green"
	}
	sb.WriteString(pfxAccent + lblAccentStyle.Render(i18n.T("set_accent")+":") + "\n")
	sb.WriteString("    " + t.Bold.Render(accentVal) + "\n\n")

	// 4. Density Option
	isSelDensity := m.Cursor == 3
	pfxDensity := "  "
	lblDensityStyle := t.ItemNormal
	if isSelDensity {
		pfxDensity = t.Symbols.ArrowRight + " "
		lblDensityStyle = t.ItemActive
	}
	densityVal := "Comfortable (Комфортная)"
	if m.Settings.Density == "compact" {
		densityVal = "Compact (Компактная)"
	}
	sb.WriteString(pfxDensity + lblDensityStyle.Render(i18n.T("set_density")+":") + "\n")
	sb.WriteString("    " + t.Bold.Render(densityVal) + "\n\n")

	// Status / Persistence note
	if m.Status != "" {
		sb.WriteString(t.BadgeSuccess.Render("✓ "+m.Status) + "\n\n")
	} else {
		sb.WriteString(t.Muted.Render(i18n.T("set_saved")) + "\n\n")
	}

	sb.WriteString(t.Muted.Render(i18n.T("set_hint")))
	return sb.String()
}
