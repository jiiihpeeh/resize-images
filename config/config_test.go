package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoadDefaults(t *testing.T) {
	os.Clearenv()
	cfg := Load()

	assert.Equal(t, "8080", cfg.Server.Port)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 4096, cfg.Image.MaxWidth)
	assert.Equal(t, 4096, cfg.Image.MaxHeight)
	assert.Equal(t, 80, cfg.Quality.JPEG)
	assert.Equal(t, 80, cfg.Quality.PNG)
	assert.Equal(t, 80, cfg.Quality.WebP)
	assert.Equal(t, 60, cfg.Quality.AVIF)
	assert.Equal(t, 75, cfg.Quality.JXL)
}

func TestLoadFromEnvironment(t *testing.T) {
	os.Clearenv()
	os.Setenv("SERVER_PORT", "9000")
	os.Setenv("SERVER_HOST", "127.0.0.1")
	os.Setenv("MAX_WIDTH", "2048")
	os.Setenv("MAX_HEIGHT", "2048")
	os.Setenv("QUALITY_JPEG", "90")
	os.Setenv("QUALITY_WEBP", "85")
	os.Setenv("QUALITY_AVIF", "50")
	os.Setenv("QUALITY_JXL", "70")
	os.Setenv("QUALITY_PNG", "75")
	os.Setenv("AVIF_MAX_RESOLUTION", "1024")
	os.Setenv("JXL_MAX_RESOLUTION", "800")
	os.Setenv("AVIF_MAX_PIXELS", "1000000")
	os.Setenv("JXL_MAX_PIXELS", "800000")
	os.Setenv("ENABLE_AVIF", "false")
	os.Setenv("ENABLE_JXL", "false")

	cfg := Load()

	assert.Equal(t, "9000", cfg.Server.Port)
	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 2048, cfg.Image.MaxWidth)
	assert.Equal(t, 2048, cfg.Image.MaxHeight)
	assert.Equal(t, 90, cfg.Quality.JPEG)
	assert.Equal(t, 85, cfg.Quality.WebP)
	assert.Equal(t, 50, cfg.Quality.AVIF)
	assert.Equal(t, 70, cfg.Quality.JXL)
	assert.Equal(t, 75, cfg.Quality.PNG)
	assert.Equal(t, 1024, cfg.Constraints.AVIFMaxResolution)
	assert.Equal(t, 800, cfg.Constraints.JXLMaxResolution)
	assert.Equal(t, 1000000, cfg.Constraints.AVIFMaxPixels)
	assert.Equal(t, 800000, cfg.Constraints.JXLMaxPixels)
	assert.False(t, cfg.Constraints.EnableAVIF)
	assert.False(t, cfg.Constraints.EnableJXL)
}

func TestJWTConfig(t *testing.T) {
	os.Clearenv()
	os.Setenv("JWT_SECRET", "my-secret")
	os.Setenv("JWT_EXPIRY", "2h")
	os.Setenv("JWT_REFRESH_EXPIRY", "7d")

	cfg := Load()

	assert.Equal(t, "my-secret", cfg.JWT.Secret)
	assert.Equal(t, 2*time.Hour, cfg.JWT.Expiry)
	assert.Equal(t, 7*24*time.Hour, cfg.JWT.RefreshExpiry)
}

func TestImageTimeout(t *testing.T) {
	os.Clearenv()
	os.Setenv("IMAGE_TIMEOUT", "60s")

	cfg := Load()

	assert.Equal(t, 60*time.Second, cfg.Image.Timeout)
}

func TestAllowedHosts(t *testing.T) {
	os.Clearenv()
	os.Setenv("ALLOWED_HOSTS", "example.com,images.example.com,cdn.example.com")

	cfg := Load()

	assert.Len(t, cfg.Image.AllowedHosts, 3)
	assert.Contains(t, cfg.Image.AllowedHosts, "example.com")
	assert.Contains(t, cfg.Image.AllowedHosts, "images.example.com")
	assert.Contains(t, cfg.Image.AllowedHosts, "cdn.example.com")
}

func TestEmptyAllowedHosts(t *testing.T) {
	os.Clearenv()

	cfg := Load()

	assert.Empty(t, cfg.Image.AllowedHosts)
}

func TestConstraintsDefaults(t *testing.T) {
	os.Clearenv()
	cfg := Load()

	assert.Equal(t, 2048, cfg.Constraints.AVIFMaxResolution)
	assert.Equal(t, 1920, cfg.Constraints.JXLMaxResolution)
	assert.Equal(t, 2500000, cfg.Constraints.AVIFMaxPixels)
	assert.Equal(t, 2000000, cfg.Constraints.JXLMaxPixels)
	assert.True(t, cfg.Constraints.EnableAVIF)
	assert.True(t, cfg.Constraints.EnableJXL)
}

func TestRateLimitConfig(t *testing.T) {
	os.Clearenv()
	os.Setenv("MAX_CONCURRENT", "5")
	os.Setenv("REQUESTS_PER_MIN", "120")

	cfg := Load()

	assert.Equal(t, 5, cfg.RateLimit.MaxConcurrent)
	assert.Equal(t, 120, cfg.RateLimit.RequestsPerMin)
}

func TestRateLimitDefaults(t *testing.T) {
	os.Clearenv()
	cfg := Load()

	assert.Equal(t, 2, cfg.RateLimit.MaxConcurrent)
	assert.Equal(t, 60, cfg.RateLimit.RequestsPerMin)
}
