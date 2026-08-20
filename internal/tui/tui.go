package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/homiakus/agctl/internal/paths"
)

// Run starts the modern Bubble Tea TUI for agctl.
func Run(p paths.Paths) error {
	app := NewApp(p)
	program := tea.NewProgram(
		app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Safe execution and panic recovery to guarantee terminal reset
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "agctl TUI recovered from unexpected panic: %v\n", r)
		}
	}()

	_, err := program.Run()
	return err
}
