/*
Package auth provides JWT authentication utilities for microservices.
*/
package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// JwtWrapper wraps the signing key and the issuer.
type JwtWrapper struct {
	SecretKey       string
	Issuer          string
	ExpirationHours int64
}

// JwtClaim adds email as a claim to the token.
type JwtClaim struct {
	ID    string `json:"ID"`
	Email string `json:"Email"`
	jwt.RegisteredClaims
}

// NewJwtWrapper creates a new JwtWrapper object.
func NewJwtWrapper(secretKey, issuer string, expirationHours int64) (*JwtWrapper, error) {
	if secretKey == "" {
		return nil, errors.New("secret key must be set")
	}

	if issuer == "" {
		return nil, errors.New("issuer must be set")
	}

	if expirationHours == 0 {
		return nil, errors.New("expiration hours must be greater than 0")
	}
	return &JwtWrapper{
		SecretKey:       secretKey,
		Issuer:          issuer,
		ExpirationHours: expirationHours,
	}, nil
}

// GenerateToken generates a jwt token.
func (j *JwtWrapper) GenerateToken(ctx context.Context, uuid, email string) (string, error) {
	claims := &JwtClaim{
		ID:    uuid,
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Local().Add(time.Hour * time.Duration(j.ExpirationHours))),
			Issuer:    j.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString([]byte(j.SecretKey))
	if err != nil {
		return "", err
	}

	return signedToken, nil
}

// ValidateToken validates the jwt token.
func (j *JwtWrapper) ValidateToken(ctx context.Context, signedToken string) (*JwtClaim, error) {
	token, err := jwt.ParseWithClaims(
		signedToken,
		&JwtClaim{},
		func(token *jwt.Token) (interface{}, error) {
			// Validate the signing method to prevent algorithm confusion attacks
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(j.SecretKey), nil
		},
	)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JwtClaim)
	if !ok {
		return nil, errors.New("couldn't parse claims")
	}

	// Check expiration using the new time handling
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("jwt is expired")
	}

	return claims, nil
}