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
	"github.com/homiakus/agctl/internal/tui/i18n"
	"github.com/homiakus/agctl/internal/tui/theme"
	"github.com/homiakus/agctl/internal/tui/views/autonomy"
	"github.com/homiakus/agctl/internal/tui/views/capabilities"
	"github.com/homiakus/agctl/internal/tui/views/dashboard"
	"github.com/homiakus/agctl/internal/tui/views/governance"
	"github.com/homiakus/agctl/internal/tui/views/settings"
	"github.com/homiakus/agctl/internal/tui/views/setup"
)

type FocusColumn int

const (
	FocusSidebar FocusColumn = iota
	FocusCenter
	FocusConsole
)

type AppModel struct {
	Paths        paths.Paths
	Focus        FocusColumn
	Sidebar      sidebar.Model
	Console      console.Model
	Header       header.Model
	StatusBar    statusbar.Model
	Palette      palette.Model
	Help         help.Model
	Modal        modal.Model
	Toast        toast.Model
	DashView     dashboard.Model
	SetupView    setup.Model
	CapView      capabilities.Model
	AutoView     autonomy.Model
	GovView      governance.Model
	SettingsView settings.Model
	Width        int
	Height       int
	TooSmall     bool
}

type LogMsg string

func NewApp(p paths.Paths) AppModel {
	_ = i18n.LoadSettings(p.AppRoot)

	commands := []palette.CommandItem{
		{ID: "dash", Title: "Панель управления (Dashboard)", Category: "Nav", Description: "Метрики и обзор системы", Shortcut: "1"},
		{ID: "setup", Title: "Установка и Doctor (Setup)", Category: "Nav", Description: "Установка и диагностика", Shortcut: "2"},
		{ID: "caps", Title: "Возможности и MCP (Capabilities)", Category: "Nav", Description: "Скиллы, серверы MCP", Shortcut: "3"},
		{ID: "auto", Title: "Автономия и Loop (Autonomy)", Category: "Nav", Description: "Роутер и фоновая очередь", Shortcut: "4"},
		{ID: "gov", Title: "Безопасность и Ops (Governance)", Category: "Nav", Description: "Политики и аудит", Shortcut: "5"},
		{ID: "settings", Title: "Настройки программы (Settings)", Category: "Nav", Description: "Язык (RU/EN), тема, акценты", Shortcut: "6"},
		{ID: "lang-toggle", Title: "Переключить язык (RU/EN)", Category: "Settings", Description: "Смена языка интерфейса"},
		{ID: "install-rec", Title: "Рекомендованная установка", Category: "Setup", Description: "Установка ядра и meta-skills"},
		{ID: "install-full", Title: "Полная установка", Category: "Setup", Description: "Все пакеты и sidecars"},
		{ID: "doctor", Title: "Диагностика Doctor", Category: "Diagnostics", Description: "Полный аудит среды"},
		{ID: "probe", Title: "Live Probe MCP", Category: "MCP", Description: "Опрос активных MCP серверов"},
		{ID: "clear-console", Title: "Очистить консоль", Category: "Console", Description: "Очистить лог", Shortcut: "c"},
		{ID: "help", Title: "Справка", Category: "Help", Description: "Горячие клавиши", Shortcut: "?"},
	}

	sb := sidebar.New()
	sb.Focused = true

	con := console.New()
	if i18n.CurrentLanguage() == i18n.LangRU {
		con.Add("[INFO] agctl 3.2.1 инициализирован (Русский интерфейс по умолчанию).")
		con.Add("[INFO] 3-колоночный интерфейс: Меню слева, Рабочая зона в центре, Консоль справа.")
		con.Add("[INFO] Клавишами 1..6 переход в разделы, Tab / Shift+Tab смена окна.")
	} else {
		con.Add("[INFO] agctl 3.2.1 initialized (Desktop 3-column layout).")
		con.Add("[INFO] Left Sidebar, Center Workspace, Right Live Console.")
		con.Add("[INFO] 1..6 jump to section, Tab / Shift+Tab cycle panels.")
	}

	app := AppModel{
		Paths:        p,
		Focus:        FocusSidebar,
		Sidebar:      sb,
		Console:      con,
		Header:       header.New("3.2.1"),
		StatusBar:    statusbar.New(),
		Palette:      palette.New(commands),
		Help:         help.New(),
		Modal:        modal.New(),
		Toast:        toast.New(),
		DashView:     dashboard.New(p),
		SetupView:    setup.New(p),
		CapView:      capabilities.New(p),
		AutoView:     autonomy.New(p),
		GovView:      governance.New(p),
		SettingsView: settings.New(p),
	}
	app.syncFocus()
	app.updateBreadcrumbs()
	return app
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

		// Exact 3-column width math (Sidebar 30 chars for full Russian titles)
		sidebarW := 30
		consoleW := msg.Width * 38 / 100
		if consoleW < 34 {
			consoleW = 34
		} else if consoleW > 55 {
			consoleW = 55
		}
		centerW := msg.Width - sidebarW - consoleW - 2
		if centerW < 28 {
			centerW = 28
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
		m.SettingsView.Width = centerW
		m.SettingsView.Height = panelH
		return m, nil

	case LogMsg:
		m.Console.Add(string(msg))
		return m, nil

	case setup.OpCompletedMsg:
		for _, line := range msg.Output {
			m.Console.Add(line)
		}
		if msg.Success {
			m.Console.Add("[SUCCESS] Операция успешно завершена.")
		} else if msg.Err != nil {
			m.Console.Add(fmt.Sprintf("[ERROR] Ошибка выполнения: %v", msg.Err))
		}
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
		case "6":
			m.setSection(5)
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
	case 5: // Settings
		var cmd tea.Cmd
		m.SettingsView, cmd = m.SettingsView.Update(msg)
		cmds = append(cmds, cmd)
		m.syncFocus()
		m.updateBreadcrumbs()
	}

	return m, tea.Batch(cmds...)
}

