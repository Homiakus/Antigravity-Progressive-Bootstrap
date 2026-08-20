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
	"github.com/homiakus/agctl/internal/tui/i18n"
	"github.com/homiakus/agctl/internal/tui/theme"
)

type SetupOption struct {
	ID       string
	KeyTitle string
	KeyDesc  string
}

type Model struct {
	Paths    paths.Paths
	Options  []SetupOption
	Cursor   int
	Running  bool
	ActiveOp string
	Width    int
	Height   int
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
			{ID: "rec", KeyTitle: "setup_rec", KeyDesc: "setup_rec_d"},
			{ID: "full", KeyTitle: "setup_full", KeyDesc: "setup_full_d"},
			{ID: "doctor", KeyTitle: "setup_doc", KeyDesc: "setup_doc_d"},
			{ID: "prereq", KeyTitle: "setup_prereq", KeyDesc: "setup_prereq_d"},
			{ID: "self", KeyTitle: "setup_self", KeyDesc: "setup_self_d"},
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
	m.ActiveOp = i18n.T(opt.KeyTitle)

	p := m.Paths
	return *m, func() tea.Msg {
		var out []string
		var err error

		switch opt.ID {
		case "rec":
			r, e := installer.Recommended(p, false)
			err = e
			if e == nil {
				out = append(out, fmt.Sprintf("[OK] Установлен бинарник: %s", r.InstalledBinary))
				out = append(out, fmt.Sprintf("[OK] Установлены пакеты скиллов: %v", r.SkillPackCounts))
			} else {
				out = append(out, fmt.Sprintf("[ERROR] Ошибка установки: %v", e))
			}
		case "full":
			r, e := installer.Full(p, false)
			err = e
			if e == nil {
				out = append(out, fmt.Sprintf("[OK] Полная установка завершена: %s", r.InstalledBinary))
				out = append(out, fmt.Sprintf("[OK] Пакеты: %v", r.SkillPackCounts))
				for _, w := range r.MCPWarnings {
					out = append(out, fmt.Sprintf("[WARN] MCP: %s", w))
				}
			} else {
				out = append(out, fmt.Sprintf("[ERROR] Ошибка: %v", e))
			}
		case "doctor":
			doc := doctor.RunAdvanced(p, "", false)
			if doc.HasErrors() {
				out = append(out, "[WARN] Doctor обнаружил замечания по конфигурации:")
			} else {
				out = append(out, "[OK] Комплексная проверка Doctor PASSED (все проверки здоровы):")
			}
			for _, finding := range doc.Findings {
				out = append(out, fmt.Sprintf("  [%s] %s: %s", finding.Level, finding.Area, finding.Message))
			}
		case "prereq":
			tools := []string{"git", "go", "node", "npm", "rg"}
			for _, tool := range tools {
				if path, e := exec.LookPath(tool); e == nil {
					out = append(out, fmt.Sprintf("[OK] %s найден по пути: %s", tool, path))
				} else {
					out = append(out, fmt.Sprintf("[WARN] %s отсутствует в переменной PATH", tool))
				}
			}
		case "self":
			b, e := installer.InstallSelf(p)
			err = e
			if e == nil {
				out = append(out, fmt.Sprintf("[OK] Бинарник скомпилирован в: %s", b))
				if e2 := hooks.Install(p, b); e2 != nil {
					out = append(out, fmt.Sprintf("[WARN] Регистрация хуков: %v", e2))
				} else {
					out = append(out, "[OK] Хуки жизненного цикла зарегистрированы успешно.")
				}
			} else {
				out = append(out, fmt.Sprintf("[ERROR] Ошибка сборки: %v", e))
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

	sb.WriteString(t.MicroLabel.Render(i18n.T("setup_title")) + "\n\n")

	if m.Running {
		runningTxt := "◌ " + i18n.T("status_running") + ": " + m.ActiveOp
		sb.WriteString(t.BadgeInfo.Render(runningTxt) + "\n\n")
	}

	for i, opt := range m.Options {
		isSel := i == m.Cursor
		prefix := "  "
		titleStyle := t.ItemNormal
		if isSel {
			prefix = t.Symbols.ArrowRight + " "
			titleStyle = t.ItemActive
		}
		title := titleStyle.Render(i18n.T(opt.KeyTitle))
		desc := t.Muted.Render("    " + i18n.T(opt.KeyDesc))
		sb.WriteString(prefix + title + "\n" + desc + "\n\n")
	}

	sb.WriteString(t.Muted.Render(i18n.T("setup_footer")))
	return sb.String()
}
