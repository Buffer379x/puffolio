package config

import (
	"strings"
	"testing"
)

func clearPublicEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"PUFFOLIO_BASE_URL", "PUFFOLIO_PUBLIC_URL", "PUFFOLIO_COOKIE_SECURE", "PUFFOLIO_SECURE_COOKIE"} {
		t.Setenv(k, "")
	}
}
func TestCanonicalDeploymentEnvOverridesLegacyAliases(t *testing.T) {
	clearPublicEnv(t)
	t.Setenv("PUFFOLIO_BASE_URL", "https://puffolio.example")
	t.Setenv("PUFFOLIO_PUBLIC_URL", "http://legacy.invalid")
	t.Setenv("PUFFOLIO_COOKIE_SECURE", "true")
	t.Setenv("PUFFOLIO_SECURE_COOKIE", "false")
	c, e := Load()
	if e != nil {
		t.Fatal(e)
	}
	if c.PublicURL != "https://puffolio.example" || !c.SecureCookie {
		t.Fatalf("canonical deployment env ignored: origin=%q secure=%v", c.PublicURL, c.SecureCookie)
	}
}
func TestCanonicalFalseOverridesLegacySecureTrue(t *testing.T) {
	clearPublicEnv(t)
	t.Setenv("PUFFOLIO_COOKIE_SECURE", "false")
	t.Setenv("PUFFOLIO_SECURE_COOKIE", "true")
	c, e := Load()
	if e != nil || c.SecureCookie {
		t.Fatal("canonical false lost precedence", c.SecureCookie, e)
	}
}
func TestLegacyDeploymentEnvRemainsFallback(t *testing.T) {
	clearPublicEnv(t)
	t.Setenv("PUFFOLIO_PUBLIC_URL", "https://legacy.example")
	t.Setenv("PUFFOLIO_SECURE_COOKIE", "true")
	c, e := Load()
	if e != nil || c.PublicURL != "https://legacy.example" || !c.SecureCookie {
		t.Fatal(c, e)
	}
}
func TestInvalidCanonicalCookieSettingFailsLoad(t *testing.T) {
	clearPublicEnv(t)
	t.Setenv("PUFFOLIO_COOKIE_SECURE", "invalid")
	t.Setenv("PUFFOLIO_SECURE_COOKIE", "false")
	if _, e := Load(); e == nil || !strings.Contains(e.Error(), "PUFFOLIO_COOKIE_SECURE") {
		t.Fatal("invalid canonical cookie setting ignored", e)
	}
}
