package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	planDigestRE = regexp.MustCompile(`^[0-9a-f]{64}$`)
	// ErrStalePlan indicates that an envelope or provider assignment was bound to a plan
	// revision that does not match the currently active living plan.
	ErrStalePlan = errors.New("stale plan execution: plan digest mismatch")
)

// ComputePlanDigest calculates the canonical SHA-256 hexadecimal digest of a plan's content.
func ComputePlanDigest(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// ValidatePlanDigest verifies that digest is a valid 64-character lowercase hexadecimal SHA-256 string.
func ValidatePlanDigest(digest string) error {
	trimmed := strings.TrimSpace(digest)
	if trimmed == "" {
		return fmt.Errorf("plan digest is required")
	}
	if !planDigestRE.MatchString(trimmed) {
		return fmt.Errorf("invalid plan digest %q: must be a 64-character lowercase hex SHA-256 string", digest)
	}
	return nil
}

// VerifyPlanConsistency checks whether the envelope's plan digest matches the SHA-256
// digest of the active plan content. If they differ, ErrStalePlan is returned.
func VerifyPlanConsistency(envelopePlanDigest string, currentPlanContent []byte) error {
	if err := ValidatePlanDigest(envelopePlanDigest); err != nil {
		return err
	}
	activeDigest := ComputePlanDigest(currentPlanContent)
	if envelopePlanDigest != activeDigest {
		return fmt.Errorf("%w: envelope=%s active=%s", ErrStalePlan, envelopePlanDigest, activeDigest)
	}
	return nil
}
