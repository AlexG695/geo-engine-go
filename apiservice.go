package geoengine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrUserIDRequired is returned when a user ID is missing or empty.
	ErrUserIDRequired = errors.New("geoengine: user_id is required")

	// ErrRuleIDRequired is returned when a GeoIdentity rule ID is missing or empty.
	ErrRuleIDRequired = errors.New("geoengine: rule_id is required")

	// ErrRuleNameRequired is returned when a GeoIdentity rule name is missing or empty.
	ErrRuleNameRequired = errors.New("geoengine: rule name is required")

	// ErrEmailRequired is returned when an invitation email is missing or empty.
	ErrEmailRequired = errors.New("geoengine: email is required")

	// ErrRoleRequired is returned when a role is missing or empty.
	ErrRoleRequired = errors.New("geoengine: role is required")

	// ErrUserNotFound is returned when the user does not exist in the organization.
	ErrUserNotFound = errors.New("geoengine: user not found in organization")

	// ErrRuleNotFound is returned when the GeoIdentity rule does not exist.
	ErrRuleNotFound = errors.New("geoengine: geoidentity rule not found")

	// ErrUserAlreadyExists is returned when inviting an email that already belongs to the organization.
	ErrUserAlreadyExists = errors.New("geoengine: user already belongs to the organization")

	// ErrInvitationPending is returned when a pending invitation already exists for the email.
	ErrInvitationPending = errors.New("geoengine: a pending invitation already exists for this email")

	// ErrLastAdmin is returned when an operation would leave the organization without an admin.
	ErrLastAdmin = errors.New("geoengine: organization must keep at least one admin")

	// ErrLastSubject is returned when an operation would leave a rule without assigned subjects.
	ErrLastSubject = errors.New("geoengine: rule must keep at least one assigned subject")
)

// APIServiceError is a structured error returned by the api-service Users and GeoIdentity endpoints.
// Use errors.Is with the package sentinel errors (for example ErrUserNotFound) to branch on it.
type APIServiceError struct {
	StatusCode int    `json:"status_code"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

// Error formats the error into a readable string with a standard prefix.
func (e *APIServiceError) Error() string {
	if e == nil {
		return "geoengine: <nil APIServiceError>"
	}
	if e.Code != "" {
		return fmt.Sprintf("geoengine: api-service error (status %d, code %s): %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("geoengine: api-service error (status %d): %s", e.StatusCode, e.Message)
}

// Is enables errors.Is matching against the package sentinel errors.
func (e *APIServiceError) Is(target error) bool {
	if e == nil {
		return false
	}
	return target != nil && e.sentinel() == target
}

// Unwrap returns the sentinel error that corresponds to the error code, if any.
func (e *APIServiceError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.sentinel()
}

func (e *APIServiceError) sentinel() error {
	switch e.Code {
	case "USER_NOT_FOUND", "USER_REVOKED":
		return ErrUserNotFound
	case "RULE_NOT_FOUND":
		return ErrRuleNotFound
	case "USER_ALREADY_EXISTS":
		return ErrUserAlreadyExists
	case "INVITATION_PENDING":
		return ErrInvitationPending
	case "LAST_ADMIN":
		return ErrLastAdmin
	case "LAST_SUBJECT":
		return ErrLastSubject
	case "QUOTA_EXCEEDED":
		return ErrQuotaExceeded
	case "MISSING_API_KEY", "MISSING_ORGANIZATION":
		return ErrUnauthorized
	}
	switch e.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusPaymentRequired:
		return ErrQuotaExceeded
	}
	return nil
}

// doAPIServiceRequest sends a JSON request to the api-service and decodes the "data" envelope into target.
// payload and target may be nil.
func (c *Client) doAPIServiceRequest(ctx context.Context, method, path string, payload, target any) error {
	c.mu.RLock()
	closed := c.closed
	c.mu.RUnlock()
	if closed {
		return ErrClientClosed
	}

	var body io.Reader
	if payload != nil {
		buf := new(bytes.Buffer)
		if err := json.NewEncoder(buf).Encode(payload); err != nil {
			return fmt.Errorf("geoengine: failed to encode request payload: %w", err)
		}
		body = buf
	}

	endpoint := strings.TrimRight(c.opts.APIServiceURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("geoengine: failed to create request: %w", err)
	}

	now := time.Now().UnixMilli()
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", c.opts.APIKey)
	req.Header.Set("X-Geo-Environment", c.opts.Environment)
	req.Header.Set("X-Request-Time", strconv.FormatInt(now, 10))
	req.Header.Set("X-Signature", c.generateHMACSignature(c.opts.APIKey, now))
	req.Header.Set("User-Agent", userAgent)
	if c.opts.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.opts.JWTToken)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("geoengine: request canceled or deadline exceeded: %w", ctx.Err())
		}
		return fmt.Errorf("geoengine: network error: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("geoengine: failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return parseAPIServiceError(resp.StatusCode, respBody)
	}

	if target == nil {
		return nil
	}

	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("geoengine: failed to parse response JSON: %w", err)
	}
	if !envelope.Success {
		return parseAPIServiceError(resp.StatusCode, respBody)
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		return fmt.Errorf("geoengine: failed to decode response data: %w", err)
	}
	return nil
}

func parseAPIServiceError(statusCode int, body []byte) *APIServiceError {
	apiErr := &APIServiceError{StatusCode: statusCode}

	var parsed struct {
		Error *apiErrorContainer `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error != nil {
		apiErr.Message = parsed.Error.Message
		apiErr.Code = parsed.Error.Status
		for _, d := range parsed.Error.Details {
			if d.Reason != "" {
				apiErr.Code = d.Reason
				break
			}
		}
	} else {
		apiErr.Message = strings.TrimSpace(string(body))
	}

	if apiErr.Message == "" {
		apiErr.Message = http.StatusText(statusCode)
	}
	return apiErr
}
