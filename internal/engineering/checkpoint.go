package engineering

import (
	"fmt"
	"strings"
)

var checkpointRequiredFields = []string{
	"CURRENT HEAD",
	"CURRENT QUALIFIED MILESTONE",
	"ARCHITECTURE",
	"CRITICAL INVARIANTS",
	"COMPLETED THIS ITERATION",
	"RESOLVED FINDINGS",
	"OPEN CRITICAL/HIGH FINDINGS",
	"BLOCKERS",
	"NEXT TASK",
	"WHY NEXT",
	"CRITICAL FILES",
	"VERIFICATION COMMANDS",
	"IMPORTANT DECISIONS",
	"REJECTED OPTIONS",
	"NEW PROCESS LEARNING",
}

// PlanCheckpoint is the latest repository-resident context compression state.
// It is intentionally parsed from MASTER_PLAN.md so recovery does not depend on
// chat history or machine-local state.
type PlanCheckpoint struct {
	Heading string
	Fields  map[string]string
}

// ParseLatestPlanCheckpoint returns only the latest checkpoint section. A stale
// older checkpoint can therefore never satisfy a newer task completion.
func ParseLatestPlanCheckpoint(plan string) (PlanCheckpoint, error) {
	lines := strings.Split(plan, "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "### Context Compression Checkpoint") {
			start = i
		}
	}
	if start < 0 {
		return PlanCheckpoint{}, fmt.Errorf("%s has no Context Compression Checkpoint", PlanFileName)
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "### ") {
			end = i
			break
		}
	}
	cp := PlanCheckpoint{Heading: strings.TrimSpace(lines[start]), Fields: map[string]string{}}
	for _, line := range lines[start+1 : end] {
		trimmed := strings.TrimSpace(line)
		// Repository checkpoints conventionally render the field label as inline
		// code (`FIELD:`) followed by ordinary Markdown value text. Strip markup
		// before splitting so the closing backtick cannot become part of the key.
		trimmed = strings.ReplaceAll(trimmed, "`", "")
		idx := strings.Index(trimmed, ":")
		if idx <= 0 {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(trimmed[:idx]))
		value := strings.TrimSpace(trimmed[idx+1:])
		if key != "" && value != "" {
			cp.Fields[key] = value
		}
	}
	return cp, nil
}

// ValidatePlanCheckpoint proves that the latest repository-resident checkpoint
// contains the complete recovery state and belongs to the task being completed.
func ValidatePlanCheckpoint(plan, taskID string) error {
	cp, err := ParseLatestPlanCheckpoint(plan)
	if err != nil {
		return err
	}
	for _, key := range checkpointRequiredFields {
		value := strings.TrimSpace(cp.Fields[key])
		if value == "" {
			return fmt.Errorf("latest Context Compression Checkpoint missing %q", key)
		}
	}
	completed := cp.Fields["COMPLETED THIS ITERATION"]
	if !containsToken(completed, taskID) {
		return fmt.Errorf("latest Context Compression Checkpoint is stale: completed=%q task=%q", completed, taskID)
	}
	return nil
}

func containsToken(value, token string) bool {
	for _, field := range strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case ' ', '\t', '\r', '\n', ',', ';', '|', '.', ':', '(', ')', '[', ']', '{', '}', '`':
			return true
		default:
			return false
		}
	}) {
		if field == token {
			return true
		}
	}
	return false
}
