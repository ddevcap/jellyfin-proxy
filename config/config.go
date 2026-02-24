package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// DatabaseURL is the PostgreSQL connection string.
	DatabaseURL string
	// ListenAddr is the address the proxy HTTP server binds to.
	ListenAddr string
	// ExternalURL is the publicly reachable URL for this proxy, reported to clients.
	ExternalURL string
	// ServerID is the UUID the proxy presents as its Jellyfin server ID.
	ServerID string
	// ServerName is the human-readable name reported to clients.
	ServerName string
	// SessionTTL is how long a session token remains valid after its last activity.
	// Set to 0 to disable expiry (not recommended for production).
	SessionTTL time.Duration
	// LoginMaxAttempts is the number of failed login attempts allowed per IP
	// within LoginWindow before the IP is temporarily blocked.
	LoginMaxAttempts int
	// LoginWindow is the sliding window duration for counting failed logins.
	LoginWindow time.Duration
	// LoginBanDuration is how long an IP is blocked after exceeding LoginMaxAttempts.
	LoginBanDuration time.Duration
	// InitialAdminUser is the username for the auto-created admin account on first
	// startup. Only used when no users exist in the database.
	InitialAdminUser string
	// InitialAdminPassword is the plaintext password for the auto-created admin
	// account. Only used when no users exist in the database.
	InitialAdminPassword string
	// DirectStream controls whether streaming requests (video, audio, images,
	// HLS segments) are redirected directly to the backend instead of being
	// piped through the proxy. Requires clients to have direct network access
	// to all backends (e.g. via Tailscale). Default: false.
	DirectStream bool
	// ShutdownTimeout is the maximum duration to wait for in-flight requests
	// to complete during graceful shutdown.
	ShutdownTimeout time.Duration
	// CORSOrigins is an additional set of origins (comma-separated) that are
	// allowed to make credentialed cross-origin requests. The ExternalURL
	// origin is always included automatically.
	CORSOrigins []string
	// BitrateLimit is the maximum bitrate (in bits/s) that clients are allowed
	// to stream at. 0 means unlimited. Applied via the Jellyfin user policy's
	// RemoteClientBitrateLimit field.
	BitrateLimit int
	// HealthCheckInterval is how often the proxy pings each backend to determine
	// availability. Backends that fail 2 consecutive checks are skipped in
	// fan-out requests until they recover. Default: 30s.
	HealthCheckInterval time.Duration
}

func Load() Config {
	return Config{
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://jellyfin:jellyfin@localhost:5432/jellyfin_proxy?sslmode=disable"),
		ListenAddr:           getEnv("LISTEN_ADDR", ":8096"),
		ExternalURL:          getEnv("EXTERNAL_URL", "http://localhost:8096"),
		ServerID:             getEnv("SERVER_ID", "jellyfin-proxy-default-id"),
		ServerName:           getEnv("SERVER_NAME", "Jellyfin Proxy"),
		SessionTTL:           getDuration("SESSION_TTL", 30*24*time.Hour), // 30 days default
		LoginMaxAttempts:     getInt("LOGIN_MAX_ATTEMPTS", 10),
		LoginWindow:          getDuration("LOGIN_WINDOW", 15*time.Minute),
		LoginBanDuration:     getDuration("LOGIN_BAN_DURATION", 15*time.Minute),
		InitialAdminUser:     getEnv("INITIAL_ADMIN_USER", "admin"),
		InitialAdminPassword: getEnv("INITIAL_ADMIN_PASSWORD", ""),
		DirectStream:         getBool("DIRECT_STREAM", false),
		ShutdownTimeout:      getDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
		CORSOrigins:          getStringSlice("CORS_ORIGINS"),
		BitrateLimit:         getInt("BITRATE_LIMIT", 0),
		HealthCheckInterval:  getDuration("HEALTH_CHECK_INTERVAL", 30*time.Second),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		slog.Warn("invalid duration for env var, using default", "key", key, "value", v, "default", fallback)
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
		slog.Warn("invalid integer for env var, using default", "key", key, "value", v, "default", fallback)
	}
	return fallback
}

func getStringSlice(key string) []string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		parts := strings.Split(v, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func getBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
		slog.Warn("invalid boolean for env var, using default", "key", key, "value", v, "default", fallback)
	}
	return fallback
}
