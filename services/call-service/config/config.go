package config

import (
	"os"

	"github.com/rs/zerolog/log"
)

type Config struct {
	Port         string
	DatabaseURL  string
	RedisURL     string
	AsteriskURL  string
	AsteriskUser string
	AsteriskPass string
	AsteriskApp  string
}

func LoadConfig() *Config {
	port := getEnv("CALL_SERVICE_PORT", "8081")
	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgrespassword@localhost:5433/dongdo_cs?sslmode=disable")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")
	asteriskURL := getEnv("ASTERISK_URL", "http://localhost:8088")
	asteriskUser := getEnv("ASTERISK_USER", "asterisk")
	asteriskPass := getEnv("ASTERISK_PASS", "callservicesecretpassword")
	asteriskApp := getEnv("ASTERISK_APP", "dongdo-call-app")

	log.Info().
		Str("port", port).
		Str("asterisk_url", asteriskURL).
		Msg("Configuration loaded for Call Service")

	return &Config{
		Port:         port,
		DatabaseURL:  dbURL,
		RedisURL:     redisURL,
		AsteriskURL:  asteriskURL,
		AsteriskUser: asteriskUser,
		AsteriskPass: asteriskPass,
		AsteriskApp:  asteriskApp,
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
