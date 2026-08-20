package dashboard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/homiakus/agctl/internal/doctor"
	"github.com/homiakus/agctl/internal/loop"
	"github.com/homiakus/agctl/internal/mcp"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/router"
	"github.com/homiakus/agctl/internal/skills"
	"github.com/homiakus/agctl/internal/tasks"
	"github.com/homiakus/agctl/internal/tui/i18n"
	"github.com/homiakus/agctl/internal/tui/theme"
)

type QuickAction struct {
	ID       string
	KeyTitle string
	KeyDesc  string
}

type Model struct {
	Paths        paths.Paths
	RouterState  string
	LoopState    string
	SkillsCount  int
	MCPCount     int
	TasksCount   int
	DoctorStatus string
	QuickActions []QuickAction
	Cursor       int
	Width        int
	Height       int
}

func New(p paths.Paths) Model {
	m := Model{
		Paths: p,
		QuickActions: []QuickAction{
			{ID: "install-rec", KeyTitle: "dash_act_rec", KeyDesc: "dash_act_rec_d"},
			{ID: "install-full", KeyTitle: "dash_act_full", KeyDesc: "dash_act_full_d"},
			{ID: "doctor", KeyTitle: "dash_act_doc", KeyDesc: "dash_act_doc_d"},
			{ID: "probe-mcp", KeyTitle: "dash_act_probe", KeyDesc: "dash_act_probe_d"},
			{ID: "sync-skills", KeyTitle: "dash_act_sync", KeyDesc: "dash_act_sync_d"},
		},
	}
	m.Refresh()
	return m
}

func (m *Model) Refresh() {
	rc, _ := router.Load(m.Paths)
	lc, _ := loop.Load(m.Paths)
	m.RouterState = fmt.Sprintf("%v (%s)", rc.Enabled, rc.Mode)
	m.LoopState = fmt.Sprintf("%v (%s, max=%d)", lc.Enabled, lc.PermissionMode, lc.MaxExecutions)

	skList, _ := skills.List(m.Paths)
	m.SkillsCount = len(skList)

	mcpNames, _ := mcp.Names(m.Paths, "")
	m.MCPCount = len(mcpNames)

	ts, _ := tasks.List(m.Paths)
	m.TasksCount = len(ts)

	docRep := doctor.Run(m.Paths, "")
	if docRep.HasErrors() {
		m.DoctorStatus = "status_errors"
	} else {
		m.DoctorStatus = "status_healthy"
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
			if m.Cursor < len(m.QuickActions)-1 {
				m.Cursor++
			}
		}
	}
	return m, nil
}

func (m Model) SelectedAction() *QuickAction {
	if m.Cursor >= 0 && m.Cursor < len(m.QuickActions) {
		return &m.QuickActions[m.Cursor]
	}
	return nil
}

func (m Model) View() string {
	t := theme.Current()
	var sb strings.Builder

	// Top Section: Metrics
	sb.WriteString(t.MicroLabel.Render(i18n.T("dash_metrics")) + "\n\n")

	sb.WriteString(fmt.Sprintf("  %s %s\n", t.ItemNormal.Render(i18n.T("dash_router")), t.Bold.Render(m.RouterState)))
	sb.WriteString(fmt.Sprintf("  %s %s\n", t.ItemNormal.Render(i18n.T("dash_loop")), t.Bold.Render(m.LoopState)))

	statusText := i18n.T(m.DoctorStatus)
	statusStyle := t.BadgeSuccess
	if m.DoctorStatus == "status_errors" {
		statusStyle = t.BadgeError
	}
	sb.WriteString(fmt.Sprintf("  %s %s\n", t.ItemNormal.Render(i18n.T("dash_health")), statusStyle.Render("● "+statusText)))

	invStr := fmt.Sprintf("%d Skills • %d MCP • %d Tasks", m.SkillsCount, m.MCPCount, m.TasksCount)
	if i18n.CurrentLanguage() == i18n.LangRU {
		invStr = fmt.Sprintf("%d Навыков • %d MCP серверов • %d Задач", m.SkillsCount, m.MCPCount, m.TasksCount)
	}
	sb.WriteString(fmt.Sprintf("  %s %s\n\n", t.ItemNormal.Render(i18n.T("dash_inventory")), t.Bold.Render(invStr)))

	// Middle Section: Quick Actions
	sb.WriteString(t.MicroLabel.Render(i18n.T("dash_actions")) + "\n\n")
	for i, act := range m.QuickActions {
		isSel := i == m.Cursor
		prefix := "  "
		titleStyle := t.ItemNormal
		if isSel {
			prefix = t.Symbols.ArrowRight + " "
			titleStyle = t.ItemActive
		}

		title := titleStyle.Render(i18n.T(act.KeyTitle))
		desc := t.Muted.Render(" — " + i18n.T(act.KeyDesc))
		sb.WriteString(prefix + title + desc + "\n")
	}

	sb.WriteString("\n" + t.Muted.Render(i18n.T("dash_footer")))
	return sb.String()
}
