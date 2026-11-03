package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DataDir, Addr, PublicURL  string
	SecureCookie              bool
	ReadTimeout, WriteTimeout time.Duration
}

func Load() (Config, error) {
	c := Config{DataDir: env("PUFFOLIO_DATA_DIR", "/data"), Addr: env("PUFFOLIO_HTTP_ADDR", env("PUFFOLIO_ADDR", ":8080")), PublicURL: env("PUFFOLIO_BASE_URL", env("PUFFOLIO_PUBLIC_URL", "")), ReadTimeout: 120 * time.Second, WriteTimeout: 120 * time.Second}
	b, e := strconv.ParseBool(env("PUFFOLIO_COOKIE_SECURE", env("PUFFOLIO_SECURE_COOKIE", "false")))
	if e != nil {
		return c, fmt.Errorf("PUFFOLIO_COOKIE_SECURE: %w", e)
	}
	c.SecureCookie = b
	return c, nil
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
