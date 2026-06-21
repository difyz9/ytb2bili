package middleware

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	localAccessTokenType  = "access"
	localRefreshTokenType = "refresh"
)

type LocalAuthClaims struct {
	UID   string `json:"uid"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	Type  string `json:"type"`
	jwt.RegisteredClaims
}

func ParseBearerToken(header string) string {
	parts := strings.Fields(strings.TrimSpace(header))
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func ParseLocalToken(tokenString, secret, expectedType string) (*LocalAuthClaims, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("empty jwt secret")
	}

	token, err := jwt.ParseWithClaims(tokenString, &LocalAuthClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*LocalAuthClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}
	if claims.Type != expectedType {
		return nil, errors.New("token type mismatch")
	}
	if strings.TrimSpace(claims.UID) == "" {
		return nil, errors.New("missing uid claim")
	}
	return claims, nil
}

func IssueLocalTokenPair(secret string, now time.Time, uid, email, name, role string, accessTTL, refreshTTL time.Duration) (accessToken, refreshToken string, err error) {
	if strings.TrimSpace(secret) == "" {
		return "", "", errors.New("empty jwt secret")
	}
	if accessTTL <= 0 {
		accessTTL = 2 * time.Hour
	}
	if refreshTTL <= 0 {
		refreshTTL = 7 * 24 * time.Hour
	}

	base := LocalAuthClaims{
		UID:   uid,
		Email: email,
		Name:  name,
		Role:  role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uid,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	accessClaims := base
	accessClaims.Type = localAccessTokenType
	accessClaims.ExpiresAt = jwt.NewNumericDate(now.Add(accessTTL))

	refreshClaims := base
	refreshClaims.Type = localRefreshTokenType
	refreshClaims.ExpiresAt = jwt.NewNumericDate(now.Add(refreshTTL))

	accessToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(secret))
	if err != nil {
		return "", "", err
	}

	refreshToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(secret))
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}
