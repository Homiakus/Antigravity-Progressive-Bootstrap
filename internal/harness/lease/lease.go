package lease

import (
	"errors"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

var (
	ErrStaleFence   = errors.New("harness lease: stale fencing token")
	ErrLeaseExpired = errors.New("harness lease: lease expired")
)

const DefaultTTL = 30 * time.Second

func NormalizeTTL(ttl time.Duration) (time.Duration, error) {
	if ttl == 0 {
		return DefaultTTL, nil
	}
	if ttl < time.Second {
		return 0, fmt.Errorf("lease TTL must be >= 1s")
	}
	return ttl, nil
}

func Expired(current harnessmodel.Lease, now time.Time) bool {
	return !now.Before(current.ExpiresAt)
}

func ValidateAuthority(current harnessmodel.Lease, workerID harnessmodel.WorkerID, epoch uint64, now time.Time) error {
	if current.State != harnessmodel.LeaseActive || current.WorkerID != workerID || current.Epoch != epoch {
		return ErrStaleFence
	}
	if Expired(current, now) {
		return ErrLeaseExpired
	}
	return nil
}
