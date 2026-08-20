package retry

import (
	"crypto/sha256"
	"encoding/binary"
)

// DeterministicRandom maps a durable identity to a stable pseudo-random value
// in [0,1). It is intentionally stateless so a transaction retry or process
// restart computes the same jitter for the same failed Attempt.
func DeterministicRandom(key string) func() float64 {
	sum := sha256.Sum256([]byte(key))
	value := binary.BigEndian.Uint64(sum[:8]) >> 11 // 53 random bits
	const denominator = float64(uint64(1) << 53)
	stable := float64(value) / denominator
	return func() float64 { return stable }
}
