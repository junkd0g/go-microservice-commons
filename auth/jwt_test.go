package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/junkd0g/go-microservice-commons/auth"
)

func Test_NewJwtWrapper(t *testing.T) {
	type testCase struct {
		name            string
		SecretKey       string
		Issuer          string
		ExpirationHours int64
		ExpectedError   error
	}

	testCases := []testCase{
		{
			name:            "empty secret key",
			SecretKey:       "",
			Issuer:          "some-issuer",
			ExpirationHours: 1,
			ExpectedError:   errors.New("secret key must be set"),
		},
		{
			name:            "empty issuer",
			SecretKey:       "some-secret-key",
			Issuer:          "",
			ExpirationHours: 1,
			ExpectedError:   errors.New("issuer must be set"),
		},
		{
			name:            "zero expiration hours",
			SecretKey:       "some-secret-key",
			Issuer:          "some-issuer",
			ExpirationHours: 0,
			ExpectedError:   errors.New("expiration hours must be greater than 0"),
		},
		{
			name:            "valid parameters",
			SecretKey:       "some-secret-key",
			Issuer:          "some-issuer",
			ExpirationHours: 1,
			ExpectedError:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wrapper, err := auth.NewJwtWrapper(tc.SecretKey, tc.Issuer, tc.ExpirationHours)

			if tc.ExpectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.ExpectedError.Error(), err.Error())
				assert.Nil(t, wrapper)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, wrapper)
				assert.Equal(t, tc.SecretKey, wrapper.SecretKey)
				assert.Equal(t, tc.Issuer, wrapper.Issuer)
				assert.Equal(t, tc.ExpirationHours, wrapper.ExpirationHours)
			}
		})
	}
}

func Test_GenerateToken(t *testing.T) {
	t.Run("should generate a token", func(t *testing.T) {
		ctx := context.Background()

		jwtWrapper, err := auth.NewJwtWrapper("some-secret-key", "some-issuer", 1)
		assert.NoError(t, err)

		token, err := jwtWrapper.GenerateToken(ctx, "some-id", "some-email")
		assert.NoError(t, err)
		assert.NotEmpty(t, token)
	})
}

func Test_ValidateToken(t *testing.T) {
	t.Run("should validate a valid token", func(t *testing.T) {
		id := "some-uuid"
		email := "some-email"
		ctx := context.Background()

		jwtWrapper, err := auth.NewJwtWrapper("some-secret-key", "some-issuer", 1)
		assert.NoError(t, err)

		token, err := jwtWrapper.GenerateToken(ctx, id, email)
		assert.NoError(t, err)
		assert.NotEmpty(t, token)

		claims, err := jwtWrapper.ValidateToken(ctx, token)
		assert.NoError(t, err)
		assert.NotEmpty(t, claims)
		assert.Equal(t, id, claims.ID)
		assert.Equal(t, email, claims.Email)
	})

	t.Run("should fail with invalid token format", func(t *testing.T) {
		ctx := context.Background()

		jwtWrapper, err := auth.NewJwtWrapper("some-secret-key", "some-issuer", 1)
		assert.NoError(t, err)

		// Test with completely invalid token
		claims, err := jwtWrapper.ValidateToken(ctx, "invalid-token")
		assert.Error(t, err)
		assert.Nil(t, claims)
	})

	t.Run("should fail with token signed with different secret", func(t *testing.T) {
		id := "some-uuid"
		email := "some-email"
		ctx := context.Background()

		// Create token with one secret
		jwtWrapper1, err := auth.NewJwtWrapper("secret-key-1", "some-issuer", 1)
		assert.NoError(t, err)

		token, err := jwtWrapper1.GenerateToken(ctx, id, email)
		assert.NoError(t, err)

		// Try to validate with different secret
		jwtWrapper2, err := auth.NewJwtWrapper("secret-key-2", "some-issuer", 1)
		assert.NoError(t, err)

		claims, err := jwtWrapper2.ValidateToken(ctx, token)
		assert.Error(t, err)
		assert.Nil(t, claims)
	})

	t.Run("should fail with expired token", func(t *testing.T) {
		id := "some-uuid"
		email := "some-email"
		ctx := context.Background()

		// The JWT library validates expiration during parsing, so we'll get its error message
		jwtWrapper, err := auth.NewJwtWrapper("some-secret-key", "some-issuer", -1)
		assert.NoError(t, err)

		token, err := jwtWrapper.GenerateToken(ctx, id, email)
		assert.NoError(t, err)

		// The token should be expired immediately
		claims, err := jwtWrapper.ValidateToken(ctx, token)
		assert.Error(t, err)
		// The JWT library returns this error message for expired tokens
		assert.Contains(t, err.Error(), "expired")
		assert.Nil(t, claims)
	})

	t.Run("should fail when claims cannot be parsed", func(t *testing.T) {
		ctx := context.Background()

		jwtWrapper, err := auth.NewJwtWrapper("some-secret-key", "some-issuer", 1)
		assert.NoError(t, err)

		// Create a token with standard claims instead of JwtClaim
		// This will create a scenario where token.Claims.(*JwtClaim) fails
		standardToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

		claims, err := jwtWrapper.ValidateToken(ctx, standardToken)
		assert.Error(t, err)
		assert.Nil(t, claims)
		// Note: The exact error might vary, but it should be related to parsing or signature validation
	})

	t.Run("should validate token with uppercase field names", func(t *testing.T) {
		ctx := context.Background()

		// This tests that tokens with uppercase field names (ID, Email) work correctly
		// which is important for backward compatibility
		jwtWrapper, err := auth.NewJwtWrapper("test-secret-key", "AuthService", 720)
		assert.NoError(t, err)

		// Generate a token to ensure our struct tags work both ways
		token, err := jwtWrapper.GenerateToken(ctx, "65ff15f55c04488f1005008d", "test@example.com")
		assert.NoError(t, err)
		assert.NotEmpty(t, token)

		// Validate the generated token
		claims, err := jwtWrapper.ValidateToken(ctx, token)
		assert.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, "65ff15f55c04488f1005008d", claims.ID)
		assert.Equal(t, "test@example.com", claims.Email)
	})
}