func (m *AppModel) setSection(idx int) {
	if idx < 0 || idx >= len(m.Sidebar.Items) {
		return
	}
	m.Sidebar.Selected = idx
	m.updateBreadcrumbs()
	switch idx {
	case 0:
		m.DashView.Refresh()
		m.Console.Addf("[NAV] %s", i18n.T("sec_dashboard"))
	case 1:
		m.Console.Addf("[NAV] %s", i18n.T("sec_setup"))
	case 2:
		m.CapView.Refresh()
		m.Console.Addf("[NAV] %s", i18n.T("sec_caps"))
	case 3:
		m.AutoView.Refresh()
		m.Console.Addf("[NAV] %s", i18n.T("sec_autonomy"))
	case 4:
		m.GovView.Refresh()
		m.Console.Addf("[NAV] %s", i18n.T("sec_governance"))
	case 5:
		m.Console.Addf("[NAV] %s", i18n.T("sec_settings"))
	}
}

func (m *AppModel) updateBreadcrumbs() {
	switch m.Sidebar.Selected {
	case 0:
		m.Header.Breadcrumbs = []string{i18n.T("sec_dashboard")}
	case 1:
		m.Header.Breadcrumbs = []string{i18n.T("sec_setup")}
	case 2:
		m.Header.Breadcrumbs = []string{i18n.T("sec_caps")}
	case 3:
		m.Header.Breadcrumbs = []string{i18n.T("sec_autonomy")}
	case 4:
		m.Header.Breadcrumbs = []string{i18n.T("sec_governance")}
	case 5:
		m.Header.Breadcrumbs = []string{i18n.T("sec_settings")}
	}
}

