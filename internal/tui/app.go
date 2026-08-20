package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/homiakus/agctl/internal/doctor"
	"github.com/homiakus/agctl/internal/hooks"
	"github.com/homiakus/agctl/internal/installer"
	"github.com/homiakus/agctl/internal/mcpprobe"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/skills"
	"github.com/homiakus/agctl/internal/tui/components/console"
	"github.com/homiakus/agctl/internal/tui/components/header"
	"github.com/homiakus/agctl/internal/tui/components/help"
	"github.com/homiakus/agctl/internal/tui/components/modal"
	"github.com/homiakus/agctl/internal/tui/components/palette"
	"github.com/homiakus/agctl/internal/tui/components/sidebar"
	"github.com/homiakus/agctl/internal/tui/components/statusbar"
	"github.com/homiakus/agctl/internal/tui/components/toast"
	"github.com/homiakus/agctl/internal/tui/theme"
	"github.com/homiakus/agctl/internal/tui/views/autonomy"
	"github.com/homiakus/agctl/internal/tui/views/capabilities"
	"github.com/homiakus/agctl/internal/tui/views/dashboard"
	"github.com/homiakus/agctl/internal/tui/views/governance"
	"github.com/homiakus/agctl/internal/tui/views/setup"
)

type FocusColumn int

const (
	FocusSidebar FocusColumn = iota
	FocusCenter
	FocusConsole
)

type AppModel struct {
	Paths     paths.Paths
	Focus     FocusColumn
	Sidebar   sidebar.Model
	Console   console.Model
	Header    header.Model
	StatusBar statusbar.Model
	Palette   palette.Model
	Help      help.Model
	Modal     modal.Model
	Toast     toast.Model
	DashView  dashboard.Model
	SetupView setup.Model
	CapView   capabilities.Model
	AutoView  autonomy.Model
	GovView   governance.Model
	Width     int
	Height    int
	TooSmall  bool
}

type LogMsg string

func NewApp(p paths.Paths) AppModel {
	commands := []palette.CommandItem{
		{ID: "dash", Title: "Go to Dashboard", Category: "Nav", Description: "System metrics and quick overview", Shortcut: "1"},
		{ID: "setup", Title: "Go to Setup & Doctor", Category: "Nav", Description: "Installers, diagnostics and updates", Shortcut: "2"},
		{ID: "caps", Title: "Go to Capabilities", Category: "Nav", Description: "Skills, MCP Servers and Packs", Shortcut: "3"},
		{ID: "auto", Title: "Go to Autonomy & Orchestration", Category: "Nav", Description: "Router, Loop and Headless Queue", Shortcut: "4"},
		{ID: "gov", Title: "Go to Governance & Security", Category: "Nav", Description: "Permissions, Audit, Telemetry, Backups", Shortcut: "5"},
		{ID: "install-rec", Title: "Run Recommended Install", Category: "Setup", Description: "Install core binary and essential meta-skills"},
		{ID: "install-full", Title: "Run Full Stable Setup", Category: "Setup", Description: "Install all packs and verify MCP"},
		{ID: "doctor", Title: "Run Doctor Diagnostics", Category: "Diagnostics", Description: "Complete health check"},
		{ID: "probe", Title: "Live Probe MCP Servers", Category: "MCP", Description: "Ping active MCP tools"},
		{ID: "clear-console", Title: "Clear Live Console", Category: "Console", Description: "Wipe console logs", Shortcut: "c"},
		{ID: "help", Title: "Open Help Cheatsheet", Category: "Help", Description: "Show keybindings", Shortcut: "?"},
	}

	sb := sidebar.New()
	sb.Focused = true

	con := console.New()
	con.Add("[INFO] agctl 3.2.1 initialized. 3-column desktop layout.")
	con.Add("[INFO] Tab / Shift+Tab switches panels. 1..5 direct jump.")

	return AppModel{
		Paths:     p,
		Focus:     FocusSidebar,
		Sidebar:   sb,
		Console:   con,
		Header:    header.New("3.2.1"),
		StatusBar: statusbar.New(),
		Palette:   palette.New(commands),
		Help:      help.New(),
		Modal:     modal.New(),
		Toast:     toast.New(),
		DashView:  dashboard.New(p),
		SetupView: setup.New(p),
		CapView:   capabilities.New(p),
		AutoView:  autonomy.New(p),
		GovView:   governance.New(p),
	}
}

