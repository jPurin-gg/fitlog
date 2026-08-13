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
	service := auth.NewService(repository)
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
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].MaxAge != 3600 {
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
