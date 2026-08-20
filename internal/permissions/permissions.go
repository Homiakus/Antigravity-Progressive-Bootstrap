package permissions

import (
	"fmt"
	"sort"
	"strings"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/paths"
)

type Profile struct {
	Name           string
	ToolPermission string
	ArtifactReview string
	Sandbox        bool
	NonWorkspace   bool
	Allow          []string
	Ask            []string
	Deny           []string
}

var destructiveDenies = []string{
	"command(rm -rf)",
	"command(sudo)",
	"command(diskpart)",
	"command(format)",
	"command(git reset --hard)",
	"command(git clean -fd)",
	"command(git clean -fdx)",
	"command(git push --force)",
	"command(git push -f)",
	"write_file(.git/)",
	"read_file(.ssh/)",
	"write_file(.ssh/)",
	"read_file(.aws/)",
	"write_file(.aws/)",
	"read_file(.gnupg/)",
	"write_file(.gnupg/)",
	"read_file(.kube/)",
	"write_file(.kube/)",
}

func GetProfile(name string) (Profile, error) {
	switch name {
	case "safe":
		return Profile{Name: name, ToolPermission: "request-review", ArtifactReview: "asks-for-review", Sandbox: true, NonWorkspace: false, Deny: destructiveDenies}, nil
	case "balanced":
		return Profile{Name: name, ToolPermission: "proceed-in-sandbox", ArtifactReview: "agent-decides", Sandbox: true, NonWorkspace: false,
			Allow: []string{"command(git)", "command(go)", "command(gofmt)", "command(gopls)", "command(npm)", "command(npx)", "mcp(*)"},
			Deny:  destructiveDenies}, nil
	case "autonomous":
		return Profile{Name: name, ToolPermission: "always-proceed", ArtifactReview: "always-proceed", Sandbox: true, NonWorkspace: false,
			Allow: []string{"command(*)", "mcp(*)", "read_url(*)", "execute_url(*)", "read_file(*)", "write_file(*)"},
			Ask:   []string{}, Deny: destructiveDenies}, nil
	case "yolo", "unrestricted":
		return Profile{Name: "yolo", ToolPermission: "always-proceed", ArtifactReview: "always-proceed", Sandbox: false, NonWorkspace: true,
			Allow: []string{"command(*)", "mcp(*)", "read_url(*)", "execute_url(*)", "read_file(*)", "write_file(*)", "unsandboxed(*)"},
			Ask:   []string{}, Deny: []string{}}, nil
	default:
		return Profile{}, fmt.Errorf("unknown permission profile %q", name)
	}
}

func Apply(p paths.Paths, name string) error {
	prof, err := GetProfile(name)
	if err != nil {
		return err
	}
	settings, err := jsonx.ReadMap(p.CLISettings)
	if err != nil {
		return err
	}
	settings["toolPermission"] = prof.ToolPermission
	settings["artifactReviewPolicy"] = prof.ArtifactReview
	settings["enableTerminalSandbox"] = prof.Sandbox
	settings["allowNonWorkspaceAccess"] = prof.NonWorkspace
	settings["permissions"] = map[string]any{
		"allow": prof.Allow,
		"ask":   prof.Ask,
		"deny":  prof.Deny,
	}
	return jsonx.WriteAtomic(p.CLISettings, settings, p.BackupsRoot)
}

type Audit struct {
	ToolPermission string
	ArtifactReview string
	Sandbox        any
	NonWorkspace   any
	Allow          []string
	Ask            []string
	Deny           []string
	Conflicts      []string
}

func AuditSettings(p paths.Paths) (Audit, error) {
	m, err := jsonx.ReadMap(p.CLISettings)
	if err != nil {
		return Audit{}, err
	}
	a := Audit{ToolPermission: str(m["toolPermission"]), ArtifactReview: str(m["artifactReviewPolicy"]), Sandbox: m["enableTerminalSandbox"], NonWorkspace: m["allowNonWorkspaceAccess"]}
	if perms, ok := m["permissions"].(map[string]any); ok {
		a.Allow = stringSlice(perms["allow"])
		a.Ask = stringSlice(perms["ask"])
		a.Deny = stringSlice(perms["deny"])
	}
	// Exact duplicate rules are guaranteed conflicts due to Deny > Ask > Allow.
	allowSet := set(a.Allow)
	askSet := set(a.Ask)
	denySet := set(a.Deny)
	for rule := range allowSet {
		if _, ok := askSet[rule]; ok {
			a.Conflicts = append(a.Conflicts, "ASK overrides ALLOW: "+rule)
		}
		if _, ok := denySet[rule]; ok {
			a.Conflicts = append(a.Conflicts, "DENY overrides ALLOW: "+rule)
		}
	}
	// Broad ask/deny wildcards also shadow narrower allows in the same namespace.
	for _, broad := range []string{"command(*)", "mcp(*)", "read_url(*)", "execute_url(*)", "read_file(*)", "write_file(*)", "unsandboxed(*)"} {
		ns := strings.SplitN(broad, "(", 2)[0] + "("
		if _, ok := askSet[broad]; ok {
			for rule := range allowSet {
				if strings.HasPrefix(rule, ns) {
					a.Conflicts = append(a.Conflicts, "broad ASK shadows ALLOW: "+broad+" > "+rule)
				}
			}
		}
		if _, ok := denySet[broad]; ok {
			for rule := range allowSet {
				if strings.HasPrefix(rule, ns) {
					a.Conflicts = append(a.Conflicts, "broad DENY shadows ALLOW: "+broad+" > "+rule)
				}
			}
		}
	}
	sort.Strings(a.Conflicts)
	a.Conflicts = unique(a.Conflicts)
	return a, nil
}

func NoPromptsChecklist() string {
	return `Antigravity no-prompts checklist

CLI/global documented settings:
- toolPermission = always-proceed
- artifactReviewPolicy = always-proceed
- permissions.ask = []
- allow command(*), mcp(*), read_url(*), execute_url(*), read_file(*), write_file(*)
- add unsandboxed(*) only if you intentionally disable containment

Permission precedence is Deny > Ask > Allow. A broad command(*) or mcp(*) in Ask/Deny will shadow narrower Allow rules.

For a fully unattended desktop Project, also verify the current Project UI has no stricter project override and Artifact Review is set to Always Proceed. Hooks can auto-allow tool calls but cannot override platform/organization guardrails or external authentication.`
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func stringSlice(v any) []string {
	var out []string
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			out = append(out, fmt.Sprint(e))
		}
	case []string:
		out = append(out, x...)
	}
	return out
}
func set(xs []string) map[string]struct{} {
	m := map[string]struct{}{}
	for _, x := range xs {
		m[x] = struct{}{}
	}
	return m
}
func unique(xs []string) []string {
	if len(xs) == 0 {
		return xs
	}
	out := []string{xs[0]}
	for _, x := range xs[1:] {
		if x != out[len(out)-1] {
			out = append(out, x)
		}
	}
	return out
}
