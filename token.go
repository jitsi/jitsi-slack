package jitsi

import (
	"crypto/x509"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/vincent-petithory/dataurl"
)

// TokenGenerator generates conference tokens for auth'ed users.
type TokenGenerator struct {
	Lifetime   time.Duration
	PrivateKey string
	Issuer     string
	Audience   string
	Kid        string
}

// JWTInput is the input required to generate a meeting JWT for a user.
type JWTInput struct {
	TenantID   string
	TenantName string
	RoomClaim  string
	UserID     string
	UserName   string
	AvatarURL  string
}

// CreateJWT generates conference tokens for auth'ed users.
func (g TokenGenerator) CreateJWT(in JWTInput) (string, error) {
	now := time.Now()
	exp := now.Add(g.Lifetime)
	claims := jwt.MapClaims{
		"iss":  g.Issuer,
		"nbf":  now.Unix(),
		"exp":  exp.Unix(),
		"sub":  in.TenantName,
		"aud":  g.Audience,
		"room": in.RoomClaim,
		"context": contextClaim{
			User: userClaim{
				DisplayName: in.UserName,
				ID:          in.UserID,
				AvatarURL:   in.AvatarURL,
			},
			Group: in.TenantName,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = g.Kid

	data, err := dataurl.DecodeString(g.PrivateKey)
	if err != nil {
		return "", err
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(data.Data)
	if err != nil {
		return "", err
	}

	return token.SignedString(privateKey)
}

type userClaim struct {
	ID          string `json:"id"`
	DisplayName string `json:"name"`
	AvatarURL   string `json:"avatar"`
}

type contextClaim struct {
	User  userClaim `json:"user"`
	Group string    `json:"group"`
}