func (m *AppModel) syncFocus() {
	m.Sidebar.Focused = m.Focus == FocusSidebar
	m.Console.Focused = m.Focus == FocusConsole

	moveText := i18n.T("hint_move")
	selText := i18n.T("hint_select")
	execText := i18n.T("hint_execute")
	panelText := i18n.T("hint_panel")
	cmdText := i18n.T("hint_commands")
	helpText := i18n.T("hint_help")

	switch m.Focus {
	case FocusSidebar:
		m.StatusBar.Hints = []statusbar.KeyHint{
			{Key: "↑↓", Desc: moveText},
			{Key: "enter/→", Desc: selText},
			{Key: "tab", Desc: panelText},
			{Key: "ctrl+k", Desc: cmdText},
			{Key: "?", Desc: helpText},
		}
	case FocusCenter:
		m.StatusBar.Hints = []statusbar.KeyHint{
			{Key: "↑↓", Desc: moveText},
			{Key: "enter/space", Desc: execText},
			{Key: "←", Desc: i18n.T("hint_sidebar")},
			{Key: "→", Desc: i18n.T("hint_console")},
			{Key: "tab", Desc: panelText},
			{Key: "?", Desc: helpText},
		}
	case FocusConsole:
		m.StatusBar.Hints = []statusbar.KeyHint{
			{Key: "c", Desc: i18n.T("hint_clear")},
			{Key: "←", Desc: i18n.T("hint_workspace")},
			{Key: "tab", Desc: panelText},
			{Key: "?", Desc: helpText},
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
	case "settings":
		m.setSection(5)

	case "lang-toggle":
		if i18n.CurrentLanguage() == i18n.LangRU {
			i18n.SetLanguage(i18n.LangEN)
			m.Console.Add("[SETTINGS] Language switched to English.")
		} else {
			i18n.SetLanguage(i18n.LangRU)
			m.Console.Add("[SETTINGS] Язык интерфейса переключен на Русский.")
		}
		m.syncFocus()
		m.updateBreadcrumbs()

	case "clear-console":
		m.Console.Clear()
	case "help":
		m.Help.Visible = true

	case "install-rec":
		m.setSection(1)
		m.Console.Add("[CMD] > agctl install recommended")
		m.Console.Add("[EXEC] Установка базовых мета-скиллов и системного бинарника...")
		return m, func() tea.Msg {
			r, err := installer.Recommended(p, false)
			if err != nil {
				return LogMsg(fmt.Sprintf("[ERROR] Ошибка установки: %v", err))
			}
			return LogMsg(fmt.Sprintf("[SUCCESS] Бинарник установлен: %s\n[OK] Пакеты скиллов: %v", r.InstalledBinary, r.SkillPackCounts))
		}

	case "install-full":
		m.setSection(1)
		m.Console.Add("[CMD] > agctl install full --prereqs")
		m.Console.Add("[EXEC] Запуск полной установки всех компонентов...")
		return m, func() tea.Msg {
			r, err := installer.Full(p, false)
			if err != nil {
				return LogMsg(fmt.Sprintf("[ERROR] Ошибка полной установки: %v", err))
			}
			res := fmt.Sprintf("[SUCCESS] Полная установка завершена: %s\n[OK] Пакеты: %v", r.InstalledBinary, r.SkillPackCounts)
			for _, w := range r.MCPWarnings {
				res += fmt.Sprintf("\n[WARN] %s", w)
			}
			return LogMsg(res)
		}

	case "doctor":
		m.setSection(1)
		m.Console.Add("[CMD] > agctl doctor --self-test")
		m.Console.Add("[EXEC] Запуск комплексного аудита окружения...")
		return m, func() tea.Msg {
			doc := doctor.RunAdvanced(p, "", false)
			if doc.HasErrors() {
				res := fmt.Sprintf("[WARN] Doctor обнаружил %d замечаний:", len(doc.Findings))
				for _, f := range doc.Findings {
					res += fmt.Sprintf("\n  [%s] %s: %s", f.Level, f.Area, f.Message)
				}
				return LogMsg(res)
			}
			res := fmt.Sprintf("[OK] Комплексный аудит Doctor пройден (%d проверок успешно):", len(doc.Findings))
			for _, f := range doc.Findings {
				res += fmt.Sprintf("\n  ● %s: %s", f.Area, f.Message)
			}
			return LogMsg(res)
		}

	case "probe", "probe-mcp":
		m.setSection(2)
		m.Console.Add("[CMD] > agctl mcp probe")
		m.Console.Add("[EXEC] Опрос времени отклика и инструментов всех MCP серверов...")
		return m, func() tea.Msg {
			rep := mcpprobe.ProbeAll(p, "", 8*time.Second)
			healthy := 0
			res := "[OK] Результаты Live Probe MCP:"
			for _, r := range rep {
				if r.OK {
					healthy++
					res += fmt.Sprintf("\n  ● [OK] %s: %dms (инструментов: %d)", r.Name, r.LatencyMS, len(r.Tools))
				} else {
					res += fmt.Sprintf("\n  ✕ [FAIL] %s: %s", r.Name, r.Error)
				}
			}
			res += fmt.Sprintf("\n[SUCCESS] Итог: %d из %d серверов активны и готовы к работе.", healthy, len(rep))
			return LogMsg(res)
		}

	case "sync-skills":
		m.setSection(2)
		m.Console.Add("[CMD] > agctl skills sync-recommended")
		m.Console.Add("[EXEC] Синхронизация репозиториев скиллов...")
		return m, func() tea.Msg {
			r, err := skills.SyncRecommended(p)
			if err != nil {
				return LogMsg(fmt.Sprintf("[ERROR] Ошибка синхронизации: %v", err))
			}
			return LogMsg(fmt.Sprintf("[SUCCESS] Синхронизация завершена успешно:\n[OK] Результат: %v", r))
		}

	case "self":
		m.Console.Add("[CMD] > agctl install self")
		m.Console.Add("[EXEC] Перекомпиляция agctl и регистрация хуков жизненного цикла...")
		return m, func() tea.Msg {
			b, err := installer.InstallSelf(p)
			if err != nil {
				return LogMsg(fmt.Sprintf("[ERROR] Ошибка компиляции: %v", err))
			}
			_ = hooks.Install(p, b)
			return LogMsg(fmt.Sprintf("[SUCCESS] Бинарник установлен: %s\n[OK] Хуки зарегистрированы.", b))
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
	case 5:
		centerContent = m.SettingsView.View()
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
