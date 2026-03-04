package middleware

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestGenerateAndValidateToken(t *testing.T) {
	secret := "test-secret-key"
	user := User{
		ID:       "123",
		Username: "testuser",
		Email:    "test@example.com",
	}

	token, expiresAt, err := GenerateToken(secret, user, time.Hour)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Greater(t, expiresAt, time.Now().Unix())

	claims, err := ValidateToken(secret, token)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, claims.UserID)
	assert.Equal(t, user.Email, claims.Email)
}

func TestValidateTokenInvalidSecret(t *testing.T) {
	secret := "test-secret-key"
	user := User{
		ID:       "123",
		Username: "testuser",
		Email:    "test@example.com",
	}

	token, _, err := GenerateToken(secret, user, time.Hour)
	assert.NoError(t, err)

	_, err = ValidateToken("wrong-secret", token)
	assert.Error(t, err)
}

func TestValidateTokenInvalidToken(t *testing.T) {
	_, err := ValidateToken("secret", "invalid.token")
	assert.Error(t, err)
}

func TestGenerateRefreshToken(t *testing.T) {
	secret := "test-secret-key"
	user := User{
		ID:       "123",
		Username: "testuser",
		Email:    "test@example.com",
	}

	token, expiresAt, err := GenerateRefreshToken(secret, user, time.Hour*24*7)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Greater(t, expiresAt, time.Now().Unix())

	claims, err := ValidateToken(secret, token)
	assert.NoError(t, err)
	assert.Equal(t, "image-resizer-refresh", claims.Issuer)
}

func TestTokenExpiry(t *testing.T) {
	secret := "test-secret-key"
	user := User{
		ID:       "123",
		Username: "testuser",
		Email:    "test@example.com",
	}

	token, _, err := GenerateToken(secret, user, -time.Hour)
	assert.NoError(t, err)

	claims, err := ValidateToken(secret, token)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestJWTAuthMiddlewareMissingHeader(t *testing.T) {
	secret := "test-secret-key"

	_, err := ValidateToken(secret, "")
	assert.Error(t, err)
}

func TestClaimsHaveCorrectIssuer(t *testing.T) {
	secret := "test-secret-key"
	user := User{
		ID:       "123",
		Username: "testuser",
		Email:    "test@example.com",
	}

	token, _, err := GenerateToken(secret, user, time.Hour)
	assert.NoError(t, err)

	parsedToken, _ := jwt.ParseWithClaims(token, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})

	claims := parsedToken.Claims.(*JWTClaims)
	assert.Equal(t, "image-resizer", claims.Issuer)
}
