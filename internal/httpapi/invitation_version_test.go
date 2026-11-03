package httpapi

import "testing"

func TestInvitationResponseVersionSupportsFirstProfilePatch(t *testing.T) {
	admin, _ := setupClient(t)
	inv := admin.do("POST", "/api/v1/admin/users", map[string]any{"username": "invited-user", "displayName": "Invited", "role": "user"}, 201)
	c := &client{h: admin.h, t: t}
	activated := c.do("POST", "/api/v1/auth/invite", map[string]any{"token": inv["invitationToken"], "password": "Invite-long-test-password"}, 200)
	returned := activated["user"].(map[string]any)["version"]
	me := c.do("GET", "/api/v1/me", nil, 200)
	if returned != me["user"].(map[string]any)["version"] {
		t.Fatalf("activation version %v differs from current %v", returned, me["user"].(map[string]any)["version"])
	}
	c.do("PATCH", "/api/v1/me", map[string]any{"version": returned, "displayName": "Updated invitee"}, 200)
}
