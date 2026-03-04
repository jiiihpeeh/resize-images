//go:build integration
// +build integration

package test_scripts

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"

	"image-resizer/config"
	"image-resizer/handlers"
)

// Integration tests for image handler
// Note: These tests require libvips to be installed and are tagged with integration
// Run with: go test -tags=integration ./test_scripts/...

func setupTestApp() *fiber.App {
	cfg := config.Load()
	app := fiber.New()
	imageHandler := handlers.NewImageHandler(cfg)
	app.Get("/health", imageHandler.Health)
	return app
}

func TestHealthEndpoint(t *testing.T) {
	app := setupTestApp()

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestHealthResponse(t *testing.T) {
	app := setupTestApp()

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(body), "healthy")
	assert.Contains(t, string(body), "version")
}
