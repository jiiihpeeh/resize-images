//go:build integration
// +build integration

package test_scripts

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"

	"image-resizer/config"
	"image-resizer/handlers"
	"image-resizer/middleware"

	_ "github.com/mattn/go-sqlite3"
)

// TestResizeCafeImage performs an integration test using the local cafe.jpg
func TestResizeCafeImage(t *testing.T) {
	// Locate cafe.jpg in the project root
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// We are running in test_scripts/ package, so root is one level up
	projectRoot := filepath.Dir(wd)
	imagePath := filepath.Join(projectRoot, "cafe.jpg")

	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		t.Skipf("cafe.jpg not found at %s, skipping test", imagePath)
	}

	// Start a local server to serve the image (simulating a remote URL)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, imagePath)
	}))
	defer ts.Close()

	// Setup Config & App
	cfg := config.Load()
	app := fiber.New()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	createTableSQL := `CREATE TABLE IF NOT EXISTS users (
		username TEXT PRIMARY KEY,
		password_hash TEXT,
		passkey_hash TEXT,
		blocked BOOLEAN DEFAULT 0
	);`
	db.Exec(createTableSQL)

	imageHandler := handlers.NewImageHandler(cfg)
	authHandler := handlers.NewAuthHandler(cfg, db)

	// Setup Routes
	app.Post("/auth/login", authHandler.Login)
	app.Get("/resize", middleware.JWTAuthMiddleware(cfg.JWT.Secret), imageHandler.Resize)

	// 1. Login to get JWT
	loginPayload := map[string]string{
		"username": "admin",
		"password": "admin",
	}
	loginBody, _ := json.Marshal(loginPayload)

	req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var loginResp map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &loginResp)
	token := loginResp["token"].(string)

	// 2. Request Resize (Resize cafe.jpg to 300px width)
	targetURL := fmt.Sprintf("/resize?url=%s&width=300", ts.URL)
	reqResize := httptest.NewRequest("GET", targetURL, nil)
	reqResize.Header.Set("Authorization", "Bearer "+token)

	respResize, err := app.Test(reqResize, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, respResize.StatusCode)
	assert.Equal(t, "image/jpeg", respResize.Header.Get("Content-Type"))

	// Save the output for visual inspection
	outData, err := io.ReadAll(respResize.Body)
	assert.NoError(t, err)
	outPath := filepath.Join(projectRoot, "cafe_resized.jpg")
	err = os.WriteFile(outPath, outData, 0644)
	assert.NoError(t, err)
}
