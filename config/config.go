package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server      ServerConfig
	JWT         JWTConfig
	Image       ImageConfig
	Quality     QualityConfig
	Constraints ConstraintsConfig
	RateLimit   RateLimitConfig
}

type RateLimitConfig struct {
	MaxConcurrent  int
	RequestsPerMin int
}

type ServerConfig struct {
	Port string
	Host string
}

type JWTConfig struct {
	Secret        string
	Expiry        time.Duration
	RefreshExpiry time.Duration
}

type ImageConfig struct {
	MaxWidth     int
	MaxHeight    int
	AllowedHosts []string
	Timeout      time.Duration
}

type QualityConfig struct {
	JPEG int
	PNG  int
	WebP int
	AVIF int
	JXL  int
}

type ConstraintsConfig struct {
	AVIFMaxResolution int
	JXLMaxResolution  int
	AVIFMaxPixels     int
	JXLMaxPixels      int
	EnableJXL         bool
	EnableAVIF        bool
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
		},
		JWT: JWTConfig{
			Secret:        getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
			Expiry:        getDurationEnv("JWT_EXPIRY", 24*time.Hour),
			RefreshExpiry: getDurationEnv("JWT_REFRESH_EXPIRY", 7*24*time.Hour),
		},
		Image: ImageConfig{
			MaxWidth:     getIntEnv("MAX_WIDTH", 4096),
			MaxHeight:    getIntEnv("MAX_HEIGHT", 4096),
			AllowedHosts: getSliceEnv("ALLOWED_HOSTS", []string{}),
			Timeout:      getDurationEnv("IMAGE_TIMEOUT", 30*time.Second),
		},
		Quality: QualityConfig{
			JPEG: getIntEnv("QUALITY_JPEG", 80),
			PNG:  getIntEnv("QUALITY_PNG", 80),
			WebP: getIntEnv("QUALITY_WEBP", 80),
			AVIF: getIntEnv("QUALITY_AVIF", 60),
			JXL:  getIntEnv("QUALITY_JXL", 75),
		},
		Constraints: ConstraintsConfig{
			AVIFMaxResolution: getIntEnv("AVIF_MAX_RESOLUTION", 2048),
			JXLMaxResolution:  getIntEnv("JXL_MAX_RESOLUTION", 1920),
			AVIFMaxPixels:     getIntEnv("AVIF_MAX_PIXELS", 2500000),
			JXLMaxPixels:      getIntEnv("JXL_MAX_PIXELS", 2000000),
			EnableJXL:         getEnv("ENABLE_JXL", "true") == "true",
			EnableAVIF:        getEnv("ENABLE_AVIF", "true") == "true",
		},
		RateLimit: RateLimitConfig{
			MaxConcurrent:  getIntEnv("MAX_CONCURRENT", 2),
			RequestsPerMin: getIntEnv("REQUESTS_PER_MIN", 60),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getSliceEnv(key string, defaultValue []string) []string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return splitAndTrim(value, ",")
	}
	return defaultValue
}

func splitAndTrim(s string, sep string) []string {
	parts := make([]string, 0)
	for _, p := range split(s, sep) {
		trimmed := trim(p)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func split(s, sep string) []string {
	result := make([]string, 0)
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trim(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