func (m AppModel) Init() tea.Cmd {
	return nil
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.TooSmall = msg.Width < 70 || msg.Height < 18

		m.Header.Width = msg.Width
		m.StatusBar.Width = msg.Width
		m.Palette.Width = msg.Width - 10
		m.Help.Width = msg.Width - 10
		m.Modal.Width = msg.Width - 10

		panelH := msg.Height - 3
		if panelH < 5 {
			panelH = 5
		}

		// Exact 3-column width math
		sidebarW := 24
		consoleW := msg.Width / 3
		if consoleW < 32 {
			consoleW = 32
		} else if consoleW > 50 {
			consoleW = 50
		}
		centerW := msg.Width - sidebarW - consoleW - 2
		if centerW < 25 {
			centerW = 25
		}

		m.Sidebar.Width = sidebarW
		m.Sidebar.Height = panelH

		m.Console.Width = consoleW
		m.Console.Height = panelH

		m.DashView.Width = centerW
		m.DashView.Height = panelH
		m.SetupView.Width = centerW
		m.SetupView.Height = panelH
		m.CapView.Width = centerW
		m.CapView.Height = panelH
		m.AutoView.Width = centerW
		m.AutoView.Height = panelH
		m.GovView.Width = centerW
		m.GovView.Height = panelH
		return m, nil

	case LogMsg:
		m.Console.Add(string(msg))
		return m, nil

	case toast.ClearToastMsg:
		m.Toast.Clear()
		return m, nil

	case tea.KeyMsg:
		// Modal handling
		if m.Modal.Visible {
			var cmd tea.Cmd
			m.Modal, cmd = m.Modal.Update(msg)
			return m, cmd
		}

		// Help handling
		if m.Help.Visible {
			if msg.String() == "esc" || msg.String() == "?" || msg.String() == "q" {
				m.Help.Visible = false
				return m, nil
			}
		}

		// Palette handling
		if m.Palette.Visible {
			if msg.String() == "enter" {
				if sel := m.Palette.SelectedCommand(); sel != nil {
					m.Palette.Close()
					return m.ExecutePaletteCommand(sel.ID)
				}
			}
			var cmd tea.Cmd
			m.Palette, cmd = m.Palette.Update(msg)
			return m, cmd
		}

		// Global Section Switching & Keys
		switch msg.String() {
		case "ctrl+k":
			m.Palette.Open()
			return m, nil
		case "?":
			m.Help.Toggle()
			return m, nil
		case "1":
			m.setSection(0)
			return m, nil
		case "2":
			m.setSection(1)
			return m, nil
		case "3":
			m.setSection(2)
			return m, nil
		case "4":
			m.setSection(3)
			return m, nil
		case "5":
			m.setSection(4)
			return m, nil
		case "tab":
			m.Focus = (m.Focus + 1) % 3
			m.syncFocus()
			return m, nil
		case "shift+tab":
			if m.Focus == 0 {
				m.Focus = 2
			} else {
				m.Focus--
			}
			m.syncFocus()
			return m, nil
		case "q", "ctrl+c":
			return m, tea.Quit
		}

		// Panel-specific Navigation
		switch m.Focus {
		case FocusSidebar:
			switch msg.String() {
			case "up", "k":
				if m.Sidebar.Selected > 0 {
					m.setSection(m.Sidebar.Selected - 1)
				}
				return m, nil
			case "down", "j":
				if m.Sidebar.Selected < len(m.Sidebar.Items)-1 {
					m.setSection(m.Sidebar.Selected + 1)
				}
				return m, nil
			case "right", "l", "enter":
				m.Focus = FocusCenter
				m.syncFocus()
				return m, nil
			}

		case FocusConsole:
			switch msg.String() {
			case "c":
				m.Console.Clear()
				return m, nil
			case "left", "h":
				m.Focus = FocusCenter
				m.syncFocus()
				return m, nil
			}

		case FocusCenter:
			switch msg.String() {
			case "left", "h":
				m.Focus = FocusSidebar
				m.syncFocus()
				return m, nil
			case "right", "l":
				m.Focus = FocusConsole
				m.syncFocus()
				return m, nil
			}
		}
	}

	// Update Active View in Center
	switch m.Sidebar.Selected {
	case 0: // Dashboard
		var cmd tea.Cmd
		m.DashView, cmd = m.DashView.Update(msg)
		cmds = append(cmds, cmd)
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" && m.Focus == FocusCenter {
			if act := m.DashView.SelectedAction(); act != nil {
				return m.ExecutePaletteCommand(act.ID)
			}
		}
	case 1: // Setup
		var cmd tea.Cmd
		m.SetupView, cmd = m.SetupView.Update(msg)
		cmds = append(cmds, cmd)
	case 2: // Capabilities
		var cmd tea.Cmd
		m.CapView, cmd = m.CapView.Update(msg)
		cmds = append(cmds, cmd)
	case 3: // Autonomy
		var cmd tea.Cmd
		m.AutoView, cmd = m.AutoView.Update(msg)
		cmds = append(cmds, cmd)
	case 4: // Governance
		var cmd tea.Cmd
		m.GovView, cmd = m.GovView.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *AppModel) setSection(idx int) {
	if idx < 0 || idx >= len(m.Sidebar.Items) {
		return
	}
	m.Sidebar.Selected = idx
	switch idx {
	case 0:
		m.Header.Breadcrumbs = []string{"DASHBOARD"}
		m.DashView.Refresh()
		m.Console.Addf("[NAV] Switched to 01 Dashboard")
	case 1:
		m.Header.Breadcrumbs = []string{"SETUP & DOCTOR"}
		m.Console.Addf("[NAV] Switched to 02 Setup & Doctor")
	case 2:
		m.Header.Breadcrumbs = []string{"CAPABILITIES"}
		m.CapView.Refresh()
		m.Console.Addf("[NAV] Switched to 03 Capabilities")
	case 3:
		m.Header.Breadcrumbs = []string{"AUTONOMY & ORCHESTRATION"}
		m.AutoView.Refresh()
		m.Console.Addf("[NAV] Switched to 04 Autonomy & Orchestration")
	case 4:
		m.Header.Breadcrumbs = []string{"GOVERNANCE & OPS"}
		m.GovView.Refresh()
		m.Console.Addf("[NAV] Switched to 05 Governance & Safety")
	}
}

