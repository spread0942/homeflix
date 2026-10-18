package config

import (
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL    string
	MediaRoot      string
	Port           string
	MaxUploadBytes int64
}

func Load() Config {
	cfg := Config{
		DatabaseURL:    getenv("DATABASE_URL", "postgres://sunny:sunny@localhost:5432/thousand_sunny?sslmode=disable"),
		MediaRoot:      getenv("MEDIA_ROOT", "./data"),
		Port:           getenv("PORT", "8080"),
		MaxUploadBytes: getenvInt64("MAX_UPLOAD_GB", 50) << 30,
	}
	return cfg
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}
