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
	"github.com/homiakus/agctl/internal/tui/theme"
)

type QuickAction struct {
	ID    string
	Title string
	Desc  string
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
			{ID: "install-rec", Title: "Recommended Install", Desc: "Install core meta-skills & hooks"},
			{ID: "install-full", Title: "Full Stable Setup", Desc: "Install all packs and verify MCP"},
			{ID: "doctor", Title: "Run Doctor Diagnostics", Desc: "Audit environment and tools"},
			{ID: "probe-mcp", Title: "Live Probe MCP Servers", Desc: "Measure real latency & active tools"},
			{ID: "sync-skills", Title: "Sync Recommended Skills", Desc: "Update Superpowers & Gemini packs"},
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
		m.DoctorStatus = "✕ ERRORS"
	} else {
		m.DoctorStatus = "● HEALTHY"
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
	sb.WriteString(t.MicroLabel.Render("SYSTEM OVERVIEW & METRICS") + "\n\n")

	sb.WriteString(fmt.Sprintf("  %s %s\n", t.ItemNormal.Render("Router:"), t.Bold.Render(m.RouterState)))
	sb.WriteString(fmt.Sprintf("  %s %s\n", t.ItemNormal.Render("Loop:"), t.Bold.Render(m.LoopState)))
	sb.WriteString(fmt.Sprintf("  %s %s\n", t.ItemNormal.Render("Health:"), t.BadgeSuccess.Render(m.DoctorStatus)))
	sb.WriteString(fmt.Sprintf("  %s %s\n\n", t.ItemNormal.Render("Inventory:"), t.Bold.Render(fmt.Sprintf("%d Skills • %d MCP • %d Tasks", m.SkillsCount, m.MCPCount, m.TasksCount))))

	// Middle Section: Quick Actions
	sb.WriteString(t.MicroLabel.Render("QUICK ACTIONS") + "\n\n")
	for i, act := range m.QuickActions {
		isSel := i == m.Cursor
		prefix := "  "
		titleStyle := t.ItemNormal
		if isSel {
			prefix = t.Symbols.ArrowRight + " "
			titleStyle = t.ItemActive
		}

		title := titleStyle.Render(act.Title)
		desc := t.Muted.Render(" — " + act.Desc)
		sb.WriteString(prefix + title + desc + "\n")
	}

	sb.WriteString("\n" + t.Muted.Render("Enter execute • Tab panel switch"))
	return sb.String()
}
