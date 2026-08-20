package retry

import (
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestCircuitClosedOpenHalfOpenClosed(t *testing.T) {
	now := time.Unix(7000, 0).UTC()
	policy := BreakerPolicy{FailureThreshold: 2, Cooldown: 30 * time.Second}
	b := harnessmodel.CircuitBreaker{ServiceKey: "github-api", State: harnessmodel.CircuitClosed, FailureThreshold: 2, UpdatedAt: now}

	var err error
	b, err = RecordFailure(b, policy, now)
	if err != nil {
		t.Fatal(err)
	}
	if b.State != harnessmodel.CircuitClosed || b.ConsecutiveFailures != 1 {
		t.Fatalf("first failure opened breaker: %+v", b)
	}
	b, err = RecordFailure(b, policy, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if b.State != harnessmodel.CircuitOpen || b.ConsecutiveFailures != 2 || b.NextProbeAt.IsZero() {
		t.Fatalf("threshold did not open breaker: %+v", b)
	}

	blocked, err := Allow(b, now.Add(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if blocked.Allow {
		t.Fatalf("open breaker allowed early call: %+v", blocked)
	}

	probe, err := Allow(b, b.NextProbeAt)
	if err != nil {
		t.Fatal(err)
	}
	if !probe.Allow || !probe.Probe || probe.Breaker.State != harnessmodel.CircuitHalfOpen || !probe.Breaker.ProbeInFlight {
		t.Fatalf("probe not acquired: %+v", probe)
	}
	secondProbe, err := Allow(probe.Breaker, b.NextProbeAt)
	if err != nil {
		t.Fatal(err)
	}
	if secondProbe.Allow {
		t.Fatalf("second half-open probe was allowed: %+v", secondProbe)
	}

	b, err = RecordSuccess(probe.Breaker, b.NextProbeAt.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if b.State != harnessmodel.CircuitClosed || b.ConsecutiveFailures != 0 || b.ProbeInFlight || !b.NextProbeAt.IsZero() {
		t.Fatalf("successful probe did not close/reset breaker: %+v", b)
	}
}

func TestHalfOpenFailureReopensCircuit(t *testing.T) {
	now := time.Unix(8000, 0).UTC()
	policy := BreakerPolicy{FailureThreshold: 3, Cooldown: time.Minute}
	b := harnessmodel.CircuitBreaker{ServiceKey: "llm-provider", State: harnessmodel.CircuitHalfOpen, ProbeInFlight: true, ConsecutiveFailures: 3, FailureThreshold: 3, UpdatedAt: now}
	b, err := RecordFailure(b, policy, now)
	if err != nil {
		t.Fatal(err)
	}
	if b.State != harnessmodel.CircuitOpen || b.ProbeInFlight || !b.NextProbeAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("half-open failure did not reopen circuit: %+v", b)
	}
}
