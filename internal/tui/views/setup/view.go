package setup

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/homiakus/agctl/internal/doctor"
	"github.com/homiakus/agctl/internal/hooks"
	"github.com/homiakus/agctl/internal/installer"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/tui/theme"
)

type SetupOption struct {
	ID    string
	Title string
	Desc  string
}

type Model struct {
	Paths       paths.Paths
	Options     []SetupOption
	Cursor      int
	Running     bool
	ActiveOp    string
	Width       int
	Height      int
}

type OpCompletedMsg struct {
	OpID    string
	Output  []string
	Success bool
	Err     error
}

func New(p paths.Paths) Model {
	return Model{
		Paths: p,
		Options: []SetupOption{
			{ID: "rec", Title: "Recommended Install", Desc: "Install embedded skills, hooks & core binary"},
			{ID: "full", Title: "Full Stable Setup", Desc: "Install all packs, sidecars and probe all MCP"},
			{ID: "doctor", Title: "Doctor Diagnostics", Desc: "Run comprehensive environment check"},
			{ID: "prereq", Title: "Check Prerequisites", Desc: "Verify Git, Go, Node.js, Ripgrep"},
			{ID: "self", Title: "Install/Update agctl Binary & Hooks", Desc: "Compile self into bin and register hooks"},
		},
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.Running {
			return m, nil
		}
		switch msg.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.Options)-1 {
				m.Cursor++
			}
		case "enter":
			return m.StartSelected()
		}
	case OpCompletedMsg:
		m.Running = false
		m.ActiveOp = ""
		return m, nil
	}
	return m, nil
}

func (m *Model) StartSelected() (Model, tea.Cmd) {
	if m.Cursor < 0 || m.Cursor >= len(m.Options) {
		return *m, nil
	}
	opt := m.Options[m.Cursor]
	m.Running = true
	m.ActiveOp = opt.Title

	p := m.Paths
	return *m, func() tea.Msg {
		var out []string
		var err error

		switch opt.ID {
		case "rec":
			r, e := installer.Recommended(p, false)
			err = e
			if e == nil {
				out = append(out, fmt.Sprintf("Installed binary: %s", r.InstalledBinary))
				out = append(out, fmt.Sprintf("Skill packs: %v", r.SkillPackCounts))
			}
		case "full":
			r, e := installer.Full(p, false)
			err = e
			if e == nil {
				out = append(out, fmt.Sprintf("Installed binary: %s", r.InstalledBinary))
				out = append(out, fmt.Sprintf("Skill packs: %v", r.SkillPackCounts))
				if len(r.MCPWarnings) > 0 {
					out = append(out, fmt.Sprintf("Warnings: %v", r.MCPWarnings))
				}
			}
		case "doctor":
			doc := doctor.RunAdvanced(p, "", false)
			if doc.HasErrors() {
				out = append(out, "[ERROR] Doctor found configuration/environment issues")
			} else {
				out = append(out, "[OK] Doctor environment check PASSED")
			}
			for _, finding := range doc.Findings {
				out = append(out, fmt.Sprintf("[%s] %s: %s", finding.Level, finding.Area, finding.Message))
			}
		case "prereq":
			tools := []string{"git", "go", "node", "npm", "rg"}
			for _, tool := range tools {
				if path, e := exec.LookPath(tool); e == nil {
					out = append(out, fmt.Sprintf("[OK] %s at: %s", tool, path))
				} else {
					out = append(out, fmt.Sprintf("[MISSING] %s not in PATH", tool))
				}
			}
		case "self":
			b, e := installer.InstallSelf(p)
			err = e
			if e == nil {
				out = append(out, fmt.Sprintf("Binary installed: %s", b))
				if e2 := hooks.Install(p, b); e2 != nil {
					out = append(out, fmt.Sprintf("Hooks warning: %v", e2))
				} else {
					out = append(out, "Lifecycle hooks registered successfully.")
				}
			}
		}

		return OpCompletedMsg{
			OpID:    opt.ID,
			Output:  out,
			Success: err == nil,
			Err:     err,
		}
	}
}

func (m Model) View() string {
	t := theme.Current()
	var sb strings.Builder

	sb.WriteString(t.MicroLabel.Render("SETUP, PREREQUISITES & DOCTOR DIAGNOSTICS") + "\n\n")

	if m.Running {
		sb.WriteString(t.BadgeInfo.Render("◌ Running: "+m.ActiveOp) + "\n\n")
	}

	for i, opt := range m.Options {
		isSel := i == m.Cursor
		prefix := "  "
		titleStyle := t.ItemNormal
		if isSel {
			prefix = t.Symbols.ArrowRight + " "
			titleStyle = t.ItemActive
		}
		title := titleStyle.Render(opt.Title)
		desc := t.Muted.Render("    " + opt.Desc)
		sb.WriteString(prefix + title + "\n" + desc + "\n\n")
	}

	sb.WriteString(t.Muted.Render("Press Enter to execute • Results stream to Live Console"))
	return sb.String()
}
