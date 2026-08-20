package sidebar

import (
	"strings"

	"github.com/homiakus/agctl/internal/tui/theme"
)

type Item struct {
	Num   string
	Title string
	Icon  string
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
			{Num: "01", Title: "Dashboard", Icon: "●"},
			{Num: "02", Title: "Setup & Doctor", Icon: "+"},
			{Num: "03", Title: "Capabilities", Icon: "◆"},
			{Num: "04", Title: "Autonomy & Loop", Icon: "↺"},
			{Num: "05", Title: "Governance & Ops", Icon: "■"},
		},
	}
}

func (m Model) View() string {
	t := theme.Current()
	var sb strings.Builder

	// Header
	sb.WriteString(t.SidebarTitle.Render("◈ NAVIGATION") + "\n\n")

	for i, item := range m.Items {
		isSel := i == m.Selected
		numStr := t.SidebarItemNumber.Render(item.Num)

		if isSel {
			prefix := t.Symbols.ArrowRight + " "
			title := t.SidebarItemActive.Render(item.Icon + " " + item.Title)
			sb.WriteString(prefix + numStr + " " + title + "\n")
		} else {
			prefix := "  "
			title := t.SidebarItemNormal.Render(item.Icon + " " + item.Title)
			sb.WriteString(prefix + numStr + " " + title + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n" + t.Muted.Render("1..5 Jump • Tab Move"))

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
