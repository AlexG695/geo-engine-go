package geoengine

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Organization roles accepted by the Users service.
// The short form ("admin", "viewer", ...) is also accepted by the API.
const (
	RoleAdmin      = "roles/geoengine.admin"
	RoleDispatcher = "roles/geoengine.dispatcher"
	RoleBilling    = "roles/geoengine.billing"
	RoleViewer     = "roles/geoengine.viewer"
	RoleDevice     = "roles/geoengine.device"
)

// User status values returned by the API.
const (
	UserStatusActive  = "active"
	UserStatusPending = "pending"
	UserStatusRevoked = "revoked"
)

// User is a member of the organization that owns the API key.
type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	Name        string    `json:"name"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
	Permissions []string  `json:"permissions,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	LastLoginAt time.Time `json:"last_login_at,omitempty"`
}

// Role describes an organization role and the permissions it grants.
type Role struct {
	ID          string   `json:"id"`
	Permissions []string `json:"permissions"`
}

// InviteUserRequest is the payload for UsersService.Invite.
type InviteUserRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
	// InviterName is shown in the invitation email. Optional.
	InviterName string `json:"inviter_name,omitempty"`
	// Lang selects the email language: "es" (default), "en" or "pt". Optional.
	Lang string `json:"lang,omitempty"`
}

// Validate checks the required fields before sending the request.
func (r InviteUserRequest) Validate() error {
	if strings.TrimSpace(r.Email) == "" {
		return ErrEmailRequired
	}
	if strings.TrimSpace(r.Role) == "" {
		return ErrRoleRequired
	}
	return nil
}

// Invitation is the result of UsersService.Invite. The invitation email is sent by GeoEngine;
// InviteURL is returned as well so it can be delivered through other channels.
type Invitation struct {
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	InviteURL string    `json:"invite_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// UsersService manages organization members, roles and invitations through the api-service.
// Safe for concurrent use across multiple goroutines.
type UsersService struct {
	client *Client
}

func newUsersService(client *Client) *UsersService {
	return &UsersService{client: client}
}

// List returns the members of the organization.
func (s *UsersService) List(ctx context.Context) ([]User, error) {
	var users []User
	if err := s.client.doAPIServiceRequest(ctx, http.MethodGet, "/api/users", nil, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// Get returns a single member of the organization.
func (s *UsersService) Get(ctx context.Context, userID string) (*User, error) {
	path, err := userPath(userID, "")
	if err != nil {
		return nil, err
	}
	var user User
	if err := s.client.doAPIServiceRequest(ctx, http.MethodGet, path, nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// ListRoles returns the roles available to the organization and their permissions.
func (s *UsersService) ListRoles(ctx context.Context) ([]Role, error) {
	var roles []Role
	if err := s.client.doAPIServiceRequest(ctx, http.MethodGet, "/api/users/roles", nil, &roles); err != nil {
		return nil, err
	}
	return roles, nil
}

// Invite creates an invitation and emails it to the recipient.
// It returns ErrInvitationPending if a pending invitation already exists for the email,
// and ErrUserAlreadyExists if the email already belongs to the organization.
func (s *UsersService) Invite(ctx context.Context, req InviteUserRequest) (*Invitation, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var inv Invitation
	if err := s.client.doAPIServiceRequest(ctx, http.MethodPost, "/api/users/invite", req, &inv); err != nil {
		return nil, err
	}
	return &inv, nil
}

// UpdateRole changes the role of a member. The new permissions apply on the user's next sign-in.
// It returns ErrLastAdmin if the change would leave the organization without an admin.
func (s *UsersService) UpdateRole(ctx context.Context, userID, role string) (*User, error) {
	if strings.TrimSpace(role) == "" {
		return nil, ErrRoleRequired
	}
	path, err := userPath(userID, "/role")
	if err != nil {
		return nil, err
	}
	var user User
	payload := struct {
		Role string `json:"role"`
	}{Role: role}
	if err := s.client.doAPIServiceRequest(ctx, http.MethodPatch, path, payload, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// Remove revokes a member's access to the organization.
// It returns ErrLastAdmin if the member is the last admin.
func (s *UsersService) Remove(ctx context.Context, userID string) error {
	path, err := userPath(userID, "")
	if err != nil {
		return err
	}
	return s.client.doAPIServiceRequest(ctx, http.MethodDelete, path, nil, nil)
}

func userPath(userID, suffix string) (string, error) {
	id := strings.TrimSpace(userID)
	if id == "" {
		return "", ErrUserIDRequired
	}
	return "/api/users/" + url.PathEscape(id) + suffix, nil
}
