package fault

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
	harnesssqlite "github.com/homiakus/agctl/internal/harness/store/sqlite"
)

type customHTTPError struct {
	code int
	msg  string
}

func (e customHTTPError) Error() string   { return e.msg }
func (e customHTTPError) StatusCode() int { return e.code }

func setupTestDB(t testing.TB) (*harnesssqlite.DB, func()) {
	ctx := context.Background()
	db, err := harnesssqlite.Open(ctx, filepath.Join(t.TempDir(), "fault_test.db"), harnesssqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	err = db.Update(ctx, func(tx harnessstore.Tx) error {
		for _, accID := range []string{"acc_test", "acc_conc_0", "acc_conc_1", "acc_conc_2", "acc_conc_3"} {
			if err := tx.UpsertProviderAccount(ctx, harnessmodel.ProviderAccount{
				ID:        harnessmodel.ProviderAccountID(accID),
				Provider:  harnessmodel.ProviderAntigravity,
				Name:      accID,
				State:     harnessmodel.ProviderAccountActive,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return db, func() { _ = db.Close() }
}

func TestClassificationMatrix(t *testing.T) {
	tests := []struct {
		name                 string
		err                  error
		expectedKind         FaultKind
		expectedErrorClass   harnessmodel.ErrorClass
		expectedRetryable    bool
		expectedFailover     bool
		expectedRetryAfter   time.Duration
		expectedAccountScope bool
		expectedModelScope   bool
	}{
		{
			name:                 "nil error",
			err:                  nil,
			expectedKind:         FaultUnknown,
			expectedErrorClass:   harnessmodel.ErrorApplicationPermanent,
			expectedRetryable:    false,
			expectedFailover:     false,
			expectedAccountScope: false,
			expectedModelScope:   false,
		},
		{
			name:                 "safety policy content filter",
			err:                  errors.New("request rejected: prompt blocked by safety policy violation"),
			expectedKind:         FaultContentFilter,
			expectedErrorClass:   harnessmodel.ErrorPolicyDenied,
			expectedRetryable:    false,
			expectedFailover:     false,
			expectedAccountScope: false,
			expectedModelScope:   false,
		},
		{
			name:                 "authentication error HTTP 401",
			err:                  customHTTPError{code: http.StatusUnauthorized, msg: "invalid api key supplied"},
			expectedKind:         FaultAuthentication,
			expectedErrorClass:   harnessmodel.ErrorPolicyDenied,
			expectedRetryable:    false,
			expectedFailover:     true,
			expectedAccountScope: true,
			expectedModelScope:   false,
		},
		{
			name:                 "model not found HTTP 404",
			err:                  errors.New("status 404: model gemini-1.0-ultra is deprecated or does not exist"),
			expectedKind:         FaultModelUnavailable,
			expectedErrorClass:   harnessmodel.ErrorApplicationPermanent,
			expectedRetryable:    false,
			expectedFailover:     true,
			expectedAccountScope: false,
			expectedModelScope:   true,
		},
		{
			name:                 "context limit exceeded",
			err:                  errors.New("maximum context length exceeded: token limit is 32768, requested 45000"),
			expectedKind:         FaultContextLimitExceeded,
			expectedErrorClass:   harnessmodel.ErrorApplicationPermanent,
			expectedRetryable:    false,
			expectedFailover:     true,
			expectedAccountScope: false,
			expectedModelScope:   true,
		},
		{
			name:                 "rate limit 429 with retry-after seconds",
			err:                  errors.New("HTTP 429: rate limit exceeded, retry-after: 5.5s"),
			expectedKind:         FaultRateLimited,
			expectedErrorClass:   harnessmodel.ErrorRateLimited,
			expectedRetryable:    true,
			expectedFailover:     false,
			expectedRetryAfter:   5500 * time.Millisecond,
			expectedAccountScope: true,
			expectedModelScope:   false,
		},
		{
			name:                 "server overloaded 503",
			err:                  customHTTPError{code: http.StatusServiceUnavailable, msg: "server overloaded; temporarily unavailable"},
			expectedKind:         FaultServerOverloaded,
			expectedErrorClass:   harnessmodel.ErrorInfraTransient,
			expectedRetryable:    true,
			expectedFailover:     true,
			expectedAccountScope: false,
			expectedModelScope:   false,
		},
		{
			name:                 "transient network 502 bad gateway",
			err:                  errors.New("502 Bad Gateway: connection reset by peer"),
			expectedKind:         FaultTransientNetwork,
			expectedErrorClass:   harnessmodel.ErrorInfraTransient,
			expectedRetryable:    true,
			expectedFailover:     false,
			expectedAccountScope: false,
			expectedModelScope:   false,
		},
		{
			name:                 "unknown unexpected error",
			err:                  errors.New("unhandled internal runtime panic in client"),
			expectedKind:         FaultUnknown,
			expectedErrorClass:   harnessmodel.ErrorApplicationTransient,
			expectedRetryable:    false,
			expectedFailover:     false,
			expectedAccountScope: false,
			expectedModelScope:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := Classify(tc.err)
			if c.Kind != tc.expectedKind {
				t.Errorf("Kind = %v, want %v", c.Kind, tc.expectedKind)
			}
			if c.ErrorClass != tc.expectedErrorClass {
				t.Errorf("ErrorClass = %v, want %v", c.ErrorClass, tc.expectedErrorClass)
			}
			if c.Retryable != tc.expectedRetryable {
				t.Errorf("Retryable = %v, want %v", c.Retryable, tc.expectedRetryable)
			}
			if c.FailoverRecommended != tc.expectedFailover {
				t.Errorf("FailoverRecommended = %v, want %v", c.FailoverRecommended, tc.expectedFailover)
			}
			if c.RetryAfter != tc.expectedRetryAfter {
				t.Errorf("RetryAfter = %v, want %v", c.RetryAfter, tc.expectedRetryAfter)
			}
			if c.AccountScope != tc.expectedAccountScope {
				t.Errorf("AccountScope = %v, want %v", c.AccountScope, tc.expectedAccountScope)
			}
			if c.ModelScope != tc.expectedModelScope {
				t.Errorf("ModelScope = %v, want %v", c.ModelScope, tc.expectedModelScope)
			}

			failure := c.ToFailure()
			if failure.Class != tc.expectedErrorClass {
				t.Errorf("ToFailure().Class = %v, want %v", failure.Class, tc.expectedErrorClass)
			}
		})
	}
}

func TestDecisionMatrix(t *testing.T) {
	now := time.Unix(10000, 0).UTC()
	policy := DefaultPolicy()

	t.Run("content filter fails permanently", func(t *testing.T) {
		fault := Classify(errors.New("safety violation"))
		dec, err := Decide(DecisionInput{
			Fault:                fault,
			TotalAttempts:        1,
			SameProviderAttempts: 1,
			Policy:               policy,
			Now:                  now,
		})
		if err != nil {
			t.Fatal(err)
		}
		if dec.Action != ActionTerminalFail {
			t.Fatalf("Action = %v, want TERMINAL_FAIL", dec.Action)
		}
	})

	t.Run("total attempts exhausted fails permanently", func(t *testing.T) {
		fault := Classify(errors.New("502 bad gateway"))
		dec, err := Decide(DecisionInput{
			Fault:                fault,
			TotalAttempts:        5, // equal to max total
			SameProviderAttempts: 1,
			Policy:               policy,
			Now:                  now,
		})
		if err != nil {
			t.Fatal(err)
		}
		if dec.Action != ActionTerminalFail {
			t.Fatalf("Action = %v, want TERMINAL_FAIL", dec.Action)
		}
	})

	t.Run("auth failure trips account circuit and fails over", func(t *testing.T) {
		fault := Classify(errors.New("401 unauthorized"))
		dec, err := Decide(DecisionInput{
			Fault:                fault,
			TotalAttempts:        1,
			SameProviderAttempts: 1,
			Policy:               policy,
			Now:                  now,
		})
		if err != nil {
			t.Fatal(err)
		}
		if dec.Action != ActionTripCircuitAndFailover || !dec.TripCircuit || dec.TripScope != "account" {
			t.Fatalf("unexpected auth decision: %+v", dec)
		}
	})

	t.Run("context limit exceeded fails over without tripping circuit", func(t *testing.T) {
		fault := Classify(errors.New("maximum context length exceeded"))
		dec, err := Decide(DecisionInput{
			Fault:                fault,
			TotalAttempts:        1,
			SameProviderAttempts: 1,
			Policy:               policy,
			Now:                  now,
		})
		if err != nil {
			t.Fatal(err)
		}
		if dec.Action != ActionFailover || dec.TripCircuit {
			t.Fatalf("unexpected context limit decision: %+v", dec)
		}
	})

	t.Run("same provider attempt limit triggers failover", func(t *testing.T) {
		fault := Classify(errors.New("500 internal server error"))
		dec, err := Decide(DecisionInput{
			Fault:                fault,
			TotalAttempts:        3,
			SameProviderAttempts: 3, // equal to max same provider
			Policy:               policy,
			Now:                  now,
		})
		if err != nil {
			t.Fatal(err)
		}
		if dec.Action != ActionFailover {
			t.Fatalf("Action = %v, want FAILOVER", dec.Action)
		}
	})

	t.Run("circuit failure threshold trips circuit", func(t *testing.T) {
		fault := Classify(errors.New("503 service unavailable"))
		circuit := &harnessmodel.ProviderCircuitState{
			AccountID:           "acc1",
			ModelID:             "model1",
			ConsecutiveFailures: 2, // next failure reaches threshold 3
		}
		dec, err := Decide(DecisionInput{
			Fault:                fault,
			TotalAttempts:        2,
			SameProviderAttempts: 2,
			Circuit:              circuit,
			Policy:               policy,
			Now:                  now,
		})
		if err != nil {
			t.Fatal(err)
		}
		if dec.Action != ActionTripCircuitAndFailover || !dec.TripCircuit {
			t.Fatalf("unexpected threshold decision: %+v", dec)
		}
	})

	t.Run("transient error on healthy circuit retries same provider with backoff", func(t *testing.T) {
		fault := Classify(errors.New("timeout: connection deadline exceeded"))
		dec, err := Decide(DecisionInput{
			Fault:                fault,
			TotalAttempts:        1,
			SameProviderAttempts: 1,
			Policy:               policy,
			Now:                  now,
			Random:               func() float64 { return 0.5 },
		})
		if err != nil {
			t.Fatal(err)
		}
		if dec.Action != ActionRetrySame {
			t.Fatalf("Action = %v, want RETRY_SAME", dec.Action)
		}
		if dec.Delay < 250*time.Millisecond || dec.Delay > 750*time.Millisecond {
			t.Fatalf("unexpected backoff delay: %v", dec.Delay)
		}
		if !dec.NotBefore.Equal(now.Add(dec.Delay)) {
			t.Fatalf("NotBefore = %v, want %v", dec.NotBefore, now.Add(dec.Delay))
		}
	})
}

func TestCircuitManager(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Unix(2000, 0).UTC()
	cm := NewCircuitManager(db)

	accountID := harnessmodel.ProviderAccountID("acc_test")
	modelID := harnessmodel.ProviderModelID("model_test")

	// 1. First failure -> created with 1 failure, CLOSED
	st1, err := cm.RecordFailure(ctx, accountID, modelID, 3, time.Minute, now)
	if err != nil {
		t.Fatalf("RecordFailure 1 failed: %v", err)
	}
	if st1.ConsecutiveFailures != 1 || st1.State != harnessmodel.CircuitClosed {
		t.Fatalf("st1: failures=%d, state=%s", st1.ConsecutiveFailures, st1.State)
	}

	// 2. Second failure -> 2 failures, CLOSED
	now = now.Add(10 * time.Second)
	st2, err := cm.RecordFailure(ctx, accountID, modelID, 3, time.Minute, now)
	if err != nil {
		t.Fatalf("RecordFailure 2 failed: %v", err)
	}
	if st2.ConsecutiveFailures != 2 || st2.State != harnessmodel.CircuitClosed {
		t.Fatalf("st2: failures=%d, state=%s", st2.ConsecutiveFailures, st2.State)
	}

	// 3. Third failure -> 3 failures, trips to OPEN
	now = now.Add(10 * time.Second)
	st3, err := cm.RecordFailure(ctx, accountID, modelID, 3, time.Minute, now)
	if err != nil {
		t.Fatalf("RecordFailure 3 failed: %v", err)
	}
	if st3.ConsecutiveFailures != 3 || st3.State != harnessmodel.CircuitOpen {
		t.Fatalf("st3: failures=%d, state=%s", st3.ConsecutiveFailures, st3.State)
	}
	if !st3.NextProbeAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("st3 NextProbeAt = %v, want %v", st3.NextProbeAt, now.Add(time.Minute))
	}

	// 4. Success -> resets consecutive failures to 0, CLOSED
	now = now.Add(2 * time.Minute)
	if err := cm.RecordSuccess(ctx, accountID, modelID, now); err != nil {
		t.Fatalf("RecordSuccess failed: %v", err)
	}

	// Verify in DB
	var stSuccess harnessmodel.ProviderCircuitState
	if err := db.View(ctx, func(r harnessstore.Reader) error {
		var err error
		stSuccess, err = r.GetProviderCircuitState(ctx, accountID, modelID)
		return err
	}); err != nil {
		t.Fatalf("get circuit state after success: %v", err)
	}
	if stSuccess.ConsecutiveFailures != 0 || stSuccess.State != harnessmodel.CircuitClosed {
		t.Fatalf("stSuccess: failures=%d, state=%s", stSuccess.ConsecutiveFailures, stSuccess.State)
	}
}
