package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
)

// JWTManager implements auth.TokenManager. Access tokens are signed JWTs
// (HS256); refresh tokens are opaque 256-bit random strings whose SHA-256 hash
// is what the database stores.
type JWTManager struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewJWTManager builds the manager.
func NewJWTManager(secret, issuer string, accessTTL, refreshTTL time.Duration) *JWTManager {
	return &JWTManager{
		secret:     []byte(secret),
		issuer:     issuer,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// accessClaims is the JWT payload for an access token.
type accessClaims struct {
	TenantID    string   `json:"tid"`
	Permissions []string `json:"perms"`
	SectorIDs   []string `json:"sectors"`
	SectorScope string   `json:"scope"`
	jwt.RegisteredClaims
}

// IssueAccess signs an access token for the given identity.
func (m *JWTManager) IssueAccess(claims auth.AccessClaims) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(m.accessTTL)

	jc := accessClaims{
		TenantID:    claims.TenantID,
		Permissions: permsToStrings(claims.Permissions),
		SectorIDs:   claims.SectorIDs,
		SectorScope: string(claims.SectorScope),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   claims.UserID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        newJTI(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jc)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

// VerifyAccess validates the token and returns its claims.
func (m *JWTManager) VerifyAccess(token string) (auth.AccessClaims, error) {
	var jc accessClaims
	parsed, err := jwt.ParseWithClaims(token, &jc, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(m.issuer))
	if err != nil || !parsed.Valid {
		return auth.AccessClaims{}, fmt.Errorf("invalid access token: %w", err)
	}

	exp := time.Time{}
	if jc.ExpiresAt != nil {
		exp = jc.ExpiresAt.Time
	}
	return auth.AccessClaims{
		TenantID:    jc.TenantID,
		UserID:      jc.Subject,
		Permissions: stringsToPerms(jc.Permissions),
		SectorIDs:   jc.SectorIDs,
		SectorScope: authz.SectorScope(jc.SectorScope),
		ExpiresAt:   exp,
	}, nil
}

// GenerateRefresh mints a new opaque refresh token and its expiry.
func (m *JWTManager) GenerateRefresh() (string, time.Time, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", time.Time{}, err
	}
	plaintext := base64.RawURLEncoding.EncodeToString(buf)
	return plaintext, time.Now().UTC().Add(m.refreshTTL), nil
}

// HashRefresh derives the SHA-256 storage hash of a refresh token.
func (m *JWTManager) HashRefresh(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

func newJTI() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func permsToStrings(perms []authz.Permission) []string {
	out := make([]string, len(perms))
	for i, p := range perms {
		out[i] = string(p)
	}
	return out
}

func stringsToPerms(ss []string) []authz.Permission {
	out := make([]authz.Permission, 0, len(ss))
	for _, s := range ss {
		out = append(out, authz.Permission(s))
	}
	return out
}

var _ auth.TokenManager = (*JWTManager)(nil)
