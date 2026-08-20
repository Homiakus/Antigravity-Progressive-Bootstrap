package sidebar

import (
	"strings"

	"github.com/homiakus/agctl/internal/tui/i18n"
	"github.com/homiakus/agctl/internal/tui/theme"
)

type Item struct {
	Num     string
	KeyText string
	Icon    string
}

type Model struct {
	Items    []Item
	Selected int
	Focused  bool
	Width    int
	Height   int
}

func New() Model {
	return Model{
		Selected: 0,
		Focused:  true,
		Items: []Item{
			{Num: "01", KeyText: "sec_dashboard", Icon: "●"},
			{Num: "02", KeyText: "sec_setup", Icon: "+"},
			{Num: "03", KeyText: "sec_caps", Icon: "◆"},
			{Num: "04", KeyText: "sec_autonomy", Icon: "↺"},
			{Num: "05", KeyText: "sec_governance", Icon: "■"},
			{Num: "06", KeyText: "sec_settings", Icon: "⚙"},
		},
	}
}

func (m Model) View() string {
	t := theme.Current()
	var sb strings.Builder

	// Header
	sb.WriteString(t.SidebarTitle.Render("◈ "+i18n.T("nav_title")) + "\n\n")

	for i, item := range m.Items {
		isSel := i == m.Selected
		numStr := t.SidebarItemNumber.Render(item.Num)
		titleText := i18n.T(item.KeyText)

		if isSel {
			prefix := t.Symbols.ArrowRight + " "
			title := t.SidebarItemActive.Render(item.Icon + " " + titleText)
			sb.WriteString(prefix + numStr + " " + title + "\n")
		} else {
			prefix := "  "
			title := t.SidebarItemNormal.Render(item.Icon + " " + titleText)
			sb.WriteString(prefix + numStr + " " + title + "\n")
		}
		sb.WriteString("\n")
	}

	hintText := "1..6 Перейти • Tab Окно"
	if i18n.CurrentLanguage() == i18n.LangEN {
		hintText = "1..6 Jump • Tab Move"
	}
	sb.WriteString(t.Muted.Render(hintText))

	boxStyle := t.SidebarBox
	if m.Focused {
		boxStyle = t.SidebarBoxActive
	}

	innerWidth := m.Width - 4
	if innerWidth < 10 {
		innerWidth = 10
	}
	innerHeight := m.Height - 2
	if innerHeight < 5 {
		innerHeight = 5
	}

	return boxStyle.Width(innerWidth).Height(innerHeight).Render(sb.String())
}
