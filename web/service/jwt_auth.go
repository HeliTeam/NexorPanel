package service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nexor/panel/database"
	"github.com/nexor/panel/database/model"
)

const (
	jwtAccessMinutes  = 15
	jwtRefreshDays    = 7
	jwtIssuer         = "nexor-panel"
	jwtTypeAccess     = "access"
	jwtTypeRefresh    = "refresh"
	minJWTSecretBytes = 32
)

// JWTAuthService issues and validates Nexor REST tokens (HS256).
type JWTAuthService struct {
	set SettingService
}

type adminClaims struct {
	AdminID   int    `json:"aid"`
	Nickname  string `json:"nick"`
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

// EnsureNexorJWTSecret returns a stored secret, generating one on first use.
func (s *SettingService) EnsureNexorJWTSecret() (string, error) {
	sec, err := s.GetNexorJWTSecret()
	if err != nil {
		return "", err
	}
	if len(sec) >= minJWTSecretBytes {
		return sec, nil
	}
	buf := make([]byte, 48)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	sec = base64.RawURLEncoding.EncodeToString(buf)
	if err := s.SetNexorJWTSecret(sec); err != nil {
		return "", err
	}
	return sec, nil
}

// IssuePair returns short-lived access and long-lived refresh JWTs for an admin.
func (j *JWTAuthService) IssuePair(admin *model.Admin) (access string, refresh string, err error) {
	if admin == nil {
		return "", "", errors.New("nil admin")
	}
	secret, err := j.set.EnsureNexorJWTSecret()
	if err != nil {
		return "", "", err
	}
	now := time.Now()
	accessClaims := adminClaims{
		AdminID:   admin.Id,
		Nickname:  admin.Nickname,
		TokenType: jwtTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   admin.Nickname,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(jwtAccessMinutes * time.Minute)),
		},
	}
	refreshClaims := adminClaims{
		AdminID:   admin.Id,
		Nickname:  admin.Nickname,
		TokenType: jwtTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   admin.Nickname,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(jwtRefreshDays * 24 * time.Hour)),
		},
	}
	at := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	rt := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	access, err = at.SignedString([]byte(secret))
	if err != nil {
		return "", "", err
	}
	refresh, err = rt.SignedString([]byte(secret))
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

// ParseAccessToken validates an access JWT and returns claims.
func (j *JWTAuthService) ParseAccessToken(token string) (*adminClaims, error) {
	secret, err := j.set.GetNexorJWTSecret()
	if err != nil || len(secret) < minJWTSecretBytes {
		return nil, errors.New("jwt not configured")
	}
	var claims adminClaims
	t, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil || !t.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.TokenType != jwtTypeAccess {
		return nil, errors.New("not an access token")
	}
	return &claims, nil
}

// ParseRefreshToken validates a refresh JWT.
func (j *JWTAuthService) ParseRefreshToken(token string) (*adminClaims, error) {
	secret, err := j.set.GetNexorJWTSecret()
	if err != nil || len(secret) < minJWTSecretBytes {
		return nil, errors.New("jwt not configured")
	}
	var claims adminClaims
	t, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil || !t.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.TokenType != jwtTypeRefresh {
		return nil, errors.New("not a refresh token")
	}
	return &claims, nil
}

// RefreshPair loads the admin and mints a new access+refresh pair.
func (j *JWTAuthService) RefreshPair(refreshToken string) (access string, refresh string, err error) {
	cl, err := j.ParseRefreshToken(refreshToken)
	if err != nil {
		return "", "", err
	}
	db := database.GetDB()
	var admin model.Admin
	if err := db.First(&admin, cl.AdminID).Error; err != nil {
		return "", "", err
	}
	return j.IssuePair(&admin)
}
