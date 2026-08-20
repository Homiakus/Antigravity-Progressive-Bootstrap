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

// Add appends a log entry, splitting multiline strings into individual lines
func (m *Model) Add(text string) {
	ts := time.Now().Format("15:04:05")
	lines := strings.Split(text, "\n")
	for _, l := range lines {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) == "" {
			continue
		}
		m.Logs = append(m.Logs, LogEntry{
			Timestamp: ts,
			Text:      l,
		})
	}
	if len(m.Logs) > 1000 {
		m.Logs = m.Logs[len(m.Logs)-1000:]
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

	// Header
	sb.WriteString(t.ConsoleTitle.Render("> "+i18n.T("live_console")) + "\n\n")

	maxLines := innerHeight - 2
	if maxLines < 1 {
		maxLines = 1
	}

	// Prepare wrapped render lines
	type renderLine struct {
		ts   string
		text string
	}
	var rendered []renderLine

	// Available width for log text after timestamp "[15:04:05] " (11 chars)
	maxTextWidth := innerWidth - 11
	if maxTextWidth < 12 {
		maxTextWidth = 12
	}

	for _, entry := range m.Logs {
		text := entry.Text
		// Wrap line if it exceeds maxTextWidth
		if len(text) <= maxTextWidth {
			rendered = append(rendered, renderLine{ts: entry.Timestamp, text: text})
		} else {
			// Multi-line wrap
			remaining := text
			first := true
			for len(remaining) > 0 {
				chunkLen := maxTextWidth
				if len(remaining) < chunkLen {
					chunkLen = len(remaining)
				}
				chunk := remaining[:chunkLen]
				remaining = remaining[chunkLen:]

				if first {
					rendered = append(rendered, renderLine{ts: entry.Timestamp, text: chunk})
					first = false
				} else {
					rendered = append(rendered, renderLine{ts: "        ", text: "  " + chunk})
				}
			}
		}
	}

	start := 0
	if len(rendered) > maxLines {
		start = len(rendered) - maxLines
	}

	for i := start; i < len(rendered); i++ {
		r := rendered[i]
		ts := t.ConsoleTimestamp.Render("[" + r.ts + "] ")
		line := r.text

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
