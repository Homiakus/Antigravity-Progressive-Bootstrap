package sqlite

import (
	"context"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func (t *transaction) GetRetryScheduleByAttempt(ctx context.Context, attemptID harnessmodel.AttemptID) (harnessmodel.RetrySchedule, error) {
	return scanRetrySchedule(t.tx.QueryRowContext(ctx, `
SELECT node_run_id, workflow_run_id, failed_attempt_id, attempt_number, failure_class, policy_ref, service_key, scheduled_at, not_before
FROM retry_schedule_history
WHERE failed_attempt_id=?`, string(attemptID)))
}
