package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/auth"
	"github.com/jPurin-gg/myfitlog-backend/internal/httpx"
)

func TestLoginRateLimitReturnsRetryTimeAndRecoversWithoutLeakingCredentials(t *testing.T) {
	appClock := &loginClock{now: time.Date(2026, time.November, 1, 12, 0, 0, 250_000_000, time.UTC)}
	handler, repository := newLimitedHandler(appClock, auth.LoginLimits{SourceAttempts: 10, AccountAttempts: 2, Window: time.Minute})
	var logs bytes.Buffer
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", handler.Login)
	logged := httpx.WithRequestID(httpx.LogRequests(slog.New(slog.NewJSONHandler(&logs, nil)), mux))
	const password = "synthetic-login-password"
	const badPassword = "synthetic-incorrect-password"
	const cookie = "synthetic-cookie-marker"
	var bodies strings.Builder
	perform := func(password, source string) *httptest.ResponseRecorder {
		t.Helper()
		request := loginHTTPTestRequest(t, "Account", password, source)
		request.AddCookie(&http.Cookie{Name: "fitlog_session", Value: cookie})
		recorder := httptest.NewRecorder()
		logged.ServeHTTP(recorder, request)
		bodies.WriteString(recorder.Body.String())
		return recorder
	}
	if got := perform(password, "198.51.100.1:1000"); got.Code != http.StatusCreated {
		t.Fatalf("registration status = %d", got.Code)
	}
	if got := perform(badPassword, "198.51.100.2:1000"); got.Code != http.StatusUnauthorized {
		t.Fatalf("incorrect password status = %d", got.Code)
	}
	lookups := repository.lookups
	limited := perform(password, "198.51.100.3:1000")
	if limited.Code != http.StatusTooManyRequests || !strings.Contains(limited.Body.String(), `"code":"RATE_LIMITED"`) || limited.Header().Get("Content-Type") != "application/problem+json; charset=utf-8" {
		t.Fatalf("limited response = %d, %s", limited.Code, limited.Body.String())
	}
	if repository.lookups != lookups || len(limited.Result().Cookies()) != 0 {
		t.Fatal("limited request accessed repository or issued a cookie")
	}
	retryAt, err := http.ParseTime(limited.Header().Get("Retry-After"))
	if err != nil || !retryAt.Equal(appClock.Now().Truncate(time.Second).Add(time.Minute+time.Second)) {
		t.Fatalf("Retry-After = %q, error = %v", limited.Header().Get("Retry-After"), err)
	}
	appClock.now = appClock.now.Add(time.Minute)
	if got := perform(password, "198.51.100.3:1000"); got.Code != http.StatusOK || len(got.Result().Cookies()) != 1 {
		t.Fatalf("login after expiry status = %d", got.Code)
	}
	for _, secret := range []string{password, badPassword, cookie, repository.users["Account"].PasswordHash, "pbkdf2_sha256$"} {
		if strings.Contains(logs.String(), secret) || strings.Contains(bodies.String(), secret) {
			t.Fatal("credentials appeared in access logs or response bodies")
		}
	}
	if !strings.Contains(logs.String(), `"status":429`) {
		t.Fatal("access log did not record the limited request")
	}
}

func TestLoginSourceLimitStopsMultipleRegistrationsAndForgedHeaders(t *testing.T) {
	appClock := &loginClock{now: time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC)}
	handler, repository := newLimitedHandler(appClock, auth.LoginLimits{SourceAttempts: 2, AccountAttempts: 10, Window: time.Minute})
	for index, nickname := range []string{"First", "Second", "Third"} {
		request := loginHTTPTestRequest(t, nickname, "synthetic-password", "198.51.100.1:1000")
		request.Header.Set("X-Forwarded-For", []string{"203.0.113.1", "203.0.113.2", "203.0.113.3"}[index])
		recorder := httptest.NewRecorder()
		handler.Login(recorder, request)
		want := http.StatusCreated
		if index == 2 {
			want = http.StatusTooManyRequests
		}
		if recorder.Code != want {
			t.Fatalf("registration %d status = %d, want %d", index+1, recorder.Code, want)
		}
	}
	if len(repository.users) != 2 || repository.lookups != 2 {
		t.Fatalf("users = %d, lookups = %d, want 2 each", len(repository.users), repository.lookups)
	}
}

func TestLoginTrustedProxySeparatesClientsAndIgnoresSpoofedPrefix(t *testing.T) {
	appClock := &loginClock{now: time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC)}
	handler, repository := newLimitedHandler(appClock, auth.LoginLimits{SourceAttempts: 1, AccountAttempts: 10, Window: time.Minute})
	handler.WithTrustedProxies([]netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")})
	for index, test := range []struct {
		nickname string
		header   string
		status   int
	}{
		{nickname: "First", header: "203.0.113.1, 198.51.100.1, 10.0.0.2", status: http.StatusCreated},
		{nickname: "Second", header: "203.0.113.1, 198.51.100.2, 10.0.0.2", status: http.StatusCreated},
		{nickname: "Third", header: "203.0.113.99, 198.51.100.1, 10.0.0.2", status: http.StatusTooManyRequests},
	} {
		request := loginHTTPTestRequest(t, test.nickname, "synthetic-password", "10.0.0.1:1000")
		request.Header.Set("X-Forwarded-For", test.header)
		recorder := httptest.NewRecorder()
		handler.Login(recorder, request)
		if recorder.Code != test.status {
			t.Fatalf("request %d status = %d, want %d", index+1, recorder.Code, test.status)
		}
	}
	if len(repository.users) != 2 {
		t.Fatalf("users = %d, want 2", len(repository.users))
	}
}

