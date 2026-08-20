package risk

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
)

const (
	RiskReadLow      = "read-low"
	RiskWriteMedium  = "write-medium"
	RiskExecHigh     = "execution-high"
	RiskExternalHigh = "external-write-high"
	RiskCritical     = "destructive-critical"
	RiskUnknown      = "unknown"
	DecisionAllow    = "allow"
	DecisionDeny     = "deny"
	DecisionAsk      = "ask"
)

type Context struct {
	PermissionMode string
	Workspaces     []string
	Home           string
}

type Policy struct {
	Version               int      `json:"version"`
	CriticalToolPatterns  []string `json:"criticalToolPatterns"`
	ExternalWritePatterns []string `json:"externalWritePatterns"`
	CriticalCommands      []string `json:"criticalCommands"`
}

func DefaultPolicy() Policy {
	return Policy{Version: 1, CriticalToolPatterns: []string{
		`(?i)(^|[_./:-])(delete|drop|destroy|wipe|purge|erase|terminate|remove[_-]?repository|delete[_-]?repository)([_./:-]|$)`,
		`(?i)(terraform|tofu)[_./:-]?(destroy|apply[_-]?destroy)`,
		`(?i)(database|db)[_./:-]?(drop|truncate|destroy)`,
		`(?i)(cluster|project|account|organization|tenant)[_./:-]?(delete|destroy|remove)`,
		`(?i)(secret|vault)[_./:-]?(delete|destroy|purge)`,
	}, ExternalWritePatterns: []string{
		`(?i)(^|[_./:-])(create|update|edit|write|merge|close|reopen|comment|send|publish|deploy|apply|push|trigger|approve|reject|assign|label|upload|invite)([_./:-]|$)`,
	}, CriticalCommands: []string{
		`(?i)(^|\s)diskpart(?:\.exe)?(\s|$)`, `(?i)(^|\s)format(?:\.com)?(\s|$)`, `(?i)(^|\s)bcdedit(?:\.exe)?(\s|$)`,
		`(?i)(^|\s)shutdown(?:\.exe)?(\s|$)`, `(?i)git\s+push\b[^\r\n]*(?:--force|-f)(?:\s|$)`, `(?i)git\s+reset\s+--hard\b`,
		`(?i)git\s+clean\s+-[^\r\n]*[xX]`, `(?i)rm\s+-rf\s+/(?:\s|$|\*)`, `(?i)Remove-Item[^\r\n]+-[Rr]ecurse[^\r\n]+-[Ff]orce[^\r\n]+[A-Za-z]:\\(?:\*|$)`,
		`(?i)(terraform|tofu)\s+destroy\b`, `(?i)kubectl\s+delete\s+(namespace|ns)\b`,
	}}
}

func LoadPolicy(p paths.Paths) (Policy, error) {
	def := DefaultPolicy()
	if p.RiskPolicy == "" {
		return def, nil
	}
	pol, err := jsonx.Read(p.RiskPolicy, def)
	if err != nil {
		return def, err
	}
	if pol.Version == 0 {
		pol.Version = 1
	}
	return pol, nil
}
func SavePolicy(p paths.Paths, pol Policy) error {
	if pol.Version == 0 {
		pol.Version = 1
	}
	return jsonx.WriteAtomic(p.RiskPolicy, pol, p.BackupsRoot)
}
func EnsurePolicy(p paths.Paths) error {
	if p.RiskPolicy == "" {
		return nil
	}
	if _, err := os.Stat(p.RiskPolicy); os.IsNotExist(err) {
		return SavePolicy(p, DefaultPolicy())
	}
	return nil
}
func ResetPolicy(p paths.Paths) error { return SavePolicy(p, DefaultPolicy()) }

// Classify returns a semantic risk class for a tool call. It intentionally
// uses both the tool name and key arguments because MCP tools often encode
// destructive intent in names such as database.drop_database rather than in a
// shell command.
func Classify(call model.ToolCall, ctx Context) model.RiskDecision {
	return classify(call, ctx, DefaultPolicy())
}

func ClassifyConfigured(p paths.Paths, call model.ToolCall, ctx Context) model.RiskDecision {
	pol, err := LoadPolicy(p)
	if err != nil {
		pol = DefaultPolicy()
	}
	return classify(call, ctx, pol)
}

