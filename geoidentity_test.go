package geoengine

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

const ruleJSON = `{"id":"r1","org_id":"o1","name":"Solo CDMX","priority":10,"action":"BLOCK","is_active":true,
	"white_ip_list":[],"black_ip_list":["10.0.0.0/8"],"assigned_subjects":[{"subject_type":"USER","subject_id":"u1"}],"subjects_count":1}`

func TestGeoIdentityCreateRule(t *testing.T) {
	geo, err := PolygonFromLatLng([][]float64{{19.40, -99.20}, {19.40, -99.10}, {19.50, -99.10}})
	if err != nil {
		t.Fatalf("PolygonFromLatLng: %v", err)
	}
	priority := 10

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/users/geoidentity/rules" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got["name"] != "Solo CDMX" || got["action"] != "BLOCK" || got["priority"] != float64(10) {
			t.Errorf("unexpected payload: %v", got)
		}
		if _, ok := got["is_active"]; ok {
			t.Errorf("nil IsActive must be omitted")
		}
		subjects, _ := got["assigned_subjects"].([]any)
		if len(subjects) != 1 {
			t.Errorf("expected 1 subject, got %v", got["assigned_subjects"])
		}
		writeJSON(w, http.StatusCreated, `{"success":true,"data":`+ruleJSON+`}`)
	})

	rule, err := c.GeoIdentity.CreateRule(context.Background(), CreateRuleRequest{
		Name:             "Solo CDMX",
		Priority:         &priority,
		Action:           RuleActionBlock,
		BlackIPList:      []string{"10.0.0.0/8"},
		GeoLimit:         geo,
		AssignedSubjects: []SubjectAssignment{{SubjectType: SubjectUser, SubjectID: "u1"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.ID != "r1" || rule.Action != RuleActionBlock || len(rule.Assignments) != 1 {
		t.Fatalf("unexpected rule: %+v", rule)
	}
}

func TestGeoIdentityCreateRuleRequiresName(t *testing.T) {
	c := New("k", WithAPIServiceURL("http://127.0.0.1:0"))
	defer c.Close()
	if _, err := c.GeoIdentity.CreateRule(context.Background(), CreateRuleRequest{Name: " "}); !errors.Is(err, ErrRuleNameRequired) {
		t.Fatalf("expected ErrRuleNameRequired, got %v", err)
	}
}

func TestGeoIdentityListRulesFilter(t *testing.T) {
	active := false
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/users/geoidentity/rules" || r.URL.Query().Get("is_active") != "false" {
			t.Errorf("unexpected request %s", r.URL.String())
		}
		writeJSON(w, http.StatusOK, `{"success":true,"data":[`+ruleJSON+`]}`)
	})

	rules, err := c.GeoIdentity.ListRules(context.Background(), &ListRulesOptions{IsActive: &active})
	if err != nil || len(rules) != 1 {
		t.Fatalf("unexpected result: %v %v", rules, err)
	}
}

