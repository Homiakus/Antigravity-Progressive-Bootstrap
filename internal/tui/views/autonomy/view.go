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

	sb.WriteString(t.MicroLabel.Render("AUTONOMY ENGINE & ORCHESTRATION") + "\n\n")

	items := []struct {
		Label string
		Value string
		Desc  string
	}{
		{Label: "Adaptive Router", Value: fmt.Sprintf("%v", m.RouterConf.Enabled), Desc: "Intercepts requests & routes to smallest capability set"},
		{Label: "Router Mode", Value: m.RouterConf.Mode, Desc: "transparent vs balanced vs maximum"},
		{Label: "Autonomous Loop", Value: fmt.Sprintf("%v", m.LoopConf.Enabled), Desc: "Evaluates goals until verification passes"},
		{Label: "Loop Permission", Value: m.LoopConf.PermissionMode, Desc: "guarded vs unrestricted"},
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

	sb.WriteString(t.MicroLabel.Render("HEADLESS TASK QUEUE: ") + t.Bold.Render(fmt.Sprintf("%d tasks", len(m.TaskList))) + "\n")
	if len(m.TaskList) == 0 {
		sb.WriteString(t.Muted.Render("  No queued tasks running.\n"))
	} else {
		for i, tk := range m.TaskList {
			if i >= 3 {
				break
			}
			st := t.BadgeNeutral.Render(tk.Status)
			if tk.Status == tasks.StatusRunning {
				st = t.BadgeInfo.Render("◌ running")
			} else if tk.Status == tasks.StatusSucceeded {
				st = t.BadgeSuccess.Render("● completed")
			}
			sb.WriteString(fmt.Sprintf("  • %s %s\n", t.Bold.Render(tk.ID), st))
		}
	}

	sb.WriteString("\n" + t.Muted.Render("Space/Enter toggle parameter • Tab switch panel"))
	return sb.String()
}
