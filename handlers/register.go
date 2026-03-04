package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"

	"image-resizer/config"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/argon2"
)

type RegistrationHandler struct {
	cfg *config.Config
	db  *sql.DB
}

type User struct {
	Username string `json:"username"`
}

func NewRegistrationHandler(cfg *config.Config, db *sql.DB) *RegistrationHandler {
	return &RegistrationHandler{cfg: cfg, db: db}
}

func (h *RegistrationHandler) Register(c *fiber.Ctx) error {
	// Registration secret is required
	regSecret := GetRegistrationSecret()
	if regSecret == "" {
		log.Println("Registration attempt failed: REGISTRATION_SECRET is not configured on the server.")
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Registration is currently disabled by the server administrator."})
	}
	if c.Get("X-Registration-Secret") != regSecret {
		log.Printf("Registration failed: Secret mismatch. Expected length: %d, Received length: %d", len(regSecret), len(c.Get("X-Registration-Secret")))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Invalid or missing registration secret"})
	}

	var req User
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Username == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Username required"})
	}

	hashString := func(s string) (string, error) {
		salt := make([]byte, 16)
		if _, err := rand.Read(salt); err != nil {
			return "", err
		}
		time, memory, threads, keyLen := uint32(1), uint32(64*1024), uint8(4), uint32(32)
		hash := argon2.IDKey([]byte(s), salt, time, memory, threads, keyLen)
		b64Salt := base64.RawStdEncoding.EncodeToString(salt)
		b64Hash := base64.RawStdEncoding.EncodeToString(hash)
		return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, memory, time, threads, b64Salt, b64Hash), nil
	}

	// Generate passkey
	passkeyBytes := make([]byte, 24) // 24 bytes -> 32 chars base64
	rand.Read(passkeyBytes)
	passkey := base64.RawURLEncoding.EncodeToString(passkeyBytes)
	passkeyHash, _ := hashString(passkey)

	_, err := h.db.Exec("INSERT INTO users (username, password_hash, passkey_hash, blocked) VALUES (?, ?, ?, ?)", req.Username, nil, passkeyHash, false)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not register user"})
	}

	go h.db.Exec("INSERT INTO activity_logs (user_id, action, details, ip_address) VALUES (?, ?, ?, ?)", req.Username, "register", "success", c.IP())

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "User registered successfully",
		"user":    req.Username,
		"passkey": passkey,
	})
}
