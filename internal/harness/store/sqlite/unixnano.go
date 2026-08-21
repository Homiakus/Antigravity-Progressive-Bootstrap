package sqlite

import (
	"fmt"
	"math"
	"time"
)

var (
	minUnixNanoTime = time.Unix(0, math.MinInt64).UTC()
	maxUnixNanoTime = time.Unix(0, math.MaxInt64).UTC()
)

// checkedUnixNano converts a timestamp used by an integer scheduler index.
// time.Time supports a much wider calendar range than UnixNano's signed int64
// representation; calling UnixNano outside this interval silently yields an
// undefined/wrapped value. Durable ordering must reject that state instead.
func checkedUnixNano(value time.Time) (int64, error) {
	if value.IsZero() {
		return 0, fmt.Errorf("zero time has no durable Unix-nanosecond value")
	}
	value = value.UTC()
	if value.Before(minUnixNanoTime) || value.After(maxUnixNanoTime) {
		return 0, fmt.Errorf("time %s is outside durable Unix-nanosecond range", value.Format(time.RFC3339Nano))
	}
	return value.UnixNano(), nil
}

func nullableCheckedUnixNano(value time.Time) (any, error) {
	if value.IsZero() {
		return nil, nil
	}
	ns, err := checkedUnixNano(value)
	if err != nil {
		return nil, err
	}
	return ns, nil
}

// checkedUnixNanoWindow verifies both ends of an interval so SQL expressions
// such as window_start_ns + window_ns are guaranteed to remain int64-safe.
func checkedUnixNanoWindow(start time.Time, window time.Duration) (int64, error) {
	if window <= 0 {
		return 0, fmt.Errorf("window must be positive")
	}
	startNS, err := checkedUnixNano(start)
	if err != nil {
		return 0, err
	}
	end := start.UTC().Add(window)
	if end.Before(start.UTC()) {
		return 0, fmt.Errorf("window end precedes start")
	}
	if _, err := checkedUnixNano(end); err != nil {
		return 0, fmt.Errorf("window end is outside durable range: %w", err)
	}
	return startNS, nil
}
