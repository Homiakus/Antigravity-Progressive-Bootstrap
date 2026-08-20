package help

import (
	"strings"

	"github.com/homiakus/agctl/internal/tui/i18n"
	"github.com/homiakus/agctl/internal/tui/theme"
)

type HelpSection struct {
	Title string
	Items [][2]string // [key, description]
}

type Model struct {
	Visible bool
	Width   int
}

func New() Model {
	return Model{
		Visible: false,
		Width:   60,
	}
}

func (m *Model) Toggle() {
	m.Visible = !m.Visible
}

func (m Model) sections() []HelpSection {
	if i18n.CurrentLanguage() == i18n.LangRU {
		return []HelpSection{
			{
				Title: "НАВИГАЦИЯ",
				Items: [][2]string{
					{"1..6", "Мгновенный переход в раздел"},
					{"↑ ↓ / j k", "Перемещение по пунктам списка"},
					{"Tab / Shift+Tab", "Переключение между 3 окнами"},
					{"← / →", "Переход в соседнюю колонку"},
				},
			},
			{
				Title: "ДЕЙСТВИЯ",
				Items: [][2]string{
					{"Enter", "Запустить действие / Подтвердить"},
					{"Space", "Переключить параметр / чекбокс"},
					{"c", "Очистить логи консоли"},
					{"r", "Повторить / Живой опрос MCP"},
				},
			},
			{
				Title: "ГЛОБАЛЬНЫЕ",
				Items: [][2]string{
					{"Ctrl+K", "Палитра команд и быстрый запуск"},
					{"?", "Открыть / Закрыть эту справку"},
					{"Esc / q", "Назад / Закрыть окно / Выход"},
				},
			},
		}
	}

	return []HelpSection{
		{
			Title: "NAVIGATION",
			Items: [][2]string{
				{"1..6", "Switch top-level section"},
				{"↑ ↓ / j k", "Move cursor in active list"},
				{"Tab / S-Tab", "Cycle focus between panels"},
				{"← / →", "Switch between adjacent panels"},
			},
		},
		{
			Title: "ACTIONS",
			Items: [][2]string{
				{"Enter", "Open / Execute / Select item"},
				{"Space", "Toggle option / Checkbox"},
				{"c", "Clear console logs"},
				{"r", "Refresh / Live probe MCP"},
			},
		},
		{
			Title: "GLOBAL",
			Items: [][2]string{
				{"Ctrl+K", "Command palette & quick launcher"},
				{"?", "Toggle this help cheatsheet"},
				{"Esc / q", "Back / Close modal / Quit"},
			},
		},
	}
}

func (m Model) View() string {
	t := theme.Current()
	var sb strings.Builder

	titleTxt := "  Keyboard Shortcuts & Help  "
	if i18n.CurrentLanguage() == i18n.LangRU {
		titleTxt = "  Горячие клавиши и Справка  "
	}
	sb.WriteString(t.Title.Render(titleTxt) + "\n\n")

	for _, sec := range m.sections() {
		sb.WriteString(t.Subtitle.Render("◈ "+sec.Title) + "\n")
		for _, item := range sec.Items {
			key := t.Key.Render(item[0])
			desc := t.KeyDesc.Render(item[1])
			sb.WriteString("  " + key + "  " + desc + "\n")
		}
		sb.WriteString("\n")
	}

	dismissTxt := "Press Esc or ? to close"
	if i18n.CurrentLanguage() == i18n.LangRU {
		dismissTxt = "Нажмите Esc или ? для закрытия"
	}
	sb.WriteString(t.Muted.Render(dismissTxt))

	modalWidth := 56
	if m.Width > 0 && m.Width < modalWidth+4 {
		modalWidth = m.Width - 4
	}

	return t.ModalBox.Width(modalWidth).Render(sb.String())
}
