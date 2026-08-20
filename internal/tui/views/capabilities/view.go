package capabilities

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/homiakus/agctl/internal/mcp"
	"github.com/homiakus/agctl/internal/mcpprobe"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/skills"
	"github.com/homiakus/agctl/internal/tui/theme"
)

type SubCategory int

const (
	SubSkills SubCategory = iota
	SubMCP
	SubPacks
)

type Model struct {
	Paths       paths.Paths
	ActiveSub   SubCategory
	SkillsList  []skills.Item
	MCPNames    []string
	ProbeReport []mcpprobe.Report
	Cursor      int
	Probing     bool
	Width       int
	Height      int
}

type ProbeDoneMsg struct {
	Report []mcpprobe.Report
	Err    error
}

func New(p paths.Paths) Model {
	m := Model{
		Paths:     p,
		ActiveSub: SubSkills,
	}
	m.Refresh()
	return m
}

func (m *Model) Refresh() {
	sks, _ := skills.List(m.Paths)
	m.SkillsList = sks

	names, _ := mcp.Names(m.Paths, "")
	m.MCPNames = names

	if m.Cursor >= len(m.currentItems()) {
		m.Cursor = 0
	}
}

func (m Model) currentItems() []string {
	switch m.ActiveSub {
	case SubSkills:
		var items []string
		for _, s := range m.SkillsList {
			items = append(items, s.Name)
		}
		return items
	case SubMCP:
		return m.MCPNames
	case SubPacks:
		return []string{
			"Superpowers (Official Charm & Worktree Skills)",
			"Addy Agent Skills (Full Engineering Pack)",
			"Google Gemini Skills (Interactions, Live, Multimodal)",
			"No AI Slop (Editorial & Voice Preservation)",
		}
	}
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "h", "left":
			if m.ActiveSub > 0 {
				m.ActiveSub--
				m.Cursor = 0
			}
		case "l", "right":
			if m.ActiveSub < 2 {
				m.ActiveSub++
				m.Cursor = 0
			}
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.currentItems())-1 {
				m.Cursor++
			}
		case "r":
			if m.ActiveSub == SubMCP && !m.Probing {
				m.Probing = true
				p := m.Paths
				return m, func() tea.Msg {
					rep := mcpprobe.ProbeAll(p, "", 8*time.Second)
					return ProbeDoneMsg{Report: rep}
				}
			}
		}
	case ProbeDoneMsg:
		m.Probing = false
		m.ProbeReport = msg.Report
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	t := theme.Current()
	var sb strings.Builder

	// Top Sub-tabs
	sb.WriteString(t.MicroLabel.Render("CAPABILITIES & RUNTIME EXTENSIONS") + "\n\n")

	tab1 := " 1. Skills "
	tab2 := " 2. MCP Servers "
	tab3 := " 3. Skill Packs "
	switch m.ActiveSub {
	case SubSkills:
		tab1 = t.Key.Render(tab1)
		tab2 = t.KeyDesc.Render(tab2)
		tab3 = t.KeyDesc.Render(tab3)
	case SubMCP:
		tab1 = t.KeyDesc.Render(tab1)
		tab2 = t.Key.Render(tab2)
		tab3 = t.KeyDesc.Render(tab3)
	case SubPacks:
		tab1 = t.KeyDesc.Render(tab1)
		tab2 = t.KeyDesc.Render(tab2)
		tab3 = t.Key.Render(tab3)
	}
	sb.WriteString(tab1 + " " + tab2 + " " + tab3 + "\n\n")

	items := m.currentItems()
	if len(items) == 0 {
		sb.WriteString(t.Muted.Render("  No items installed.\n"))
	} else {
		maxShow := 8
		start := 0
		if m.Cursor >= maxShow {
			start = m.Cursor - maxShow + 1
		}
		end := start + maxShow
		if end > len(items) {
			end = len(items)
		}

		for i := start; i < end; i++ {
			it := items[i]
			isSel := i == m.Cursor
			prefix := "  "
			titleStyle := t.ItemNormal
			if isSel {
				prefix = t.Symbols.ArrowRight + " "
				titleStyle = t.ItemActive
			}
			sb.WriteString(prefix + titleStyle.Render(it) + "\n")
		}
	}

	sb.WriteString("\n")
	if len(items) > 0 && m.Cursor < len(items) {
		selectedName := items[m.Cursor]
		sb.WriteString(t.MicroLabel.Render("DETAILS: ") + t.Bold.Render(selectedName) + "\n")

		switch m.ActiveSub {
		case SubSkills:
			for _, sk := range m.SkillsList {
				if sk.Name == selectedName {
					sb.WriteString(t.Muted.Render("Path: "+sk.Path) + "\n")
					break
				}
			}
		case SubMCP:
			if m.Probing {
				sb.WriteString(t.BadgeInfo.Render("◌ Probing latency...") + "\n")
			} else if len(m.ProbeReport) > 0 {
				for _, res := range m.ProbeReport {
					if res.Name == selectedName {
						if res.OK {
							sb.WriteString(t.BadgeSuccess.Render(fmt.Sprintf("● Healthy (Latency: %dms)", res.LatencyMS)) + "\n")
						} else {
							sb.WriteString(t.BadgeError.Render(fmt.Sprintf("✕ Unhealthy: %s", res.Error)) + "\n")
						}
					}
				}
			} else {
				sb.WriteString(t.Muted.Render("Press 'r' to probe server latency & tools") + "\n")
			}
		case SubPacks:
			sb.WriteString(t.Muted.Render("Press Enter to synchronize pack from Dashboard") + "\n")
		}
	}

	sb.WriteString("\n" + t.Muted.Render("← / → switch tabs • ↑ / ↓ select"))
	return sb.String()
}