func (m *AppModel) syncFocus() {
	m.Sidebar.Focused = m.Focus == FocusSidebar
	m.Console.Focused = m.Focus == FocusConsole
	switch m.Focus {
	case FocusSidebar:
		m.StatusBar.Hints = []statusbar.KeyHint{
			{Key: "↑↓", Desc: "switch section"},
			{Key: "enter/→", Desc: "workspace"},
			{Key: "tab", Desc: "panel"},
			{Key: "ctrl+k", Desc: "commands"},
			{Key: "?", Desc: "help"},
		}
	case FocusCenter:
		m.StatusBar.Hints = []statusbar.KeyHint{
			{Key: "↑↓", Desc: "select action"},
			{Key: "enter", Desc: "execute"},
			{Key: "←", Desc: "sidebar"},
			{Key: "→", Desc: "console"},
			{Key: "tab", Desc: "panel"},
			{Key: "?", Desc: "help"},
		}
	case FocusConsole:
		m.StatusBar.Hints = []statusbar.KeyHint{
			{Key: "c", Desc: "clear logs"},
			{Key: "←", Desc: "workspace"},
			{Key: "tab", Desc: "sidebar"},
			{Key: "?", Desc: "help"},
		}
	}
}

func (m AppModel) ExecutePaletteCommand(id string) (AppModel, tea.Cmd) {
	p := m.Paths
	switch id {
	case "dash":
		m.setSection(0)
	case "setup":
		m.setSection(1)
	case "caps":
		m.setSection(2)
	case "auto":
		m.setSection(3)
	case "gov":
		m.setSection(4)
	case "clear-console":
		m.Console.Clear()
	case "help":
		m.Help.Visible = true

	case "install-rec":
		m.setSection(1)
		m.Console.Add("[CMD] Starting Recommended Install...")
		return m, func() tea.Msg {
			r, err := installer.Recommended(p, false)
			if err != nil {
				return LogMsg(fmt.Sprintf("[ERROR] Recommended install failed: %v", err))
			}
			return LogMsg(fmt.Sprintf("[SUCCESS] Installed binary: %s | Skill packs: %v", r.InstalledBinary, r.SkillPackCounts))
		}

	case "install-full":
		m.setSection(1)
		m.Console.Add("[CMD] Starting Full Stable Setup...")
		return m, func() tea.Msg {
			r, err := installer.Full(p, false)
			if err != nil {
				return LogMsg(fmt.Sprintf("[ERROR] Full setup failed: %v", err))
			}
			return LogMsg(fmt.Sprintf("[SUCCESS] Full setup complete: %s | Packs: %v", r.InstalledBinary, r.SkillPackCounts))
		}

	case "doctor":
		m.setSection(1)
		m.Console.Add("[CMD] Running Doctor Diagnostics...")
		return m, func() tea.Msg {
			doc := doctor.RunAdvanced(p, "", false)
			if doc.HasErrors() {
				return LogMsg(fmt.Sprintf("[WARN] Doctor found %d warnings/errors", len(doc.Findings)))
			}
			return LogMsg(fmt.Sprintf("[OK] Doctor audit passed (%d checks healthy)", len(doc.Findings)))
		}

	case "probe", "probe-mcp":
		m.setSection(2)
		m.Console.Add("[CMD] Probing active MCP servers...")
		return m, func() tea.Msg {
			rep := mcpprobe.ProbeAll(p, "", 8*time.Second)
			healthy := 0
			for _, r := range rep {
				if r.OK {
					healthy++
				}
			}
			return LogMsg(fmt.Sprintf("[OK] MCP Probe complete: %d / %d healthy", healthy, len(rep)))
		}

	case "sync-skills":
		m.setSection(2)
		m.Console.Add("[CMD] Synchronizing recommended skill packs...")
		return m, func() tea.Msg {
			r, err := skills.SyncRecommended(p)
			if err != nil {
				return LogMsg(fmt.Sprintf("[ERROR] Sync skills failed: %v", err))
			}
			return LogMsg(fmt.Sprintf("[SUCCESS] Sync skills complete: %v", r))
		}

	case "self":
		m.Console.Add("[CMD] Updating self binary & hooks...")
		return m, func() tea.Msg {
			b, err := installer.InstallSelf(p)
			if err != nil {
				return LogMsg(fmt.Sprintf("[ERROR] Install self failed: %v", err))
			}
			_ = hooks.Install(p, b)
			return LogMsg(fmt.Sprintf("[SUCCESS] Self binary installed: %s", b))
		}
	}
	return m, nil
}

