//go:build integration

package app_test

import (
	"context"
	"net/http"
	"testing"
)

type sessionBody struct {
	ID       int    `json:"id"`
	Nickname string `json:"nickname"`
	Created  bool   `json:"created"`
}

// Regression test for #89 against real PostgreSQL: logging in never registers, and a nickname that
// differs only by letter case or character width resolves to the registered account.
func TestLoginAndRegisterAreSeparateAndFoldNicknamesHTTP(t *testing.T) {
	f := newManualAPIFixture(t, nil) // registers "integration-user"
	const password = "test-only-password"
	credentials := func(nickname, password string) map[string]string {
		return map[string]string{"nickname": nickname, "password": password}
	}
	registered := decodeAPI[sessionBody](t, f.request(t, "GET", "/api/auth/me", nil, "", http.StatusOK))

	f.request(t, "POST", "/api/auth/login", credentials("integration-usr", password), "", http.StatusUnauthorized)
	for _, variant := range []string{"integration-user", "Integration-User", "INTEGRATION-USER", "ｉｎｔｅｇｒａｔｉｏｎ－ｕｓｅｒ"} {
		loggedIn := decodeAPI[sessionBody](t, f.request(t, "POST", "/api/auth/login", credentials(variant, password), "", http.StatusOK))
		if loggedIn.ID != registered.ID || loggedIn.Nickname != "integration-user" || loggedIn.Created {
			t.Fatalf("login as %q = %+v, want user %d", variant, loggedIn, registered.ID)
		}
		f.request(t, "POST", "/api/auth/login", credentials(variant, "wrong-password"), "", http.StatusUnauthorized)
		f.request(t, "POST", "/api/auth/register", credentials(variant, password), "", http.StatusConflict)
	}

	other := decodeAPI[sessionBody](t, f.request(t, "POST", "/api/auth/register", credentials("別のユーザー", password), "", http.StatusCreated))
	if other.ID == registered.ID || other.Nickname != "別のユーザー" || !other.Created {
		t.Fatalf("second registration = %+v", other)
	}

	var users int
	if err := f.db.QueryRowContext(context.Background(), `SELECT count(*) FROM users`).Scan(&users); err != nil {
		t.Fatal(err)
	}
	if users != 2 {
		t.Fatalf("users = %d, want the two registered ones", users)
	}
}
