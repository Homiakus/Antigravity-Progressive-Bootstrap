package loop

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
)

const (
	PermissionGuarded      = "guarded"
	PermissionUnrestricted = "unrestricted"
)

func DefaultConfig() model.LoopConfig {
	return model.LoopConfig{
		Enabled:                   false,
		MaxExecutions:             50,
		PermissionMode:            PermissionGuarded,
		RequireVerification:       true,
		ContinueOnRecoverableStop: true,
	}
}

func Load(p paths.Paths) (model.LoopConfig, error) {
	cfg, err := jsonx.Read(p.LoopConfig, DefaultConfig())
	if err != nil {
		return cfg, err
	}
	if cfg.PermissionMode == "" {
		cfg.PermissionMode = PermissionGuarded
	}
	return cfg, nil
}

func Save(p paths.Paths, cfg model.LoopConfig) error {
	if cfg.PermissionMode != PermissionGuarded && cfg.PermissionMode != PermissionUnrestricted {
		return fmt.Errorf("invalid permission mode %q", cfg.PermissionMode)
	}
	if cfg.MaxExecutions < 0 {
		return fmt.Errorf("max executions must be >= 0")
	}
	return jsonx.WriteAtomic(p.LoopConfig, cfg, p.BackupsRoot)
}

func EnableProfile(p paths.Paths, profile string) (model.LoopConfig, error) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	switch profile {
	case "standard":
		cfg.MaxExecutions = 25
	case "deep":
		cfg.MaxExecutions = 50
	case "until-done", "unlimited":
		cfg.MaxExecutions = 0
	case "unrestricted":
		cfg.MaxExecutions = 0
		cfg.PermissionMode = PermissionUnrestricted
	default:
		return cfg, fmt.Errorf("unknown loop profile %q", profile)
	}
	return cfg, Save(p, cfg)
}

func Disable(p paths.Paths) error {
	cfg, _ := Load(p)
	cfg.Enabled = false
	return Save(p, cfg)
}

func statePath(p paths.Paths, conversationID string) string {
	sum := sha256.Sum256([]byte(conversationID))
	name := hex.EncodeToString(sum[:8]) + ".json"
	return filepath.Join(p.StateRoot, name)
}

func LoadState(p paths.Paths, conversationID string) (model.TaskState, bool, error) {
	path := statePath(p, conversationID)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return model.TaskState{}, false, nil
	}
	if err != nil {
		return model.TaskState{}, false, err
	}
	st, err := jsonx.Read(path, model.TaskState{})
	return st, true, err
}

func SaveState(p paths.Paths, st model.TaskState) error {
	st.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	return jsonx.WriteAtomic(statePath(p, st.ConversationID), st, "")
}

func EnsureTaskState(p paths.Paths, in model.PreInvocationInput) (model.TaskState, error) {
	old, exists, err := LoadState(p, in.ConversationID)
	if err != nil {
		return old, err
	}
	// Stop-continue may start another invocation/execution while the task is still incomplete.
	// Never overwrite an active incomplete state.
	if exists && !old.Complete && !old.HardBlocker {
		return old, nil
	}
	// If the last task completed, a larger trajectory size indicates a later user turn.
	if exists && old.Complete && in.InitialNumSteps <= old.InitialNumSteps {
		return old, nil
	}

	taskID := fmt.Sprintf("%s-%d-%d", shortID(in.ConversationID), in.InitialNumSteps, time.Now().Unix())
	st := model.TaskState{
		ConversationID:  in.ConversationID,
		TaskID:          taskID,
		InitialNumSteps: in.InitialNumSteps,
		StartedAt:       time.Now().Format(time.RFC3339Nano),
		UpdatedAt:       time.Now().Format(time.RFC3339Nano),
		Verification:    []string{},
	}
	if err := SaveState(p, st); err != nil {
		return st, err
	}
	return st, nil
}

func shortID(s string) string {
	s = regexp.MustCompile(`[^A-Za-z0-9_.-]+`).ReplaceAllString(s, "_")
	if len(s) > 24 {
		return s[:24]
	}
	return s
}

func CompletionInjection(exe string, st model.TaskState) string {
	qexe := exe
	if strings.ContainsAny(exe, " \t") {
		qexe = `"` + exe + `"`
	}
	return fmt.Sprintf(`AUTONOMOUS COMPLETION LOOP — ACTIVE

You own the delegated outcome, not just the next step. Do not stop after a plan, partial implementation, one failed test, a progress summary, or a recoverable tool error.

Task ID: %s

Mandatory cycle:
1. infer a concrete Definition of Done from the user's request and repository contracts;
2. inspect the relevant implementation and configuration;
3. implement the next missing requirement;
4. run the strongest practical targeted verification;
5. on failure: capture evidence -> diagnose root cause -> change approach/code -> rerun;
6. review all original requirements for omissions;
7. run broader regression/build/E2E/security checks where applicable;
8. inspect the final diff/state;
9. only then mark the task complete.

Before your final answer, call this program through the terminal to record verified completion:
%s state complete --conversation %q --task-id %q --summary "<concise summary>" --verify "<actual check 1>" --verify "<actual check 2>"

Only use verification entries for checks that actually ran. The Stop hook will reject premature termination unless complete=true, verified=true, hardBlocker=false and verification evidence is non-empty.

If completion is genuinely impossible without an unavailable secret, external authorization, quota, required physical resource, or a non-inferable irreversible product decision, record a blocker instead:
%s state block --conversation %q --task-id %q --summary "<exact external blocker>"

Do NOT classify normal build/test/lint failures, missing dev dependencies, coding uncertainty, or recoverable tool errors as blockers. Fix or re-route them. Do not ask the user whether to continue.`, st.TaskID, qexe, st.ConversationID, st.TaskID, qexe, st.ConversationID, st.TaskID)
}

func MarkComplete(p paths.Paths, conversation, taskID, summary string, verification []string) error {
	st, ok, err := LoadState(p, conversation)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no active state for conversation %q", conversation)
	}
	if st.TaskID != taskID {
		return fmt.Errorf("task id mismatch: active=%q got=%q", st.TaskID, taskID)
	}
	if strings.TrimSpace(summary) == "" {
		return fmt.Errorf("completion summary is required")
	}
	clean := make([]string, 0, len(verification))
	for _, v := range verification {
		if strings.TrimSpace(v) != "" {
			clean = append(clean, strings.TrimSpace(v))
		}
	}
	if len(clean) == 0 {
		return fmt.Errorf("at least one actual verification item is required")
	}
	st.Complete = true
	st.Verified = true
	st.HardBlocker = false
	st.Summary = summary
	st.Verification = clean
	return SaveState(p, st)
}

func MarkBlocker(p paths.Paths, conversation, taskID, summary string) error {
	st, ok, err := LoadState(p, conversation)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no active state for conversation %q", conversation)
	}
	if st.TaskID != taskID {
		return fmt.Errorf("task id mismatch")
	}
	if strings.TrimSpace(summary) == "" {
		return fmt.Errorf("blocker summary is required")
	}
	st.Complete = false
	st.Verified = false
	st.HardBlocker = true
	st.Summary = summary
	return SaveState(p, st)
}

func ResetState(p paths.Paths, conversation string) error {
	err := os.Remove(statePath(p, conversation))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