func classify(call model.ToolCall, ctx Context, pol Policy) model.RiskDecision {
	name := strings.TrimSpace(call.Name)
	lname := strings.ToLower(name)

	if lname == "ask_question" || lname == "ask_permission" {
		return decide(RiskWriteMedium, "routine interactive pause", ctx)
	}

	if lname == "run_command" || strings.Contains(lname, "command") || strings.Contains(lname, "terminal") {
		cmd := argString(call.Args, "CommandLine", "command", "cmd")
		for _, pattern := range pol.CriticalCommands {
			re, err := regexp.Compile(pattern)
			if err == nil && re.MatchString(cmd) {
				return model.RiskDecision{Decision: DecisionDeny, Risk: RiskCritical, Reason: "destructive/catastrophic command detected"}
			}
		}
		return decide(RiskExecHigh, "local command execution", ctx)
	}

	if isFileWriteName(lname) {
		target := argString(call.Args, "TargetFile", "path", "file", "filePath", "target")
		if target != "" && sensitiveOutsideWorkspace(target, ctx.Workspaces, ctx.Home) {
			return model.RiskDecision{Decision: DecisionDeny, Risk: RiskCritical, Reason: "write to sensitive path outside active workspace"}
		}
		return decide(RiskWriteMedium, "file mutation", ctx)
	}

	for _, pattern := range pol.CriticalToolPatterns {
		re, err := regexp.Compile(pattern)
		if err == nil && re.MatchString(lname) {
			return model.RiskDecision{Decision: DecisionDeny, Risk: RiskCritical, Reason: "semantically destructive tool action: " + name}
		}
	}

	for _, pattern := range pol.ExternalWritePatterns {
		re, err := regexp.Compile(pattern)
		if err == nil && re.MatchString(lname) {
			return decide(RiskExternalHigh, "external side-effecting tool action: "+name, ctx)
		}
	}

	if isReadOnlyName(lname) {
		return model.RiskDecision{Decision: DecisionAllow, Risk: RiskReadLow, Reason: "read-only/introspection tool"}
	}

	if looksMCP(lname) {
		// Unknown MCP tools are allowed in guarded autonomous mode so the system
		// remains no-prompt, but they are surfaced as unknown risk for telemetry.
		return decide(RiskUnknown, "unclassified MCP/tool action", ctx)
	}

	return decide(RiskUnknown, "unclassified tool action", ctx)
}

func decide(riskClass, reason string, ctx Context) model.RiskDecision {
	mode := strings.ToLower(strings.TrimSpace(ctx.PermissionMode))
	if mode == "unrestricted" || mode == "yolo" {
		return model.RiskDecision{Decision: DecisionAllow, Risk: riskClass, Reason: reason + "; unrestricted mode"}
	}
	// Guarded is intentionally no-prompt. Critical actions are denied; all
	// other classes continue autonomously and can be audited later.
	if riskClass == RiskCritical {
		return model.RiskDecision{Decision: DecisionDeny, Risk: riskClass, Reason: reason}
	}
	return model.RiskDecision{Decision: DecisionAllow, Risk: riskClass, Reason: reason}
}

func isFileWriteName(name string) bool {
	needles := []string{"write_to_file", "replace_file", "multi_replace", "edit_file", "create_file", "write_file", "patch_file"}
	for _, n := range needles {
		if strings.Contains(name, n) {
			return true
		}
	}
	return false
}

func isReadOnlyName(name string) bool {
	prefixes := []string{"read", "get", "list", "search", "find", "view", "inspect", "describe", "query", "fetch", "open", "status", "logs", "diff", "show"}
	for _, p := range prefixes {
		if name == p || strings.HasPrefix(name, p+"_") || strings.Contains(name, "."+p+"_") || strings.Contains(name, "__"+p+"_") {
			return true
		}
	}
	return false
}

func looksMCP(name string) bool {
	return strings.Contains(name, "mcp") || strings.Contains(name, "__") || strings.Contains(name, ".")
}

func argString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		for k, v := range m {
			if strings.EqualFold(k, key) {
				return fmt.Sprint(v)
			}
		}
	}
	return ""
}

func sensitiveOutsideWorkspace(target string, workspaces []string, home string) bool {
	if target == "" || home == "" {
		return false
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	abs = filepath.Clean(abs)
	for _, workspace := range workspaces {
		wa, err := filepath.Abs(workspace)
		if err == nil && isWithin(abs, filepath.Clean(wa)) {
			return false
		}
	}
	for _, rel := range []string{".ssh", ".aws", ".gnupg", ".kube", filepath.Join(".config", "gcloud")} {
		if isWithin(abs, filepath.Join(home, rel)) {
			return true
		}
	}
	return false
}

func isWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
