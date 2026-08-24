package policy

import (
	"context"
	"fmt"
	"strings"
)

type Decision string

const (
	DecisionAllow           Decision = "ALLOW"
	DecisionDeny            Decision = "DENY"
	DecisionRequireApproval Decision = "REQUIRE_APPROVAL"
)

type WorkerTrust string

const (
	TrustLocal           WorkerTrust = "TRUSTED_LOCAL"
	TrustRemote          WorkerTrust = "TRUSTED_REMOTE"
	TrustUntrustedRemote WorkerTrust = "UNTRUSTED_REMOTE"
)

func (t WorkerTrust) Level() int {
	switch t {
	case TrustLocal:
		return 3
	case TrustRemote:
		return 2
	case TrustUntrustedRemote:
		return 1
	default:
		return 0
	}
}

type SecretRef string

func (s SecretRef) Valid() bool {
	return strings.HasPrefix(string(s), "secret://")
}

func (s SecretRef) Name() string {
	return strings.TrimPrefix(string(s), "secret://")
}

type PolicyRule struct {
	Capability string      `json:"capability"`
	MinTrust   WorkerTrust `json:"minTrust,omitempty"`
	Decision   Decision    `json:"decision"`
	Reason     string      `json:"reason,omitempty"`
}

type EvaluationRequest struct {
	Capability  string      `json:"capability"`
	Risk        string      `json:"risk,omitempty"`
	WorkerTrust WorkerTrust `json:"workerTrust"`
	SecretRefs  []SecretRef `json:"secretRefs,omitempty"`
}

type EvaluationResult struct {
	Decision Decision `json:"decision"`
	Reason   string   `json:"reason"`
}

type Engine struct {
	rules []PolicyRule
}

func NewEngine(rules []PolicyRule) *Engine {
	return &Engine{rules: rules}
}

func DefaultRules() []PolicyRule {
	return []PolicyRule{
		{Capability: "filesystem.read", MinTrust: TrustUntrustedRemote, Decision: DecisionAllow},
		{Capability: "filesystem.write", MinTrust: TrustRemote, Decision: DecisionAllow},
		{Capability: "process.execute", MinTrust: TrustLocal, Decision: DecisionAllow},
		{Capability: "network.external", MinTrust: TrustRemote, Decision: DecisionAllow},
		{Capability: "github.read", MinTrust: TrustUntrustedRemote, Decision: DecisionAllow},
		{Capability: "github.write", MinTrust: TrustRemote, Decision: DecisionRequireApproval, Reason: "github write requires human approval"},
		{Capability: "github.merge", MinTrust: TrustLocal, Decision: DecisionRequireApproval, Reason: "github merge requires human approval"},
		{Capability: "deployment.production", MinTrust: TrustLocal, Decision: DecisionRequireApproval, Reason: "production deployment requires approval"},
	}
}

func (e *Engine) Evaluate(ctx context.Context, req EvaluationRequest) EvaluationResult {
	if req.Capability == "" {
		return EvaluationResult{Decision: DecisionDeny, Reason: "capability is required"}
	}

	// Secret safety: Untrusted remote workers cannot receive tasks with secrets
	if len(req.SecretRefs) > 0 {
		if req.WorkerTrust == TrustUntrustedRemote {
			return EvaluationResult{
				Decision: DecisionDeny,
				Reason:   "untrusted remote workers are forbidden from accessing secrets",
			}
		}
	}

	// Find matching rule
	for _, rule := range e.rules {
		if rule.Capability == req.Capability || rule.Capability == "*" {
			// Check trust level
			if rule.MinTrust != "" && req.WorkerTrust.Level() < rule.MinTrust.Level() {
				return EvaluationResult{
					Decision: DecisionDeny,
					Reason:   fmt.Sprintf("worker trust %s is lower than required %s for capability %s", req.WorkerTrust, rule.MinTrust, req.Capability),
				}
			}
			return EvaluationResult{
				Decision: rule.Decision,
				Reason:   rule.Reason,
			}
		}
	}

	// Default fallback: allow read, require approval for high risk, deny unknown destructive
	if strings.Contains(req.Risk, "critical") || strings.Contains(req.Risk, "high") {
		return EvaluationResult{
			Decision: DecisionRequireApproval,
			Reason:   "high risk action without explicit allow rule requires approval",
		}
	}

	return EvaluationResult{
		Decision: DecisionAllow,
		Reason:   "allowed by default baseline policy",
	}
}
