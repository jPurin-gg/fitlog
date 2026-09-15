package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/auth"
	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
)

func TestLoginIssuesSignedCookieUsedByMiddleware(t *testing.T) {
	repository := &authRepository{}
	service := auth.NewService(repository).WithPasswordHashIterations(1)
	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	handler := NewHandler(
		service,
		auth.NewTokenSigner([]byte("0123456789abcdef0123456789abcdef"), time.Hour),
		clock.Fixed{Time: now},
		"fitlog_session",
		false,
		time.Hour,
	)

	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"nickname":"Mitsuki","password":"password"}`))
	loginRecorder := httptest.NewRecorder()
	handler.Login(loginRecorder, loginRequest)
	if loginRecorder.Code != http.StatusCreated {
		t.Fatalf("Login() status = %d, body = %s", loginRecorder.Code, loginRecorder.Body.String())
	}
	cookies := loginRecorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "fitlog_session" || !cookies[0].HttpOnly || cookies[0].Secure || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].MaxAge != 3600 {
		t.Fatalf("session cookie = %#v", cookies)
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meRequest.AddCookie(cookies[0])
	meRecorder := httptest.NewRecorder()
	handler.Authenticate(http.HandlerFunc(handler.Me)).ServeHTTP(meRecorder, meRequest)
	if meRecorder.Code != http.StatusOK || !strings.Contains(meRecorder.Body.String(), `"nickname":"Mitsuki"`) {
		t.Fatalf("authenticated Me() = %d, %s", meRecorder.Code, meRecorder.Body.String())
	}
}

func TestLoginSetsSecureCookieWhenConfigured(t *testing.T) {
	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	handler := NewHandler(
		auth.NewService(&authRepository{}).WithPasswordHashIterations(1),
		auth.NewTokenSigner([]byte("0123456789abcdef0123456789abcdef"), time.Hour),
		clock.Fixed{Time: now},
		"fitlog_session",
		true,
		time.Hour,
	)

	recorder := httptest.NewRecorder()
	handler.Login(recorder, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"nickname":"Mitsuki","password":"password"}`)))
	cookies := recorder.Result().Cookies()
	if recorder.Code != http.StatusCreated || len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly {
		t.Fatalf("Login() = %d, Set-Cookie = %q", recorder.Code, recorder.Header().Get("Set-Cookie"))
	}
}

func TestLoginRejectsMalformedBodyAndWrongPassword(t *testing.T) {
	repository := &authRepository{}
	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	handler := NewHandler(
		auth.NewService(repository).WithPasswordHashIterations(1),
		auth.NewTokenSigner([]byte("0123456789abcdef0123456789abcdef"), time.Hour),
		clock.Fixed{Time: now},
		"fitlog_session",
		false,
		time.Hour,
	)
	registerRecorder := httptest.NewRecorder()
	handler.Login(registerRecorder, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"nickname":"Mitsuki","password":"password"}`)))
	if registerRecorder.Code != http.StatusCreated || repository.user.PasswordHash == "" {
		t.Fatalf("register Login() = %d, stored = %#v", registerRecorder.Code, repository.user)
	}

	tests := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{name: "invalid json", body: `{"nickname":"Mitsuki",`, status: http.StatusBadRequest, code: "VALIDATION_ERROR"},
		{name: "wrong password", body: `{"nickname":"Mitsuki","password":"wrong"}`, status: http.StatusUnauthorized, code: "UNAUTHENTICATED"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.Login(recorder, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(test.body)))
			if recorder.Code != test.status || recorder.Header().Get("Content-Type") != "application/problem+json; charset=utf-8" || !strings.Contains(recorder.Body.String(), `"code":"`+test.code+`"`) || len(recorder.Result().Cookies()) != 0 {
				t.Fatalf("Login() = %d, %q, %q; Set-Cookie = %q", recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.String(), recorder.Header().Get("Set-Cookie"))
			}
		})
	}
}

func TestAuthenticateRejectsMissingGarbageAndExpiredCookies(t *testing.T) {
	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	signer := auth.NewTokenSigner([]byte("0123456789abcdef0123456789abcdef"), time.Hour)
	handler := NewHandler(auth.NewService(&authRepository{}), signer, clock.Fixed{Time: now}, "fitlog_session", false, time.Hour)
	expired, err := signer.Sign(1, now.Add(-2*time.Hour))
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	tests := []struct {
		name   string
		cookie *http.Cookie
		clears bool
	}{
		{name: "no cookie", cookie: nil, clears: false},
		{name: "garbage cookie", cookie: &http.Cookie{Name: "fitlog_session", Value: "garbage"}, clears: true},
		{name: "expired cookie", cookie: &http.Cookie{Name: "fitlog_session", Value: expired}, clears: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
			request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
			if test.cookie != nil {
				request.AddCookie(test.cookie)
			}
			recorder := httptest.NewRecorder()
			handler.Authenticate(next).ServeHTTP(recorder, request)
			if called || recorder.Code != http.StatusUnauthorized || recorder.Header().Get("Content-Type") != "application/problem+json; charset=utf-8" || !strings.Contains(recorder.Body.String(), `"code":"UNAUTHENTICATED"`) {
				t.Fatalf("called = %v, response = %d, %q, %q", called, recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.String())
			}
			cookies := recorder.Result().Cookies()
			if !test.clears {
				if len(cookies) != 0 {
					t.Fatalf("Set-Cookie = %q, want none", recorder.Header().Get("Set-Cookie"))
				}
				return
			}
			if len(cookies) != 1 || cookies[0].Name != "fitlog_session" || cookies[0].Value != "" || cookies[0].MaxAge >= 0 {
				t.Fatalf("clearing Set-Cookie = %q", recorder.Header().Get("Set-Cookie"))
			}
		})
	}
}

func TestLogoutClearsSessionCookie(t *testing.T) {
	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	handler := NewHandler(
		auth.NewService(&authRepository{}),
		auth.NewTokenSigner([]byte("0123456789abcdef0123456789abcdef"), time.Hour),
		clock.Fixed{Time: now},
		"fitlog_session",
		true,
		time.Hour,
	)

	recorder := httptest.NewRecorder()
	handler.Logout(recorder, httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil))
	cookies := recorder.Result().Cookies()
	if recorder.Code != http.StatusNoContent || len(cookies) != 1 {
		t.Fatalf("Logout() = %d, Set-Cookie = %q", recorder.Code, recorder.Header().Get("Set-Cookie"))
	}
	cookie := cookies[0]
	if cookie.Name != "fitlog_session" || cookie.Value != "" || cookie.Path != "/" || cookie.MaxAge >= 0 || !cookie.Expires.Before(now) || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("clearing cookie = %#v", cookie)
	}
}

type authRepository struct {
	user auth.StoredUser
}

func (r *authRepository) FindByNickname(_ context.Context, nickname string) (auth.StoredUser, error) {
	if r.user.ID == 0 || r.user.Nickname != nickname {
		return auth.StoredUser{}, auth.ErrUserNotFound
	}
	return r.user, nil
}

func (r *authRepository) FindByID(_ context.Context, userID int) (auth.User, error) {
	if r.user.ID != userID {
		return auth.User{}, auth.ErrUserNotFound
	}
	return r.user.User, nil
}

func (r *authRepository) Create(_ context.Context, nickname, passwordHash string) (auth.User, error) {
	r.user = auth.StoredUser{User: auth.User{ID: 1, Nickname: nickname}, PasswordHash: passwordHash}
	return r.user.User, nil
}

func (r *authRepository) SetPasswordHash(_ context.Context, _ int, passwordHash string) error {
	r.user.PasswordHash = passwordHash
	return nil
}
