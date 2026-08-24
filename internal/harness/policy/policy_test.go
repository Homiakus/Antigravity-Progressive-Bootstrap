package policy

import (
	"context"
	"testing"
)

func TestPolicyEngineEvaluation(t *testing.T) {
	ctx := context.Background()
	engine := NewEngine(DefaultRules())

	// 1. Filesystem read allowed for untrusted worker
	res := engine.Evaluate(ctx, EvaluationRequest{
		Capability:  "filesystem.read",
		WorkerTrust: TrustUntrustedRemote,
	})
	if res.Decision != DecisionAllow {
		t.Fatalf("expected ALLOW for filesystem.read, got %s", res.Decision)
	}

	// 2. Process execute denied for untrusted worker
	res = engine.Evaluate(ctx, EvaluationRequest{
		Capability:  "process.execute",
		WorkerTrust: TrustUntrustedRemote,
	})
	if res.Decision != DecisionDeny {
		t.Fatalf("expected DENY for process.execute on untrusted worker, got %s", res.Decision)
	}

	// 3. Process execute allowed on trusted local worker
	res = engine.Evaluate(ctx, EvaluationRequest{
		Capability:  "process.execute",
		WorkerTrust: TrustLocal,
	})
	if res.Decision != DecisionAllow {
		t.Fatalf("expected ALLOW for process.execute on local worker, got %s", res.Decision)
	}

	// 4. Github write requires approval
	res = engine.Evaluate(ctx, EvaluationRequest{
		Capability:  "github.write",
		WorkerTrust: TrustLocal,
	})
	if res.Decision != DecisionRequireApproval {
		t.Fatalf("expected REQUIRE_APPROVAL for github.write, got %s", res.Decision)
	}

	// 5. Secret safety: untrusted remote worker forbidden from secrets
	res = engine.Evaluate(ctx, EvaluationRequest{
		Capability:  "filesystem.read",
		WorkerTrust: TrustUntrustedRemote,
		SecretRefs:  []SecretRef{"secret://vault/api_key"},
	})
	if res.Decision != DecisionDeny {
		t.Fatalf("expected DENY for untrusted worker with secrets, got %s", res.Decision)
	}
}
