//go:build integration
// +build integration

package test_scripts

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"

	"image-resizer/config"
	"image-resizer/handlers"

	_ "github.com/mattn/go-sqlite3"
)

// Note: These tests require libvips to be installed and are tagged with integration
// Run with: go test -tags=integration ./test_scripts/...

func setupAuthTestApp() *fiber.App {
	cfg := config.Load()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	createTableSQL := `CREATE TABLE IF NOT EXISTS users (
		username TEXT PRIMARY KEY,
		password_hash TEXT,
		passkey_hash TEXT,
		blocked BOOLEAN DEFAULT 0
	);`
	db.Exec(createTableSQL)

	app := fiber.New()
	authHandler := handlers.NewAuthHandler(cfg, db)
	app.Post("/auth/login", authHandler.Login)
	app.Post("/auth/refresh", authHandler.Refresh)
	return app
}

func TestLoginSuccess(t *testing.T) {
	app := setupAuthTestApp()

	body := `{"username":"admin","password":"admin"}`
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(respBody), "token")
	assert.Contains(t, string(respBody), "refresh_token")
}

func TestLoginMissingUsername(t *testing.T) {
	app := setupAuthTestApp()

	body := `{"password":"admin"}`
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestLoginMissingPassword(t *testing.T) {
	app := setupAuthTestApp()

	body := `{"username":"admin"}`
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestLoginInvalidBody(t *testing.T) {
	app := setupAuthTestApp()

	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestRefreshSuccess(t *testing.T) {
	app := setupAuthTestApp()

	loginBody := `{"username":"admin","password":"admin"}`
	loginReq := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, _ := app.Test(loginReq)
	loginRespBody, _ := io.ReadAll(loginResp.Body)

	var result map[string]interface{}
	fromJSON(loginRespBody, &result)
	refreshToken := result["refresh_token"].(string)

	refreshBody := `{"refresh_token":"` + refreshToken + `"}`
	refreshReq := httptest.NewRequest("POST", "/auth/refresh", bytes.NewBufferString(refreshBody))
	refreshReq.Header.Set("Content-Type", "application/json")
	refreshResp, err := app.Test(refreshReq)

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, refreshResp.StatusCode)
}

func TestRefreshMissingToken(t *testing.T) {
	app := setupAuthTestApp()

	body := `{}`
	req := httptest.NewRequest("POST", "/auth/refresh", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestRefreshInvalidToken(t *testing.T) {
	app := setupAuthTestApp()

	body := `{"refresh_token":"invalid.token.here"}`
	req := httptest.NewRequest("POST", "/auth/refresh", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func fromJSON(data []byte, v interface{}) {
	json.Unmarshal(data, v)
}
