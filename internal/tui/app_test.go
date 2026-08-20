package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/homiakus/agctl/internal/paths"
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
	if !strings.Contains(res120, "NAVIGATION") {
		t.Errorf("expected Left Sidebar with NAVIGATION, got:\n%s", res120)
	}
	if !strings.Contains(res120, "LIVE CONSOLE") {
		t.Errorf("expected Right Live Console, got:\n%s", res120)
	}
	if !strings.Contains(res120, "SYSTEM OVERVIEW") {
		t.Errorf("expected Center Workspace with System Overview, got:\n%s", res120)
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

func TestAppSectionSwitchingAndLogging(t *testing.T) {
	app := NewApp(testPaths())
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	// Press '2' -> Setup section
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	view := m.View()
	if !strings.Contains(view, "SETUP, PREREQUISITES") {
		t.Errorf("expected SETUP & DOCTOR section, got:\n%s", view)
	}
	if !strings.Contains(view, "[NAV] Switched") {
		t.Errorf("expected console log to record navigation to Setup, got:\n%s", view)
	}

	// Press '3' -> Capabilities section
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	view = m.View()
	if !strings.Contains(view, "CAPABILITIES") {
		t.Errorf("expected CAPABILITIES section, got:\n%s", view)
	}

	// Press '4' -> Autonomy section
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	view = m.View()
	if !strings.Contains(view, "AUTONOMY ENGINE") {
		t.Errorf("expected AUTONOMY section, got:\n%s", view)
	}

	// Press '5' -> Governance section
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	view = m.View()
	if !strings.Contains(view, "GOVERNANCE, SECURITY") {
		t.Errorf("expected GOVERNANCE section, got:\n%s", view)
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