func (m AppModel) View() string {
	if m.TooSmall {
		t := theme.Current()
		return fmt.Sprintf("\n  %s\n\n  Required size: ≥ 70 × 18\n  Current size:  %d × %d\n\n  Please resize your terminal window to continue.",
			t.BadgeWarning.Render("Terminal window is too small"), m.Width, m.Height)
	}

	t := theme.Current()

	// Top Header (exact width)
	headerView := m.Header.View()

	// Left: Sidebar
	sidebarView := m.Sidebar.View()

	// Center: Active View Content
	var centerContent string
	switch m.Sidebar.Selected {
	case 0:
		centerContent = m.DashView.View()
	case 1:
		centerContent = m.SetupView.View()
	case 2:
		centerContent = m.CapView.View()
	case 3:
		centerContent = m.AutoView.View()
	case 4:
		centerContent = m.GovView.View()
	}

	centerBoxStyle := t.Box
	if m.Focus == FocusCenter {
		centerBoxStyle = t.BoxActive
	}

	centerInnerW := m.DashView.Width - 4
	if centerInnerW < 10 {
		centerInnerW = 10
	}
	centerInnerH := m.Sidebar.Height - 2
	if centerInnerH < 3 {
		centerInnerH = 3
	}

	centerView := centerBoxStyle.
		Width(centerInnerW).
		Height(centerInnerH).
		Render(centerContent)

	// Right: Live Console & Logs
	consoleView := m.Console.View()

	// 3-Column horizontal composition (exact W cells)
	workspaceRow := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, " ", centerView, " ", consoleView)

	// Bottom Status Bar (exact width)
	statusView := m.StatusBar.View()

	// Overlay Modals
	if m.Modal.Visible {
		modalContent := m.Modal.View()
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, modalContent)
	}

	if m.Help.Visible {
		helpContent := m.Help.View()
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, helpContent)
	}

	if m.Palette.Visible {
		paletteContent := m.Palette.View()
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, paletteContent)
	}

	return headerView + "\n" + workspaceRow + "\n" + statusView
}
