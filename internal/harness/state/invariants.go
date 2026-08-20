package state

import (
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func NewRetryAttempt(previous harnessmodel.Attempt, previousState AttemptState, newID harnessmodel.AttemptID, at time.Time) (harnessmodel.Attempt, error) {
	if !previousState.Terminal() {
		return harnessmodel.Attempt{}, fmt.Errorf("cannot retry non-terminal attempt in state %s", previousState)
	}
	if previous.Number < 1 {
		return harnessmodel.Attempt{}, fmt.Errorf("previous attempt number must be >= 1")
	}
	if newID == "" || newID == previous.ID {
		return harnessmodel.Attempt{}, fmt.Errorf("retry requires a distinct attempt id")
	}
	return harnessmodel.Attempt{
		ID:        newID,
		NodeRunID: previous.NodeRunID,
		Number:    previous.Number + 1,
		State:     harnessmodel.AttemptCreated,
		CreatedAt: at.UTC(),
	}, nil
}
