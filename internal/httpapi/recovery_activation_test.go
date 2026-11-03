package httpapi

import (
	"bytes"
	"context"
	"github.com/Buffer379x/puffolio/internal/auth"
	"github.com/Buffer379x/puffolio/internal/backup"
	"github.com/Buffer379x/puffolio/internal/config"
	"github.com/Buffer379x/puffolio/internal/store"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPendingRecoveryInitializesApplication(t *testing.T) {
	for _, restored := range []bool{false, true} {
		name := "expired"
		if restored {
			name = "restored"
		}
		t.Run(name, func(t *testing.T) {
			c, s := setupClient(t)
			inv := c.do("POST", "/api/v1/admin/users", map[string]any{"username": "pending", "displayName": "Pending", "role": "user"}, 201)
			id := inv["user"].(map[string]any)["id"].(string)
			if _, e := s.DB.Exec("UPDATE one_time_tokens SET expires_at=? WHERE user_id=?", time.Now().Add(-time.Hour).Unix(), id); e != nil {
				t.Fatal(e)
			}
			anonymous := &client{h: c.h, t: t}
			anonymous.do("POST", "/api/v1/auth/invite", map[string]any{"token": inv["invitationToken"], "password": "Pending-long-password"}, 422)
			if restored {
				var archive bytes.Buffer
				if e := backup.Create(context.Background(), s, &archive, "test"); e != nil {
					t.Fatal(e)
				}
				p := filepath.Join(t.TempDir(), "backup.zip")
				if e := os.WriteFile(p, archive.Bytes(), 0600); e != nil {
					t.Fatal(e)
				}
				dest := filepath.Join(t.TempDir(), "restored")
				if e := backup.Restore(context.Background(), p, dest); e != nil {
					t.Fatal(e)
				}
				other, e := store.Open(context.Background(), config.Config{DataDir: dest})
				if e != nil {
					t.Fatal(e)
				}
				defer other.Close()
				s = other
				c.h = New(Dependencies{Store: s, Config: config.Config{DataDir: s.Dir}})
			}
			if e := auth.New(s).ResetPassword(context.Background(), "pending", "Recovered-long-password"); e != nil {
				t.Fatal(e)
			}
			recovered := &client{h: c.h, t: t}
			recovered.do("POST", "/api/v1/auth/login", map[string]any{"username": "pending", "password": "Recovered-long-password"}, 200)
			recovered.do("GET", "/api/v1/preferences", nil, 200)
			recovered.do("GET", "/api/v1/dashboard", nil, 200)
			var tokens int
			s.DB.QueryRow("SELECT count(*) FROM one_time_tokens WHERE user_id=?", id).Scan(&tokens)
			if tokens != 0 {
				t.Fatal("recovery retained invitation")
			}
			anonymous.h = c.h
			anonymous.do("POST", "/api/v1/auth/invite", map[string]any{"token": inv["invitationToken"], "password": "Pending-long-password"}, 422)
			// A second recovery must retain existing settings, versions and collection data.
			_, e := s.DB.Exec("UPDATE user_preferences SET data='{}',version=7 WHERE user_id=?", id)
			if e != nil {
				t.Fatal(e)
			}
			_, e = s.DB.Exec("UPDATE dashboards SET data='[]',version=9 WHERE user_id=?", id)
			if e != nil {
				t.Fatal(e)
			}
			recovered.do("POST", "/api/v1/liquids", map[string]any{"name": "Preserved"}, 201)
			if e = auth.New(s).ResetPassword(context.Background(), "pending", "Another-long-password"); e != nil {
				t.Fatal(e)
			}
			var data string
			var version int
			if e = s.DB.QueryRow("SELECT data,version FROM user_preferences WHERE user_id=?", id).Scan(&data, &version); e != nil || data != "{}" || version != 7 {
				t.Fatalf("preferences overwritten: %s/%d %v", data, version, e)
			}
			if e = s.DB.QueryRow("SELECT data,version FROM dashboards WHERE user_id=?", id).Scan(&data, &version); e != nil || data != "[]" || version != 9 {
				t.Fatalf("dashboard overwritten: %s/%d %v", data, version, e)
			}
			recovered = &client{h: c.h, t: t}
			recovered.do("POST", "/api/v1/auth/login", map[string]any{"username": "pending", "password": "Another-long-password"}, 200)
			if recovered.do("GET", "/api/v1/liquids", nil, 200)["total"] != float64(1) {
				t.Fatal("recovery lost content")
			}
		})
	}
}

func TestRecoveryFillsOnlyMissingApplicationRows(t *testing.T) {
	for _, existing := range []string{"user_preferences", "dashboards"} {
		t.Run(existing, func(t *testing.T) {
			c, s := setupClient(t)
			ctx := context.Background()
			u, _, e := auth.New(s).Invite(ctx, "partial", "Partial", "user")
			if e != nil {
				t.Fatal(e)
			}
			raw := `[]`
			if existing == "user_preferences" {
				raw = `{"theme":"dark","timezone":"UTC","view":"list","images":false,"listFields":[],"gridFields":[],"roles":{}}`
			}
			if _, e = s.DB.Exec("INSERT INTO "+existing+"(user_id,data,version) VALUES(?,?,7)", u.ID, raw); e != nil {
				t.Fatal(e)
			}
			if e = auth.New(s).ResetPassword(ctx, "partial", "Partial-long-password"); e != nil {
				t.Fatal(e)
			}
			var preserved string
			var version int
			if e = s.DB.QueryRow("SELECT data,version FROM "+existing+" WHERE user_id=?", u.ID).Scan(&preserved, &version); e != nil || preserved != raw || version != 7 {
				t.Fatalf("existing state replaced: %s/%d %v", preserved, version, e)
			}
			recovered := &client{h: c.h, t: t}
			recovered.do("POST", "/api/v1/auth/login", map[string]any{"username": "partial", "password": "Partial-long-password"}, 200)
			recovered.do("GET", "/api/v1/preferences", nil, 200)
			recovered.do("GET", "/api/v1/dashboard", nil, 200)
		})
	}
}
