package fault

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

// FaultKind defines semantic categories of provider failures.
type FaultKind string

const (
	// FaultRateLimited indicates provider quota, TPM or RPM limits were reached (HTTP 429).
	FaultRateLimited FaultKind = "RATE_LIMITED"
	// FaultContextLimitExceeded indicates prompt/context exceeded model max context window.
	FaultContextLimitExceeded FaultKind = "CONTEXT_LIMIT_EXCEEDED"
	// FaultAuthentication indicates invalid credentials, unauthorized, or suspended account (HTTP 401/403).
	FaultAuthentication FaultKind = "AUTHENTICATION"
	// FaultModelUnavailable indicates requested model was not found or disabled (HTTP 404).
	FaultModelUnavailable FaultKind = "MODEL_UNAVAILABLE"
	// FaultServerOverloaded indicates provider upstream capacity is overloaded (HTTP 503).
	FaultServerOverloaded FaultKind = "SERVER_OVERLOADED"
	// FaultTransientNetwork indicates network timeouts, connection resets, or 5xx gateway errors.
	FaultTransientNetwork FaultKind = "TRANSIENT_NETWORK"
	// FaultContentFilter indicates prompt or response violated safety/content policies.
	FaultContentFilter FaultKind = "CONTENT_FILTER"
	// FaultUnknown represents unclassified provider failures.
	FaultUnknown FaultKind = "UNKNOWN"
)

// Classification contains structured diagnostics and remediation hints for a provider error.
type Classification struct {
	Kind                FaultKind               `json:"kind"`
	ErrorClass          harnessmodel.ErrorClass `json:"errorClass"`
	Message             string                  `json:"message"`
	HTTPStatusCode      int                     `json:"httpStatusCode,omitempty"`
	RetryAfter          time.Duration           `json:"retryAfter,omitempty"`
	Retryable           bool                    `json:"retryable"`
	FailoverRecommended bool                    `json:"failoverRecommended"`
	AccountScope        bool                    `json:"accountScope"`
	ModelScope          bool                    `json:"modelScope"`
	RawError            error                   `json:"-"`
}

var (
	retryAfterRegex = regexp.MustCompile(`(?i)retry[- ]after[:= ]*([0-9]+(?:\.[0-9]+)?)\s*(s|sec|seconds|ms|m|min)?`)
)

// HTTPStatusError represents an error carrying an explicit HTTP status code.
type HTTPStatusError interface {
	error
	StatusCode() int
}