func TestGeoIdentityUpdateRuleRemovesGeoLimit(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/users/geoidentity/rules/r1" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var got map[string]json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&got)
		if string(got["geo_limit"]) != "null" {
			t.Errorf("expected explicit geo_limit null, got %s", got["geo_limit"])
		}
		if _, ok := got["name"]; ok {
			t.Errorf("nil fields must be omitted")
		}
		writeJSON(w, http.StatusOK, `{"success":true,"data":`+ruleJSON+`}`)
	})

	if _, err := c.GeoIdentity.UpdateRule(context.Background(), "r1", UpdateRuleRequest{GeoLimit: json.RawMessage("null")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGeoIdentityAssignments(t *testing.T) {
	tests := []struct {
		name       string
		call       func(*GeoIdentityService) (*Rule, error)
		wantMethod string
		wantBody   assignmentPayload
	}{
		{
			name:       "assign to user",
			call:       func(s *GeoIdentityService) (*Rule, error) { return s.AssignToUser(context.Background(), "r1", "u2") },
			wantMethod: http.MethodPost,
			wantBody:   assignmentPayload{Scope: "user", UserID: "u2"},
		},
		{
			name:       "assign to organization",
			call:       func(s *GeoIdentityService) (*Rule, error) { return s.AssignToOrganization(context.Background(), "r1") },
			wantMethod: http.MethodPost,
			wantBody:   assignmentPayload{Scope: "organization"},
		},
		{
			name: "unassign from user",
			call: func(s *GeoIdentityService) (*Rule, error) {
				return s.UnassignFromUser(context.Background(), "r1", "u2")
			},
			wantMethod: http.MethodDelete,
			wantBody:   assignmentPayload{Scope: "user", UserID: "u2"},
		},
		{
			name: "unassign from organization",
			call: func(s *GeoIdentityService) (*Rule, error) {
				return s.UnassignFromOrganization(context.Background(), "r1")
			},
			wantMethod: http.MethodDelete,
			wantBody:   assignmentPayload{Scope: "organization"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.wantMethod || r.URL.Path != "/api/users/geoidentity/rules/r1/assignments" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				var got assignmentPayload
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if got != tt.wantBody {
					t.Errorf("expected body %+v, got %+v", tt.wantBody, got)
				}
				writeJSON(w, http.StatusOK, `{"success":true,"data":`+ruleJSON+`}`)
			})

			if _, err := tt.call(c.GeoIdentity); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestGeoIdentityErrors(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/users/geoidentity/rules/r1/assignments":
			writeJSON(w, http.StatusConflict, apiErrorBody(http.StatusConflict, "LAST_SUBJECT", "La regla debe conservar al menos un sujeto"))
		case "/api/users/geoidentity/rules/missing":
			writeJSON(w, http.StatusNotFound, apiErrorBody(http.StatusNotFound, "RULE_NOT_FOUND", "Regla GeoIdentity no encontrada"))
		default:
			writeJSON(w, http.StatusNotFound, apiErrorBody(http.StatusNotFound, "USER_NOT_FOUND", "Usuario no encontrado en tu organización"))
		}
	})
	ctx := context.Background()

	if _, err := c.GeoIdentity.UnassignFromUser(ctx, "r1", "u1"); !errors.Is(err, ErrLastSubject) {
		t.Fatalf("expected ErrLastSubject, got %v", err)
	}
	if _, err := c.GeoIdentity.GetRule(ctx, "missing"); !errors.Is(err, ErrRuleNotFound) {
		t.Fatalf("expected ErrRuleNotFound, got %v", err)
	}
	if _, err := c.GeoIdentity.ListUserRules(ctx, "ghost"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
	if _, err := c.GeoIdentity.AssignToUser(ctx, "r1", ""); !errors.Is(err, ErrUserIDRequired) {
		t.Fatalf("expected ErrUserIDRequired, got %v", err)
	}
	if err := c.GeoIdentity.DeleteRule(ctx, ""); !errors.Is(err, ErrRuleIDRequired) {
		t.Fatalf("expected ErrRuleIDRequired, got %v", err)
	}
	// Distinct sentinels must not be confused with each other.
	if _, err := c.GeoIdentity.GetRule(ctx, "missing"); errors.Is(err, ErrGeofenceNotFound) || errors.Is(err, ErrUserNotFound) {
		t.Fatalf("rule not found must not match other not-found sentinels: %v", err)
	}
}

func TestPolygonFromLatLng(t *testing.T) {
	raw, err := PolygonFromLatLng([][]float64{{19.4, -99.2}, {19.4, -99.1}, {19.5, -99.1}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var poly struct {
		Type        string         `json:"type"`
		Coordinates [][][2]float64 `json:"coordinates"`
	}
	if err := json.Unmarshal(raw, &poly); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	ring := poly.Coordinates[0]
	if poly.Type != "Polygon" || len(ring) != 4 {
		t.Fatalf("expected closed Polygon with 4 points, got %s", raw)
	}
	if ring[0] != [2]float64{-99.2, 19.4} || ring[0] != ring[3] {
		t.Fatalf("expected [lng, lat] order and closed ring, got %v", ring)
	}

	if _, err := PolygonFromLatLng([][]float64{{1, 1}, {2, 2}}); !errors.Is(err, ErrInvalidCoordinates) {
		t.Fatalf("expected ErrInvalidCoordinates, got %v", err)
	}
	if _, err := PolygonFromLatLng([][]float64{{91, 0}, {0, 0}, {1, 1}}); !errors.Is(err, ErrInvalidLatitude) {
		t.Fatalf("expected ErrInvalidLatitude, got %v", err)
	}
	if _, err := PolygonFromLatLng([][]float64{{1}, {0, 0}, {1, 1}}); err == nil {
		t.Fatal("expected error for malformed coordinate")
	}
}
