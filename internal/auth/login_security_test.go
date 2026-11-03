package auth

import (
	"context"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/store"
	"strings"
	"testing"
)

const dummyWorkPassword = "dummy-work-only-passphrase"

func TestUnusableLoginHashesPerformRealDummyWorkWithoutEligibility(t *testing.T) {
	if !VerifyPassword(dummyLoginHash, dummyWorkPassword) {
		t.Fatal("dummy fixture must perform a successful real Argon verification")
	}
	cases := []struct {
		name, hash string
		found      bool
	}{
		{"missing", "", false}, {"pending", "invitation-pending", true}, {"invalid-parameters", strings.Replace(dummyLoginHash, "m=65536", "m=1", 1), true},
		{"invalid-salt", strings.Replace(dummyLoginHash, "MDEyMzQ1Njc4OWFiY2RlZg", "bad", 1), true}, {"invalid-key", dummyLoginHash[:len(dummyLoginHash)-10], true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			matched, eligible := checkLoginPassword(tc.hash, dummyWorkPassword, tc.found)
			if !matched {
				t.Fatal("dummy password work was not performed successfully")
			}
			if eligible {
				t.Fatal("unusable/missing credentials became eligible")
			}
		})
	}
}
func TestRealLoginPasswordRetainsEligibilityAndChecksPassword(t *testing.T) {
	hash, e := HashPassword("real-account-passphrase")
	if e != nil {
		t.Fatal(e)
	}
	matched, eligible := checkLoginPassword(hash, "real-account-passphrase", true)
	if !matched || !eligible {
		t.Fatal("valid password rejected")
	}
	matched, eligible = checkLoginPassword(hash, dummyWorkPassword, true)
	if matched || !eligible {
		t.Fatal("real account used dummy hash instead of its own hash")
	}
}
func TestDummyMatchNeverAuthenticatesMissingOrPendingAccount(t *testing.T) {
	ctx := context.Background()
	s, e := store.Open(ctx, config.Config{DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	a := New(s)
	u, _, e := a.Invite(ctx, "pending-user", "Pending", "user")
	if e != nil {
		t.Fatal(e)
	}
	for _, hash := range []string{"invitation-pending", "broken-argon-hash"} {
		if _, e = s.DB.Exec("UPDATE users SET password_hash=? WHERE id=?", hash, u.ID); e != nil {
			t.Fatal(e)
		}
		if _, e = a.Login(ctx, "pending-user", dummyWorkPassword); e == nil {
			t.Fatal("matching dummy work authenticated unusable account")
		}
	}
	if _, e = a.Login(ctx, "missing-user", dummyWorkPassword); e == nil {
		t.Fatal("matching dummy work authenticated missing account")
	}
}
func TestLoginUsernameBoundsAndNormalization(t *testing.T) {
	cases := []struct {
		input, want string
		valid       bool
	}{{"  ALIce  ", "alice", true}, {"Kelvin", "kelvin", true}, {strings.Repeat("a", 64), strings.Repeat("a", 64), true}, {"ab", "", false}, {strings.Repeat("a", 65), "", false}, {strings.Repeat(" ", 300) + "alice", "", false}, {"ali ce", "", false}, {"", "", false}}
	for _, tc := range cases {
		got, valid := loginUsername(tc.input)
		if valid != tc.valid || valid && got != tc.want {
			t.Fatalf("username length%d: valid=%v normalized=%q", len(tc.input), valid, got)
		}
	}
}