// Classify categorizes any provider error into an operational Classification.
func Classify(err error) Classification {
	if err == nil {
		return Classification{
			Kind:       FaultUnknown,
			ErrorClass: harnessmodel.ErrorApplicationPermanent,
			Message:    "no error",
		}
	}

	msg := err.Error()
	lowerMsg := strings.ToLower(msg)
	statusCode := extractStatusCode(err, lowerMsg)
	retryAfter := extractRetryAfter(lowerMsg)

	// 1. Content filtering / safety violations (Strictly non-retryable application failure)
	if strings.Contains(lowerMsg, "content filter") ||
		strings.Contains(lowerMsg, "safety policy") ||
		strings.Contains(lowerMsg, "safety violation") ||
		strings.Contains(lowerMsg, "prompt blocked") ||
		strings.Contains(lowerMsg, "blocked by safety") ||
		strings.Contains(lowerMsg, "inappropriate content") {
		return Classification{
			Kind:                FaultContentFilter,
			ErrorClass:          harnessmodel.ErrorPolicyDenied,
			Message:             msg,
			HTTPStatusCode:      statusCode,
			Retryable:           false,
			FailoverRecommended: false,
			AccountScope:        false,
			ModelScope:          false,
			RawError:            err,
		}
	}

	// 2. Authentication & permissions (Permanent for the account)
	if statusCode == http.StatusUnauthorized ||
		statusCode == http.StatusForbidden ||
		strings.Contains(lowerMsg, "unauthorized") ||
		strings.Contains(lowerMsg, "invalid api key") ||
		strings.Contains(lowerMsg, "invalid_api_key") ||
		strings.Contains(lowerMsg, "authentication failed") ||
		strings.Contains(lowerMsg, "permission denied") ||
		strings.Contains(lowerMsg, "account suspended") {
		return Classification{
			Kind:                FaultAuthentication,
			ErrorClass:          harnessmodel.ErrorPolicyDenied,
			Message:             msg,
			HTTPStatusCode:      statusCode,
			Retryable:           false,
			FailoverRecommended: true,
			AccountScope:        true,
			ModelScope:          false,
			RawError:            err,
		}
	}

	// 3. Model not found or unavailable (Permanent for the model)
	if statusCode == http.StatusNotFound ||
		strings.Contains(lowerMsg, "model not found") ||
		strings.Contains(lowerMsg, "model_not_found") ||
		strings.Contains(lowerMsg, "does not exist") ||
		strings.Contains(lowerMsg, "model is deprecated") ||
		strings.Contains(lowerMsg, "model deprecated") {
		return Classification{
			Kind:                FaultModelUnavailable,
			ErrorClass:          harnessmodel.ErrorApplicationPermanent,
			Message:             msg,
			HTTPStatusCode:      statusCode,
			Retryable:           false,
			FailoverRecommended: true,
			AccountScope:        false,
			ModelScope:          true,
			RawError:            err,
		}
	}

	// 4. Context length exceeded (Non-retryable on this model without prompt compaction)
	if strings.Contains(lowerMsg, "context length") ||
		strings.Contains(lowerMsg, "maximum context length") ||
		strings.Contains(lowerMsg, "context_length_exceeded") ||
		strings.Contains(lowerMsg, "token limit exceeded") ||
		strings.Contains(lowerMsg, "max_tokens") ||
		strings.Contains(lowerMsg, "prompt is too long") ||
		strings.Contains(lowerMsg, "too many tokens") {
		return Classification{
			Kind:                FaultContextLimitExceeded,
			ErrorClass:          harnessmodel.ErrorApplicationPermanent,
			Message:             msg,
			HTTPStatusCode:      statusCode,
			Retryable:           false,
			FailoverRecommended: true,
			AccountScope:        false,
			ModelScope:          true,
			RawError:            err,
		}
	}

	// 5. Rate limiting (Retryable with backoff, or failover if account exhausted)
	if statusCode == http.StatusTooManyRequests ||
		strings.Contains(lowerMsg, "429") ||
		strings.Contains(lowerMsg, "rate limit") ||
		strings.Contains(lowerMsg, "rate_limit") ||
		strings.Contains(lowerMsg, "quota exceeded") ||
		strings.Contains(lowerMsg, "quota_exceeded") ||
		strings.Contains(lowerMsg, "resource exhausted") ||
		strings.Contains(lowerMsg, "too many requests") {
		return Classification{
			Kind:                FaultRateLimited,
			ErrorClass:          harnessmodel.ErrorRateLimited,
			Message:             msg,
			HTTPStatusCode:      statusCode,
			RetryAfter:          retryAfter,
			Retryable:           true,
			FailoverRecommended: false,
			AccountScope:        true,
			ModelScope:          false,
			RawError:            err,
		}
	}

	// 6. Server Overloaded / Upstream capacity
	if statusCode == http.StatusServiceUnavailable ||
		strings.Contains(lowerMsg, "service unavailable") ||
		strings.Contains(lowerMsg, "server overloaded") ||
		strings.Contains(lowerMsg, "overloaded") ||
		strings.Contains(lowerMsg, "capacity exceeded") ||
		strings.Contains(lowerMsg, "temporarily unavailable") {
		return Classification{
			Kind:                FaultServerOverloaded,
			ErrorClass:          harnessmodel.ErrorInfraTransient,
			Message:             msg,
			HTTPStatusCode:      statusCode,
			RetryAfter:          retryAfter,
			Retryable:           true,
			FailoverRecommended: true,
			AccountScope:        false,
			ModelScope:          false,
			RawError:            err,
		}
	}

	// 7. Transient network / 5xx gateway / timeouts
	if statusCode == http.StatusInternalServerError ||
		statusCode == http.StatusBadGateway ||
		statusCode == http.StatusGatewayTimeout ||
		strings.Contains(lowerMsg, "500") ||
		strings.Contains(lowerMsg, "502") ||
		strings.Contains(lowerMsg, "504") ||
		strings.Contains(lowerMsg, "timeout") ||
		strings.Contains(lowerMsg, "deadline exceeded") ||
		strings.Contains(lowerMsg, "connection reset") ||
		strings.Contains(lowerMsg, "connection refused") ||
		strings.Contains(lowerMsg, "eof") ||
		strings.Contains(lowerMsg, "transport error") {
		return Classification{
			Kind:                FaultTransientNetwork,
			ErrorClass:          harnessmodel.ErrorInfraTransient,
			Message:             msg,
			HTTPStatusCode:      statusCode,
			RetryAfter:          retryAfter,
			Retryable:           true,
			FailoverRecommended: false,
			AccountScope:        false,
			ModelScope:          false,
			RawError:            err,
		}
	}

	// 8. Fallback Unknown
	return Classification{
		Kind:                FaultUnknown,
		ErrorClass:          harnessmodel.ErrorApplicationTransient,
		Message:             msg,
		HTTPStatusCode:      statusCode,
		Retryable:           false,
		FailoverRecommended: false,
		AccountScope:        false,
		ModelScope:          false,
		RawError:            err,
	}
}

// ToFailure maps a Classification to the standard harness model Failure.
func (c Classification) ToFailure() harnessmodel.Failure {
	details := map[string]string{
		"faultKind": string(c.Kind),
	}
	if c.HTTPStatusCode > 0 {
		details["statusCode"] = strconv.Itoa(c.HTTPStatusCode)
	}
	return harnessmodel.Failure{
		Class:      c.ErrorClass,
		Message:    c.Message,
		RetryAfter: c.RetryAfter,
		Details:    details,
	}
}

func extractStatusCode(err error, lowerMsg string) int {
	var statusErr HTTPStatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode()
	}
	for _, code := range []int{400, 401, 403, 404, 408, 429, 500, 502, 503, 504} {
		if strings.Contains(lowerMsg, fmt.Sprintf("status %d", code)) ||
			strings.Contains(lowerMsg, fmt.Sprintf("code %d", code)) ||
			strings.Contains(lowerMsg, fmt.Sprintf("http %d", code)) ||
			strings.Contains(lowerMsg, fmt.Sprintf("error %d", code)) {
			return code
		}
	}
	return 0
}

func extractRetryAfter(msg string) time.Duration {
	match := retryAfterRegex.FindStringSubmatch(msg)
	if len(match) < 2 {
		return 0
	}
	val, err := strconv.ParseFloat(match[1], 64)
	if err != nil || val <= 0 {
		return 0
	}
	unit := "s"
	if len(match) >= 3 && match[2] != "" {
		unit = strings.ToLower(match[2])
	}
	switch unit {
	case "ms":
		return time.Duration(val * float64(time.Millisecond))
	case "m", "min":
		return time.Duration(val * float64(time.Minute))
	default:
		return time.Duration(val * float64(time.Second))
	}
}