func TestLoginAccountLimitUsesTrimmedNicknameAcrossSources(t *testing.T) {
	appClock := &loginClock{now: time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC)}
	handler, repository := newLimitedHandler(appClock, auth.LoginLimits{SourceAttempts: 10, AccountAttempts: 1, Window: time.Minute})
	first := httptest.NewRecorder()
	handler.Login(first, loginHTTPTestRequest(t, " Account ", "synthetic-password", "198.51.100.1:1000"))
	second := httptest.NewRecorder()
	handler.Login(second, loginHTTPTestRequest(t, "Account", "synthetic-password", "198.51.100.2:2000"))
	if first.Code != http.StatusCreated || second.Code != http.StatusTooManyRequests || repository.lookups != 1 {
		t.Fatalf("statuses = %d, %d; lookups = %d", first.Code, second.Code, repository.lookups)
	}
}

func TestLoginParallelRequestsCannotExceedAccountBudget(t *testing.T) {
	appClock := &loginClock{now: time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC)}
	handler, repository := newLimitedHandler(appClock, auth.LoginLimits{
		SourceAttempts: 100, AccountAttempts: 3, Window: time.Minute, MaxConcurrentHashes: 32,
	})
	const requests = 32
	start := make(chan struct{})
	statuses := make(chan int, requests)
	var workers sync.WaitGroup
	for index := 0; index < requests; index++ {
		request := loginHTTPTestRequest(t, "Account", "synthetic-password", "198.51.100.1:1000")
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			recorder := httptest.NewRecorder()
			handler.Login(recorder, request)
			statuses <- recorder.Code
		}()
	}
	close(start)
	workers.Wait()
	close(statuses)
	allowed, limited := 0, 0
	for status := range statuses {
		switch status {
		case http.StatusOK, http.StatusCreated, http.StatusConflict:
			allowed++
		case http.StatusTooManyRequests:
			limited++
		default:
			t.Fatalf("parallel login status = %d", status)
		}
	}
	if allowed != 3 || limited != requests-3 || repository.lookups != 3 || len(repository.users) != 1 {
		t.Fatalf("allowed = %d, limited = %d, lookups = %d, users = %d", allowed, limited, repository.lookups, len(repository.users))
	}
}

type loginClock struct{ now time.Time }

func (c *loginClock) Now() time.Time { return c.now }

func newLimitedHandler(appClock *loginClock, limits auth.LoginLimits) (*Handler, *loginRepository) {
	repository := &loginRepository{users: map[string]auth.StoredUser{}}
	service := auth.NewService(repository).WithPasswordHashIterations(1).WithLoginLimits(limits, appClock)
	return NewHandler(service, auth.NewTokenSigner([]byte("0123456789abcdef0123456789abcdef"), time.Hour), appClock, "fitlog_session", false, time.Hour), repository
}

func loginHTTPTestRequest(t *testing.T, nickname, password, source string) *http.Request {
	t.Helper()
	body, err := json.Marshal(loginRequest{Nickname: nickname, Password: password})
	if err != nil {
		t.Fatal("could not encode synthetic login request")
	}
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	request.RemoteAddr = source
	return request
}

type loginRepository struct {
	mu      sync.Mutex
	users   map[string]auth.StoredUser
	lookups int
}

func (r *loginRepository) FindByNickname(_ context.Context, nickname string) (auth.StoredUser, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lookups++
	user, ok := r.users[nickname]
	if !ok {
		return auth.StoredUser{}, auth.ErrUserNotFound
	}
	return user, nil
}

func (r *loginRepository) FindByID(_ context.Context, userID int) (auth.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, user := range r.users {
		if user.ID == userID {
			return user.User, nil
		}
	}
	return auth.User{}, auth.ErrUserNotFound
}

func (r *loginRepository) Create(_ context.Context, nickname, passwordHash string) (auth.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.users[nickname]; exists {
		return auth.User{}, auth.ErrNicknameTaken
	}
	user := auth.User{ID: len(r.users) + 1, Nickname: nickname}
	r.users[nickname] = auth.StoredUser{User: user, PasswordHash: passwordHash}
	return user, nil
}

func (r *loginRepository) SetPasswordHash(_ context.Context, userID int, passwordHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for nickname, user := range r.users {
		if user.ID == userID {
			user.PasswordHash = passwordHash
			r.users[nickname] = user
			return nil
		}
	}
	return auth.ErrUserNotFound
}
