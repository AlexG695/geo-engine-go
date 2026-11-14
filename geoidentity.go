package geoengine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// RuleAction is the verdict applied when a GeoIdentity rule matches.
type RuleAction string

// Supported GeoIdentity rule actions.
const (
	RuleActionAllow     RuleAction = "ALLOW"
	RuleActionBlock     RuleAction = "BLOCK"
	RuleActionChallenge RuleAction = "CHALLENGE"
)

// SubjectType identifies who a GeoIdentity rule applies to.
type SubjectType string

// Supported GeoIdentity subject types.
const (
	SubjectUser   SubjectType = "USER"
	SubjectRole   SubjectType = "ROLE"
	SubjectDevice SubjectType = "DEVICE"
	// SubjectAll applies the rule to every member of the organization.
	SubjectAll SubjectType = "ALL"
)

// SubjectAssignment binds a rule to a subject. For SubjectAll the SubjectID is "*".
type SubjectAssignment struct {
	SubjectType SubjectType `json:"subject_type"`
	SubjectID   string      `json:"subject_id"`
}

// Rule is a GeoIdentity access policy.
type Rule struct {
	ID              string              `json:"id"`
	OrgID           string              `json:"org_id"`
	Name            string              `json:"name"`
	Description     string              `json:"description,omitempty"`
	Priority        int                 `json:"priority"`
	Action          RuleAction          `json:"action"`
	IsActive        bool                `json:"is_active"`
	WhiteIPList     []string            `json:"white_ip_list"`
	BlackIPList     []string            `json:"black_ip_list"`
	BlockDatacenter bool                `json:"block_datacenter"`
	BlockTor        bool                `json:"block_tor"`
	RiskAccepted    int                 `json:"risk_accepted"`
	GeoLimit        json.RawMessage     `json:"geo_limit,omitempty"`
	Assignments     []SubjectAssignment `json:"assigned_subjects,omitempty"`
	SubjectsCount   int                 `json:"subjects_count"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
}

// CreateRuleRequest is the payload for GeoIdentityService.CreateRule.
// When AssignedSubjects is empty the rule applies to the whole organization.
type CreateRuleRequest struct {
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Priority    *int       `json:"priority,omitempty"`
	Action      RuleAction `json:"action,omitempty"`
	IsActive    *bool      `json:"is_active,omitempty"`
	// WhiteIPList and BlackIPList accept IPs or CIDR ranges.
	WhiteIPList     []string `json:"white_ip_list,omitempty"`
	BlackIPList     []string `json:"black_ip_list,omitempty"`
	BlockDatacenter bool     `json:"block_datacenter,omitempty"`
	BlockTor        bool     `json:"block_tor,omitempty"`
	RiskAccepted    int      `json:"risk_accepted,omitempty"`
	// GeoLimit is a GeoJSON Polygon or MultiPolygon. See PolygonFromLatLng.
	GeoLimit         json.RawMessage     `json:"geo_limit,omitempty"`
	AssignedSubjects []SubjectAssignment `json:"assigned_subjects,omitempty"`
}

// UpdateRuleRequest is the payload for GeoIdentityService.UpdateRule.
// Nil fields are left unchanged. To remove the spatial boundary set GeoLimit to json.RawMessage("null").
type UpdateRuleRequest struct {
	Name             *string              `json:"name,omitempty"`
	Description      *string              `json:"description,omitempty"`
	Priority         *int                 `json:"priority,omitempty"`
	Action           *RuleAction          `json:"action,omitempty"`
	IsActive         *bool                `json:"is_active,omitempty"`
	WhiteIPList      *[]string            `json:"white_ip_list,omitempty"`
	BlackIPList      *[]string            `json:"black_ip_list,omitempty"`
	BlockDatacenter  *bool                `json:"block_datacenter,omitempty"`
	BlockTor         *bool                `json:"block_tor,omitempty"`
	RiskAccepted     *int                 `json:"risk_accepted,omitempty"`
	GeoLimit         json.RawMessage      `json:"geo_limit,omitempty"`
	AssignedSubjects *[]SubjectAssignment `json:"assigned_subjects,omitempty"`
}

// ListRulesOptions filters GeoIdentityService.ListRules. A nil IsActive returns every rule.
type ListRulesOptions struct {
	IsActive *bool
}

// GeoIdentityService manages GeoIdentity access rules and assigns them to users or to the whole organization.
// Safe for concurrent use across multiple goroutines.
type GeoIdentityService struct {
	client *Client
}

func newGeoIdentityService(client *Client) *GeoIdentityService {
	return &GeoIdentityService{client: client}
}

const geoIdentityRulesPath = "/api/users/geoidentity/rules"

// CreateRule creates a GeoIdentity rule.
func (s *GeoIdentityService) CreateRule(ctx context.Context, req CreateRuleRequest) (*Rule, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, ErrRuleNameRequired
	}
	var rule Rule
	if err := s.client.doAPIServiceRequest(ctx, http.MethodPost, geoIdentityRulesPath, req, &rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

// ListRules returns the organization's rules ordered by priority.
// Use GetRule to load a rule's assigned subjects.
func (s *GeoIdentityService) ListRules(ctx context.Context, opts *ListRulesOptions) ([]Rule, error) {
	path := geoIdentityRulesPath
	if opts != nil && opts.IsActive != nil {
		path += "?is_active=" + strconv.FormatBool(*opts.IsActive)
	}
	var rules []Rule
	if err := s.client.doAPIServiceRequest(ctx, http.MethodGet, path, nil, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// GetRule returns a rule including its assigned subjects.
func (s *GeoIdentityService) GetRule(ctx context.Context, ruleID string) (*Rule, error) {
	path, err := rulePath(ruleID, "")
	if err != nil {
		return nil, err
	}
	var rule Rule
	if err := s.client.doAPIServiceRequest(ctx, http.MethodGet, path, nil, &rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

// UpdateRule partially updates a rule.
func (s *GeoIdentityService) UpdateRule(ctx context.Context, ruleID string, req UpdateRuleRequest) (*Rule, error) {
	path, err := rulePath(ruleID, "")
	if err != nil {
		return nil, err
	}
	var rule Rule
	if err := s.client.doAPIServiceRequest(ctx, http.MethodPatch, path, req, &rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

// DeleteRule deletes a rule and all of its assignments.
func (s *GeoIdentityService) DeleteRule(ctx context.Context, ruleID string) error {
	path, err := rulePath(ruleID, "")
	if err != nil {
		return err
	}
	return s.client.doAPIServiceRequest(ctx, http.MethodDelete, path, nil, nil)
}

// AssignToUser applies the rule to a member of the organization. Assigning twice is a no-op.
func (s *GeoIdentityService) AssignToUser(ctx context.Context, ruleID, userID string) (*Rule, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrUserIDRequired
	}
	return s.changeAssignment(ctx, http.MethodPost, ruleID, assignmentPayload{Scope: "user", UserID: strings.TrimSpace(userID)})
}

// AssignToOrganization applies the rule to every member of the organization.
func (s *GeoIdentityService) AssignToOrganization(ctx context.Context, ruleID string) (*Rule, error) {
	return s.changeAssignment(ctx, http.MethodPost, ruleID, assignmentPayload{Scope: "organization"})
}

// UnassignFromUser removes a user assignment from the rule.
// It returns ErrLastSubject if it is the rule's only subject; deactivate or delete the rule instead.
func (s *GeoIdentityService) UnassignFromUser(ctx context.Context, ruleID, userID string) (*Rule, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrUserIDRequired
	}
	return s.changeAssignment(ctx, http.MethodDelete, ruleID, assignmentPayload{Scope: "user", UserID: strings.TrimSpace(userID)})
}

// UnassignFromOrganization removes the organization-wide assignment from the rule.
// It returns ErrLastSubject if it is the rule's only subject.
func (s *GeoIdentityService) UnassignFromOrganization(ctx context.Context, ruleID string) (*Rule, error) {
	return s.changeAssignment(ctx, http.MethodDelete, ruleID, assignmentPayload{Scope: "organization"})
}

// ListUserRules returns the rules that apply to a user: assigned directly, through the user's role,
// or to the whole organization.
func (s *GeoIdentityService) ListUserRules(ctx context.Context, userID string) ([]Rule, error) {
	path, err := userPath(userID, "/geoidentity/rules")
	if err != nil {
		return nil, err
	}
	var rules []Rule
	if err := s.client.doAPIServiceRequest(ctx, http.MethodGet, path, nil, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

type assignmentPayload struct {
	Scope  string `json:"scope"`
	UserID string `json:"user_id,omitempty"`
}

func (s *GeoIdentityService) changeAssignment(ctx context.Context, method, ruleID string, payload assignmentPayload) (*Rule, error) {
	path, err := rulePath(ruleID, "/assignments")
	if err != nil {
		return nil, err
	}
	var rule Rule
	if err := s.client.doAPIServiceRequest(ctx, method, path, payload, &rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

func rulePath(ruleID, suffix string) (string, error) {
	id := strings.TrimSpace(ruleID)
	if id == "" {
		return "", ErrRuleIDRequired
	}
	return geoIdentityRulesPath + "/" + url.PathEscape(id) + suffix, nil
}

// PolygonFromLatLng builds a GeoJSON Polygon for CreateRuleRequest.GeoLimit from [lat, lng] pairs,
// the same order used by Client.CreateGeofence. The ring is closed automatically.
func PolygonFromLatLng(coordinates [][]float64) (json.RawMessage, error) {
	if len(coordinates) < 3 {
		return nil, ErrInvalidCoordinates
	}

	ring := make([][]float64, 0, len(coordinates)+1)
	for _, p := range coordinates {
		if len(p) != 2 {
			return nil, fmt.Errorf("geoengine: invalid coordinate %v (expected [lat, lng])", p)
		}
		if err := ValidateCoordinates(p[0], p[1]); err != nil {
			return nil, err
		}
		ring = append(ring, []float64{p[1], p[0]})
	}

	first, last := ring[0], ring[len(ring)-1]
	if first[0] != last[0] || first[1] != last[1] {
		ring = append(ring, []float64{first[0], first[1]})
	}

	return json.Marshal(map[string]any{
		"type":        "Polygon",
		"coordinates": [][][]float64{ring},
	})
}
