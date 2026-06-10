// Package contracts holds the auth service inputs and results.
package contracts

import (
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	iamentity "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
)

// LoginCommand authenticates a user by email + password. UserAgent/IP are
// recorded with the issued refresh token.
type LoginCommand struct {
	Email     string
	Password  string
	UserAgent string
	IP        string
}

// RefreshCommand exchanges a refresh token for a new token pair (rotation).
type RefreshCommand struct {
	RefreshToken string
	UserAgent    string
	IP           string
}

// LogoutCommand revokes a refresh token.
type LogoutCommand struct {
	RefreshToken string
}

// TokenPair is the authenticated result returned by login/refresh.
type TokenPair struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	User             *iamentity.User
	Permissions      []authz.Permission
	SectorScope      authz.SectorScope
}
