package state

import harnessmodel "github.com/homiakus/agctl/internal/harness/model"

type AttemptState = harnessmodel.AttemptState

const (
	AttemptCreated   = harnessmodel.AttemptCreated
	AttemptClaimed   = harnessmodel.AttemptClaimed
	AttemptRunning   = harnessmodel.AttemptRunning
	AttemptSucceeded = harnessmodel.AttemptSucceeded
	AttemptFailed    = harnessmodel.AttemptFailed
	AttemptTimedOut  = harnessmodel.AttemptTimedOut
	AttemptCancelled = harnessmodel.AttemptCancelled
	AttemptLost      = harnessmodel.AttemptLost
	AttemptInDoubt   = harnessmodel.AttemptInDoubt
)
