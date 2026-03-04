package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"

	"image-resizer/config"
	"image-resizer/middleware"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/argon2"
)

var (
	regSecretMu sync.RWMutex
	regSecret   string
)

func InitRegistrationSecret(s string) {
	regSecretMu.Lock()
	defer regSecretMu.Unlock()
	regSecret = s
}

func GetRegistrationSecret() string {
	regSecretMu.RLock()
	defer regSecretMu.RUnlock()
	return regSecret
}

type AuthHandler struct {
	cfg *config.Config
	db  *sql.DB
}

func NewAuthHandler(cfg *config.Config, db *sql.DB) *AuthHandler {
	return &AuthHandler{cfg: cfg, db: db}
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req middleware.LoginRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.Username == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "username and password are required",
		})
	}

	user, err := h.validateCredentials(req.Username, req.Password)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid credentials",
		})
	}

	token, expiresAt, err := middleware.GenerateToken(h.cfg.JWT.Secret, user, h.cfg.JWT.Expiry)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate token",
		})
	}

	refreshToken, _, err := middleware.GenerateRefreshToken(h.cfg.JWT.Secret, user, h.cfg.JWT.RefreshExpiry)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate refresh token",
		})
	}

	go h.db.Exec("INSERT INTO activity_logs (user_id, action, details, ip_address) VALUES (?, ?, ?, ?)", user.ID, "login", "success", c.IP())

	return c.JSON(fiber.Map{
		"token":         token,
		"refresh_token": refreshToken,
		"expires_at":    expiresAt,
		"user": fiber.Map{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.RefreshToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "refresh token is required",
		})
	}

	claims, err := middleware.ValidateToken(h.cfg.JWT.Secret, req.RefreshToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid refresh token",
		})
	}

	// Check if user is blocked
	if h.isBlocked(claims.UserID) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "user is blocked",
		})
	}

	user := middleware.User{
		ID:       claims.UserID,
		Username: claims.UserID,
		Email:    claims.Email,
	}

	token, expiresAt, err := middleware.GenerateToken(h.cfg.JWT.Secret, user, h.cfg.JWT.Expiry)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate token",
		})
	}

	return c.JSON(middleware.TokenResponse{
		Token:     token,
		ExpiresAt: expiresAt,
	})
}

func (h *AuthHandler) BlockUser(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	if userID != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "admin only"})
	}

	var req struct {
		Username string `json:"username"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	result, err := h.db.Exec("UPDATE users SET blocked = 1 WHERE username = ?", req.Username)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "database error"})
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	// Generate new registration secret
	newSecretBytes := make([]byte, 16)
	if _, err := rand.Read(newSecretBytes); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate new secret"})
	}
	newSecret := hex.EncodeToString(newSecretBytes)

	// Update the secret thread-safely
	InitRegistrationSecret(newSecret)

	go h.db.Exec("INSERT INTO activity_logs (user_id, action, details, ip_address) VALUES (?, ?, ?, ?)", userID, "block_user", "blocked "+req.Username, c.IP())

	return c.JSON(fiber.Map{
		"message":                 "user blocked",
		"new_registration_secret": newSecret,
	})
}

func (h *AuthHandler) validateCredentials(username, password string) (middleware.User, error) {
	// Check admin
	adminUser := os.Getenv("ADMIN_USER")
	if adminUser == "" {
		adminUser = "admin"
	}
	adminPass := os.Getenv("ADMIN_PASSWORD")
	if adminPass == "" {
		adminPass = "admin"
	}

	if username == adminUser && password == adminPass {
		return middleware.User{
			ID:       "admin",
			Username: username,
			Email:    "admin@example.com",
		}, nil
	}

	// Check registered users
	var passwordHash sql.NullString
	var passkeyHash sql.NullString
	var blocked bool

	err := h.db.QueryRow("SELECT password_hash, passkey_hash, blocked FROM users WHERE username = ?", username).Scan(&passwordHash, &passkeyHash, &blocked)
	if err == sql.ErrNoRows {
		return middleware.User{}, fmt.Errorf("invalid credentials")
	}
	if err != nil {
		return middleware.User{}, err
	}

	if blocked {
		return middleware.User{}, fmt.Errorf("user is blocked")
	}

	verify := func(hash string) bool {
		if hash == "" {
			return false
		}
		parts := strings.Split(hash, "$")
		if len(parts) != 6 {
			return false
		}

		var memory uint32
		var time uint32
		var threads uint8
		fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads)

		salt, _ := base64.RawStdEncoding.DecodeString(parts[4])
		decodedHash, _ := base64.RawStdEncoding.DecodeString(parts[5])

		keyLen := uint32(len(decodedHash))
		newHash := argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen)

		return subtle.ConstantTimeCompare(decodedHash, newHash) == 1
	}

	if (passwordHash.Valid && verify(passwordHash.String)) || (passkeyHash.Valid && verify(passkeyHash.String)) {
		return middleware.User{
			ID:       username,
			Username: username,
			Email:    username + "@example.com",
		}, nil
	}

	return middleware.User{}, fmt.Errorf("invalid credentials")
}

func (h *AuthHandler) isBlocked(username string) bool {
	if username == "admin" {
		return false
	}

	var blocked bool
	err := h.db.QueryRow("SELECT blocked FROM users WHERE username = ?", username).Scan(&blocked)
	if err != nil {
		return false
	}
	return blocked
}
