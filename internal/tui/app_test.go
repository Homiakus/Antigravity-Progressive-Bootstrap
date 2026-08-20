package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/tui/i18n"
)

func testPaths() paths.Paths {
	p, _ := paths.Detect()
	return p
}

func TestApp3ColumnLayout(t *testing.T) {
	app := NewApp(testPaths())

	// Test 1: Tiny terminal (< 70 x 18)
	m, _ := app.Update(tea.WindowSizeMsg{Width: 60, Height: 15})
	resTiny := m.View()
	if !strings.Contains(resTiny, "too small") {
		t.Errorf("expected warning on tiny terminal, got:\n%s", resTiny)
	}

	// Test 2: Standard 120x30 terminal (3-column layout)
	m, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	res120 := m.View()
	if strings.Contains(res120, "too small") {
		t.Errorf("did not expect 'too small' on 120x30, got:\n%s", res120)
	}
	if !strings.Contains(res120, "НАВИГАЦИЯ") && !strings.Contains(res120, "NAVIGATION") {
		t.Errorf("expected Left Sidebar with Navigation, got:\n%s", res120)
	}
	if !strings.Contains(res120, "КОНСОЛЬ") && !strings.Contains(res120, "LIVE CONSOLE") {
		t.Errorf("expected Right Live Console, got:\n%s", res120)
	}
}

func TestPanelSwitching(t *testing.T) {
	app := NewApp(testPaths())
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	// Initial focus is Sidebar
	if m.(AppModel).Focus != FocusSidebar {
		t.Fatalf("expected initial focus on Sidebar")
	}

	// Press Tab -> FocusCenter
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.(AppModel).Focus != FocusCenter {
		t.Fatalf("expected focus on Center Workspace after Tab, got %v", m.(AppModel).Focus)
	}

	// Press Tab -> FocusConsole
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.(AppModel).Focus != FocusConsole {
		t.Fatalf("expected focus on Live Console after 2nd Tab, got %v", m.(AppModel).Focus)
	}

	// Press Tab -> FocusSidebar
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.(AppModel).Focus != FocusSidebar {
		t.Fatalf("expected focus back on Sidebar after 3rd Tab, got %v", m.(AppModel).Focus)
	}
}

func TestAppSectionSwitchingAndSettings(t *testing.T) {
	app := NewApp(testPaths())
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	// Press '6' -> Settings section
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'6'}})
	view := m.View()
	if !strings.Contains(view, "НАСТРОЙКИ") && !strings.Contains(view, "SETTINGS") {
		t.Errorf("expected SETTINGS section, got:\n%s", view)
	}
	if !strings.Contains(view, "Язык") && !strings.Contains(view, "Language") {
		t.Errorf("expected Language setting row, got:\n%s", view)
	}

	// Focus center and press Space to toggle language
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Tab to Center
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	viewAfterToggle := m.View()
	if i18n.CurrentLanguage() != i18n.LangEN {
		t.Errorf("expected English after toggle, got %s", i18n.CurrentLanguage())
	}
	if !strings.Contains(viewAfterToggle, "English") {
		t.Errorf("expected English UI in view, got:\n%s", viewAfterToggle)
	}

	// Toggle back to Russian
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if i18n.CurrentLanguage() != i18n.LangRU {
		t.Errorf("expected Russian after 2nd toggle, got %s", i18n.CurrentLanguage())
	}
}

func TestCommandPalette(t *testing.T) {
	app := NewApp(testPaths())
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	// Trigger palette with Ctrl+K
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	view := m.View()
	if !strings.Contains(view, "Command Palette") {
		t.Errorf("expected Command Palette modal, got:\n%s", view)
	}

	// Press Esc to dismiss
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	view = m.View()
	if strings.Contains(view, "Command Palette") {
		t.Errorf("expected Command Palette to close on Esc")
	}
}

func TestHelpModal(t *testing.T) {
	app := NewApp(testPaths())
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	// Press '?'
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	view := m.View()
	if !strings.Contains(view, "Keyboard Shortcuts & Help") {
		t.Errorf("expected Help modal, got:\n%s", view)
	}

	// Press '?' again to dismiss
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	view = m.View()
	if strings.Contains(view, "Keyboard Shortcuts & Help") {
		t.Errorf("expected Help modal to close")
	}
}
