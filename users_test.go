package geoengine

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c := New("test-users-key", WithAPIServiceURL(server.URL))
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func apiErrorBody(status int, reason, message string) string {
	b, _ := json.Marshal(map[string]any{
		"success": false,
		"error": map[string]any{
			"code":    status,
			"message": message,
			"status":  http.StatusText(status),
			"details": []map[string]any{{"@type": "type.geoengineapis.com/geoengine.rpc.ErrorInfo", "reason": reason, "domain": "api-service"}},
		},
	})
	return string(b)
}

func TestUsersList(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/users" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "test-users-key" {
			t.Errorf("missing X-API-Key header")
		}
		if r.Header.Get("User-Agent") != userAgent {
			t.Errorf("unexpected User-Agent %q", r.Header.Get("User-Agent"))
		}
		writeJSON(w, http.StatusOK, `{"success":true,"data":[{"id":"u1","email":"ana@acme.com","name":"Ana","role":"roles/geoengine.admin","status":"active"}]}`)
	})

	users, err := c.Users.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 1 || users[0].ID != "u1" || users[0].Role != RoleAdmin {
		t.Fatalf("unexpected users: %+v", users)
	}
}

func TestUsersGetEscapesID(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/api/users/a%2Fb" {
			t.Errorf("expected escaped path, got %s", r.URL.EscapedPath())
		}
		writeJSON(w, http.StatusOK, `{"success":true,"data":{"id":"a/b"}}`)
	})

	if _, err := c.Users.Get(context.Background(), "a/b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUsersInvite(t *testing.T) {
	tests := []struct {
		name    string
		req     InviteUserRequest
		status  int
		body    string
		wantErr error
	}{
		{
			name:   "success",
			req:    InviteUserRequest{Email: "new@acme.com", Role: RoleViewer, Lang: "en"},
			status: http.StatusCreated,
			body:   `{"success":true,"data":{"email":"new@acme.com","role":"roles/geoengine.viewer","status":"pending","invite_url":"https://dashboard.geoengine.dev/accept-invite?token=t"}}`,
		},
		{
			name:    "pending invitation conflict",
			req:     InviteUserRequest{Email: "dup@acme.com", Role: RoleViewer},
			status:  http.StatusConflict,
			body:    apiErrorBody(http.StatusConflict, "INVITATION_PENDING", "Ya existe una invitación pendiente para este email"),
			wantErr: ErrInvitationPending,
		},
		{
			name:    "already a member",
			req:     InviteUserRequest{Email: "ana@acme.com", Role: RoleViewer},
			status:  http.StatusConflict,
			body:    apiErrorBody(http.StatusConflict, "USER_ALREADY_EXISTS", "El usuario ya pertenece a la organización"),
			wantErr: ErrUserAlreadyExists,
		},
		{
			name:    "missing email is rejected locally",
			req:     InviteUserRequest{Role: RoleViewer},
			wantErr: ErrEmailRequired,
		},
		{
			name:    "missing role is rejected locally",
			req:     InviteUserRequest{Email: "x@acme.com"},
			wantErr: ErrRoleRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				called = true
				if r.Method != http.MethodPost || r.URL.Path != "/api/users/invite" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				var got InviteUserRequest
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Errorf("decode: %v", err)
				}
				if got.Email != tt.req.Email || got.Role != tt.req.Role || got.Lang != tt.req.Lang {
					t.Errorf("unexpected payload %+v", got)
				}
				writeJSON(w, tt.status, tt.body)
			})

			inv, err := c.Users.Invite(context.Background(), tt.req)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
				if tt.status == 0 && called {
					t.Fatal("request should not reach the server")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if inv.InviteURL == "" || inv.Status != "pending" {
				t.Fatalf("unexpected invitation: %+v", inv)
			}
		})
	}
}

func TestUsersUpdateRoleLastAdmin(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/users/u1/role" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		writeJSON(w, http.StatusConflict, apiErrorBody(http.StatusConflict, "LAST_ADMIN", "La organización debe conservar al menos un administrador"))
	})

	_, err := c.Users.UpdateRole(context.Background(), "u1", RoleViewer)
	if !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("expected ErrLastAdmin, got %v", err)
	}
	var apiErr *APIServiceError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusConflict || apiErr.Code != "LAST_ADMIN" {
		t.Fatalf("expected *APIServiceError with LAST_ADMIN, got %#v", err)
	}
}

func TestUsersRemove(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/users/u2" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "" {
			t.Errorf("DELETE without body must not send Content-Type")
		}
		writeJSON(w, http.StatusOK, `{"success":true,"data":{"status":"revoked"}}`)
	})

	if err := c.Users.Remove(context.Background(), "u2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUsersNotFoundAndValidation(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, apiErrorBody(http.StatusNotFound, "USER_NOT_FOUND", "Usuario no encontrado en tu organización"))
	})

	if _, err := c.Users.Get(context.Background(), "ghost"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
	if _, err := c.Users.Get(context.Background(), "  "); !errors.Is(err, ErrUserIDRequired) {
		t.Fatalf("expected ErrUserIDRequired, got %v", err)
	}
	if _, err := c.Users.UpdateRole(context.Background(), "u1", ""); !errors.Is(err, ErrRoleRequired) {
		t.Fatalf("expected ErrRoleRequired, got %v", err)
	}
}

func TestAPIServiceErrorAuthAndClosedClient(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusUnauthorized, apiErrorBody(http.StatusUnauthorized, "MISSING_API_KEY", "Se requiere una clave de API válida"))
	})

	if _, err := c.Users.List(context.Background()); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}

	// KeyAuth can also answer with a plain {"error": "..."} body.
	plain := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusForbidden, `{"error":"Invalid API Key"}`)
	})
	if _, err := plain.Users.List(context.Background()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}

	_ = c.Close()
	if _, err := c.Users.List(context.Background()); !errors.Is(err, ErrClientClosed) {
		t.Fatalf("expected ErrClientClosed, got %v", err)
	}
}
