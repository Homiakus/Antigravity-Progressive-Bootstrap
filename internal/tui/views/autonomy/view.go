package autonomy

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/homiakus/agctl/internal/loop"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/router"
	"github.com/homiakus/agctl/internal/tasks"
	"github.com/homiakus/agctl/internal/tui/i18n"
	"github.com/homiakus/agctl/internal/tui/theme"
)

type Model struct {
	Paths      paths.Paths
	RouterConf model.RouterConfig
	LoopConf   model.LoopConfig
	TaskList   []model.TaskRecord
	Cursor     int
	Width      int
	Height     int
}

func New(p paths.Paths) Model {
	m := Model{
		Paths: p,
	}
	m.Refresh()
	return m
}

func (m *Model) Refresh() {
	rc, _ := router.Load(m.Paths)
	m.RouterConf = rc
	lc, _ := loop.Load(m.Paths)
	m.LoopConf = lc
	ts, _ := tasks.List(m.Paths)
	m.TaskList = ts
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
			case 0:
				m.RouterConf.Enabled = !m.RouterConf.Enabled
				_ = router.Save(m.Paths, m.RouterConf)
			case 1:
				if m.RouterConf.Mode == router.ModeTransparent {
					m.RouterConf.Mode = router.ModeBalanced
				} else {
					m.RouterConf.Mode = router.ModeTransparent
				}
				_ = router.Save(m.Paths, m.RouterConf)
			case 2:
				m.LoopConf.Enabled = !m.LoopConf.Enabled
				_ = loop.Save(m.Paths, m.LoopConf)
			case 3:
				if m.LoopConf.PermissionMode == loop.PermissionUnrestricted {
					m.LoopConf.PermissionMode = loop.PermissionGuarded
				} else {
					m.LoopConf.PermissionMode = loop.PermissionUnrestricted
				}
				_ = loop.Save(m.Paths, m.LoopConf)
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	t := theme.Current()
	var sb strings.Builder

	sb.WriteString(t.MicroLabel.Render(i18n.T("auto_title")) + "\n\n")

	items := []struct {
		Label string
		Value string
		Desc  string
	}{
		{Label: i18n.T("auto_router"), Value: fmt.Sprintf("%v", m.RouterConf.Enabled), Desc: i18n.T("auto_router_d")},
		{Label: i18n.T("auto_rmode"), Value: m.RouterConf.Mode, Desc: i18n.T("auto_rmode_d")},
		{Label: i18n.T("auto_loop"), Value: fmt.Sprintf("%v", m.LoopConf.Enabled), Desc: i18n.T("auto_loop_d")},
		{Label: i18n.T("auto_lperm"), Value: m.LoopConf.PermissionMode, Desc: i18n.T("auto_lperm_d")},
	}

	for i, item := range items {
		isSel := i == m.Cursor
		prefix := "  "
		if isSel {
			prefix = t.Symbols.ArrowRight + " "
		}
		lbl := t.ItemNormal.Render(item.Label + ": ")
		val := t.Bold.Render(item.Value)
		if isSel {
			lbl = t.ItemActive.Render(item.Label + ": ")
		}
		sb.WriteString(prefix + lbl + val + "\n")
		sb.WriteString(t.Muted.Render("    "+item.Desc) + "\n\n")
	}

	tasksCountTxt := fmt.Sprintf("%d задач", len(m.TaskList))
	if i18n.CurrentLanguage() == i18n.LangEN {
		tasksCountTxt = fmt.Sprintf("%d tasks", len(m.TaskList))
	}
	sb.WriteString(t.MicroLabel.Render(i18n.T("auto_queue")) + t.Bold.Render(tasksCountTxt) + "\n")
	if len(m.TaskList) == 0 {
		sb.WriteString(t.Muted.Render("  "+i18n.T("auto_no_tasks")+"\n"))
	} else {
		for i, tk := range m.TaskList {
			if i >= 3 {
				break
			}
			st := t.BadgeNeutral.Render(tk.Status)
			if tk.Status == tasks.StatusRunning {
				st = t.BadgeInfo.Render("◌ " + i18n.T("status_running"))
			} else if tk.Status == tasks.StatusSucceeded {
				st = t.BadgeSuccess.Render("● " + i18n.T("status_healthy"))
			}
			sb.WriteString(fmt.Sprintf("  • %s %s\n", t.Bold.Render(tk.ID), st))
		}
	}

	sb.WriteString("\n" + t.Muted.Render(i18n.T("auto_footer")))
	return sb.String()
}
