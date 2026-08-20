package governance

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/homiakus/agctl/internal/backup"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/permissions"
	"github.com/homiakus/agctl/internal/securityaudit"
	"github.com/homiakus/agctl/internal/tui/theme"
)

type GovSection int

const (
	SecPermissions GovSection = iota
	SecSecurity
	SecBackups
)

type Model struct {
	Paths       paths.Paths
	ActiveSec   GovSection
	PermAudit   permissions.Audit
	AuditReport model.SecurityReport
	BackupList  []string
	Cursor      int
	Width       int
	Height      int
}

func New(p paths.Paths) Model {
	m := Model{
		Paths:     p,
		ActiveSec: SecPermissions,
	}
	m.Refresh()
	return m
}

func (m *Model) Refresh() {
	pa, _ := permissions.AuditSettings(m.Paths)
	m.PermAudit = pa

	rep, _ := securityaudit.Audit(m.Paths, "")
	m.AuditReport = rep

	bks, _ := backup.List(m.Paths)
	m.BackupList = bks
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "h", "left":
			if m.ActiveSec > 0 {
				m.ActiveSec--
				m.Cursor = 0
			}
		case "l", "right":
			if m.ActiveSec < 2 {
				m.ActiveSec++
				m.Cursor = 0
			}
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			m.Cursor++
		case " ", "enter":
			if m.ActiveSec == SecPermissions {
				if m.PermAudit.ToolPermission == "always-proceed" {
					_ = permissions.Apply(m.Paths, "safe")
				} else {
					_ = permissions.Apply(m.Paths, "autonomous")
				}
				m.Refresh()
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	t := theme.Current()
	var sb strings.Builder

	sb.WriteString(t.MicroLabel.Render("GOVERNANCE, SECURITY & RECOVERY") + "\n\n")

	tab1 := " 1. Permissions "
	tab2 := " 2. Security "
	tab3 := " 3. Backups "
	switch m.ActiveSec {
	case SecPermissions:
		tab1 = t.Key.Render(tab1)
		tab2 = t.KeyDesc.Render(tab2)
		tab3 = t.KeyDesc.Render(tab3)
	case SecSecurity:
		tab1 = t.KeyDesc.Render(tab1)
		tab2 = t.Key.Render(tab2)
		tab3 = t.KeyDesc.Render(tab3)
	case SecBackups:
		tab1 = t.KeyDesc.Render(tab1)
		tab2 = t.KeyDesc.Render(tab2)
		tab3 = t.Key.Render(tab3)
	}
	sb.WriteString(tab1 + " " + tab2 + " " + tab3 + "\n\n")

	switch m.ActiveSec {
	case SecPermissions:
		sb.WriteString(t.Bold.Render("Permission Execution Policies") + "\n\n")
		st := "Safe (prompts required for all operations)"
		if m.PermAudit.ToolPermission == "always-proceed" {
			st = "Autonomous (unattended safe sandbox execution)"
		}
		sb.WriteString(fmt.Sprintf("  %s %s\n", t.ItemNormal.Render("Tool Policy:"), t.Bold.Render(st)))
		sb.WriteString(fmt.Sprintf("  %s %s\n\n", t.ItemNormal.Render("Review Policy:"), t.Bold.Render(m.PermAudit.ArtifactReview)))
		sb.WriteString(t.Muted.Render("  Press Space or Enter to toggle mode.\n"))

	case SecSecurity:
		sb.WriteString(t.Bold.Render(fmt.Sprintf("Security Score: %d / 100", m.AuditReport.Score)) + "\n\n")
		if len(m.AuditReport.Findings) == 0 {
			sb.WriteString(t.BadgeSuccess.Render("  ● No security vulnerabilities or unsafe permission leaks detected.\n"))
		} else {
			for i, f := range m.AuditReport.Findings {
				if i >= 4 {
					break
				}
				badge := t.BadgeWarning.Render("[WARN]")
				if f.Severity == "critical" || f.Severity == "high" {
					badge = t.BadgeError.Render("[" + strings.ToUpper(f.Severity) + "]")
				}
				sb.WriteString(fmt.Sprintf("  %s %s: %s\n", badge, f.ID, f.Message))
			}
		}

	case SecBackups:
		sb.WriteString(t.Bold.Render(fmt.Sprintf("Snapshots (%d total)", len(m.BackupList))) + "\n\n")
		if len(m.BackupList) == 0 {
			sb.WriteString(t.Muted.Render("  No backups created yet.\n"))
		} else {
			for i, b := range m.BackupList {
				if i >= 4 {
					break
				}
				sb.WriteString(fmt.Sprintf("  • %s\n", t.Bold.Render(b)))
			}
		}
	}

	sb.WriteString("\n" + t.Muted.Render("← / → switch tabs • Tab next panel"))
	return sb.String()
}
