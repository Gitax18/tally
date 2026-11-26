// Package config loads and validates the server's configuration from the environment.
//
// Validation reports every problem at once and names the variable, but never includes a value:
// values can be secrets.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	minSecretLength        = 32
	defaultRetentionMonths = 24
	defaultDBLimitMB       = 1024 // Heroku Postgres Essential-0
	defaultShutdownTimeout = 25 * time.Second
	maxShutdownTimeout     = 60 * time.Second
)

// Config is the validated configuration of the server.
type Config struct {
	Addr              string // host:port to listen on; PORT on Heroku
	DatabaseURL       string
	APISecret         string // bearer token for /v1/*; never logged
	AdminPasswordHash string // scrypt hash checked by POST /v1/login; never logged
	RetentionMonths   int
	DBLimitBytes      int64  // storage limit of the database plan, for the usage meter
	GeoIPPath         string // a local .mmdb file; when empty the DB-IP file is downloaded
	GeoIPDownload     bool   // false skips the download (tests, offline development)
	TrustProxy        bool   // true behind the Heroku router: the last X-Forwarded-For hop is the client
	LogLevel          slog.Level
	ShutdownTimeout   time.Duration
}

// Load reads and validates the configuration.
func Load(getenv func(string) string) (Config, error) {
	var c Config
	var errs []error

	port := value(getenv, "PORT", "8090")
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		errs = append(errs, fieldError("PORT", errors.New("must be a port number")))
	}
	c.Addr = value(getenv, "LISTEN_HOST", "") + ":" + port

	c.DatabaseURL = value(getenv, "DATABASE_URL", "")
	if c.DatabaseURL == "" {
		errs = append(errs, fieldError("DATABASE_URL", errors.New("is required")))
	} else if u, err := url.Parse(c.DatabaseURL); err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		errs = append(errs, fieldError("DATABASE_URL", errors.New("must be a postgres:// URL")))
	}

	c.APISecret = value(getenv, "API_SECRET", "")
	if err := validateSecret(c.APISecret); err != nil {
		errs = append(errs, fieldError("API_SECRET", err))
	}

	c.AdminPasswordHash = value(getenv, "ADMIN_PASSWORD_HASH", "")
	if c.AdminPasswordHash == "" {
		errs = append(errs, fieldError("ADMIN_PASSWORD_HASH", errors.New("is required (generate with: go run ./cmd/server hash-password)")))
	} else if !strings.HasPrefix(c.AdminPasswordHash, "scrypt$") {
		errs = append(errs, fieldError("ADMIN_PASSWORD_HASH", errors.New("must be the output of: go run ./cmd/server hash-password")))
	}

	c.RetentionMonths = defaultRetentionMonths
	if raw := value(getenv, "RETENTION_MONTHS", ""); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 120 {
			errs = append(errs, fieldError("RETENTION_MONTHS", errors.New("must be a whole number of months from 1 to 120")))
		} else {
			c.RetentionMonths = n
		}
	}

	limitMB := int64(defaultDBLimitMB)
	if raw := value(getenv, "DB_LIMIT_MB", ""); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 1 {
			errs = append(errs, fieldError("DB_LIMIT_MB", errors.New("must be a positive number of megabytes")))
		} else {
			limitMB = n
		}
	}
	c.DBLimitBytes = limitMB << 20

	c.GeoIPPath = value(getenv, "GEOIP_PATH", "")
	c.GeoIPDownload = value(getenv, "GEOIP_DOWNLOAD", "true") != "false"

	// Heroku sets DYNO on every dyno; there the router always appends the client address.
	trustDefault := "false"
	if getenv("DYNO") != "" {
		trustDefault = "true"
	}
	switch value(getenv, "TRUST_PROXY", trustDefault) {
	case "true":
		c.TrustProxy = true
	case "false":
		c.TrustProxy = false
	default:
		errs = append(errs, fieldError("TRUST_PROXY", errors.New("must be true or false")))
	}

	if err := c.LogLevel.UnmarshalText([]byte(value(getenv, "LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, fieldError("LOG_LEVEL", errors.New("must be debug, info, warn or error")))
	}

	c.ShutdownTimeout = defaultShutdownTimeout
	if raw := value(getenv, "SHUTDOWN_TIMEOUT", ""); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d <= 0 || d > maxShutdownTimeout {
			errs = append(errs, fieldError("SHUTDOWN_TIMEOUT", errors.New("must be a duration up to 60s, for example 25s")))
		} else {
			c.ShutdownTimeout = d
		}
	}

	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return c, nil
}

func validateSecret(v string) error {
	if v == "" {
		return errors.New("is required (generate with: openssl rand -hex 32)")
	}
	if len(v) < minSecretLength {
		return fmt.Errorf("must be at least %d characters", minSecretLength)
	}
	if strings.Contains(strings.ToLower(v), "change-me") {
		return errors.New("is still the placeholder from .env.example")
	}
	for _, r := range v {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_'
		if !ok {
			return errors.New("may only contain letters, digits, - and _")
		}
	}
	return nil
}

func value(getenv func(string) string, key, fallback string) string {
	if v := strings.TrimSpace(getenv(key)); v != "" {
		return v
	}
	return fallback
}

func fieldError(key string, err error) error { return fmt.Errorf("%s: %w", key, err) }
