package config

import (
	"os"
)

type Config struct {
	DatabaseURL string
	MediaRoot   string
	Port        string
}

func Load() Config {
	cfg := Config{
		DatabaseURL: getenv("DATABASE_URL", "postgres://sunny:sunny@localhost:5432/thousand_sunny?sslmode=disable"),
		MediaRoot:   getenv("MEDIA_ROOT", "./data"),
		Port:        getenv("PORT", "8080"),
	}
	return cfg
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
