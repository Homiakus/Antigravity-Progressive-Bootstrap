package console

import (
	"fmt"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/tui/i18n"
	"github.com/homiakus/agctl/internal/tui/theme"
)

type LogEntry struct {
	Timestamp string
	Text      string
}

type Model struct {
	Logs    []LogEntry
	Focused bool
	Width   int
	Height  int
}

func New() Model {
	m := Model{
		Focused: false,
		Logs:    make([]LogEntry, 0),
	}
	m.Add("[INFO] agctl console initialized and ready.")
	return m
}

func (m *Model) Add(text string) {
	ts := time.Now().Format("15:04:05")
	m.Logs = append(m.Logs, LogEntry{
		Timestamp: ts,
		Text:      text,
	})
	if len(m.Logs) > 300 {
		m.Logs = m.Logs[len(m.Logs)-300:]
	}
}

func (m *Model) Addf(format string, a ...any) {
	m.Add(fmt.Sprintf(format, a...))
}

func (m *Model) Clear() {
	m.Logs = make([]LogEntry, 0)
	if i18n.CurrentLanguage() == i18n.LangRU {
		m.Add("[INFO] Консоль логов очищена.")
	} else {
		m.Add("[INFO] Console cleared.")
	}
}

func (m Model) View() string {
	t := theme.Current()
	var sb strings.Builder

	innerWidth := m.Width - 4
	if innerWidth < 15 {
		innerWidth = 15
	}
	innerHeight := m.Height - 2
	if innerHeight < 3 {
		innerHeight = 3
	}

	// Header with i18n
	sb.WriteString(t.ConsoleTitle.Render("> "+i18n.T("live_console")) + "\n\n")

	maxLines := innerHeight - 2
	if maxLines < 1 {
		maxLines = 1
	}

	start := 0
	if len(m.Logs) > maxLines {
		start = len(m.Logs) - maxLines
	}

	maxTextWidth := innerWidth - 11
	if maxTextWidth < 10 {
		maxTextWidth = 10
	}

	for i := start; i < len(m.Logs); i++ {
		entry := m.Logs[i]
		ts := t.ConsoleTimestamp.Render("[" + entry.Timestamp + "] ")

		line := entry.Text
		if len(line) > maxTextWidth {
			line = line[:maxTextWidth-3] + "..."
		}

		var lineContent string
		if strings.HasPrefix(line, "[OK]") || strings.HasPrefix(line, "[SUCCESS]") {
			lineContent = t.ConsoleSuccess.Render(line)
		} else if strings.HasPrefix(line, "[WARN]") {
			lineContent = t.ConsoleWarn.Render(line)
		} else if strings.HasPrefix(line, "[ERROR]") || strings.HasPrefix(line, "[FAIL]") {
			lineContent = t.ConsoleErr.Render(line)
		} else if strings.HasPrefix(line, "[CMD]") || strings.HasPrefix(line, "[EXEC]") {
			lineContent = t.BadgePurple.Render(line)
		} else if strings.HasPrefix(line, "[INFO]") || strings.HasPrefix(line, "[NAV]") {
			lineContent = t.ConsoleInfo.Render(line)
		} else {
			lineContent = t.Body.Render(line)
		}

		sb.WriteString(ts + lineContent + "\n")
	}

	boxStyle := t.ConsoleBox
	if m.Focused {
		boxStyle = t.ConsoleBoxActive
	}

	return boxStyle.Width(innerWidth).Height(innerHeight).Render(sb.String())
}
