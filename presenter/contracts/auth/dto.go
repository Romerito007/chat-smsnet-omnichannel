// Package auth holds the request/response DTOs for the authentication endpoints.
package auth

import (
	"time"

	authcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/auth/contracts"
	iamdto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/iam"
)

// LoginRequest is the body of POST /v1/auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RefreshRequest is the body of POST /v1/auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// LogoutRequest is the body of POST /v1/auth/logout.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// TokenResponse is returned by login and refresh.
type TokenResponse struct {
	AccessToken      string              `json:"access_token"`
	TokenType        string              `json:"token_type"`
	AccessExpiresAt  time.Time           `json:"access_expires_at"`
	RefreshToken     string              `json:"refresh_token"`
	RefreshExpiresAt time.Time           `json:"refresh_expires_at"`
	User             iamdto.UserResponse `json:"user"`
	Permissions      []string            `json:"permissions"`
}

// NewTokenResponse maps a service token pair to its DTO.
func NewTokenResponse(p *authcontracts.TokenPair) TokenResponse {
	return TokenResponse{
		AccessToken:      p.AccessToken,
		TokenType:        "Bearer",
		AccessExpiresAt:  p.AccessExpiresAt,
		RefreshToken:     p.RefreshToken,
		RefreshExpiresAt: p.RefreshExpiresAt,
		User:             iamdto.NewUserResponse(p.User),
		Permissions:      permStrings(p.Permissions),
	}
}

// MeResponse is returned by GET /v1/me.
type MeResponse struct {
	User        iamdto.UserResponse `json:"user"`
	Permissions []string            `json:"permissions"`
	SectorScope string              `json:"sector_scope"`
	SectorIDs   []string            `json:"sector_ids"`
}

func permStrings[T ~string](perms []T) []string {
	out := make([]string, len(perms))
	for i, p := range perms {
		out[i] = string(p)
	}
	return out
}
