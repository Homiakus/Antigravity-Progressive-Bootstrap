package goal

import (
	"fmt"
	"strings"
)

// NativeCommand composes the user-facing Antigravity 2.0 slash command. It is
// deliberately separate from headless AGY execution because UI slash-command
// support and CLI prompt parsing are different surfaces.
func NativeCommand(prompt string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}
	return "/goal " + prompt, nil
}

// HeadlessPrompt gives AGY CLI the same completion semantics without assuming
// that desktop slash commands are parsed by the headless -p surface.
func HeadlessPrompt(prompt string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}
	return `Run this delegated task until it is completely and verifiably finished. Do not ask for intermediate input when a reversible engineering choice can be inferred. Continue through implementation, tests, diagnosis, fixes, final requirement coverage, and regression verification. Use native Antigravity subagents when useful and obey the agctl verified completion gate.\n\nTASK:\n` + prompt, nil
}
