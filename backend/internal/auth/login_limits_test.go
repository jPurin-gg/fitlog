package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
)

func TestLoginAccountLimitPrecedesLookupAndVerificationAndExpires(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	hash, err := hashPassword("synthetic-correct", 1)
	if err != nil {
		t.Fatal("prepare synthetic password")
	}
	repo := &limitedLoginRepository{users: map[string]StoredUser{"account": {User: User{ID: 1, Nickname: "account"}, PasswordHash: hash}}}
	appClock := &clock.Fixed{Time: now}
	passwords := &observedPasswords{}
	service := NewService(repo).WithLoginLimits(LoginLimits{AccountAttempts: 3}, appClock)
	service.passwords = passwords
	for i := 0; i < 3; i++ {
		_, _, err := service.LoginFrom(context.Background(), fmt.Sprintf("192.0.2.%d", i+1), " account ", "synthetic-wrong")
		if apperr.As(err).Status != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected unauthorized", i)
		}
	}
	for i := 0; i < 2; i++ {
		_, _, err := service.LoginFrom(context.Background(), "198.51.100.1", "account", "synthetic-correct")
		assertLoginLimited(t, err, now.Add(5*time.Minute))
	}
	if repo.lookups != 3 || passwords.verifications != 3 {
		t.Fatalf("blocked requests did work: lookups=%d verifications=%d", repo.lookups, passwords.verifications)
	}
	appClock.Time = now.Add(5*time.Minute - time.Nanosecond)
	_, _, err = service.LoginFrom(context.Background(), "198.51.100.1", "account", "synthetic-correct")
	assertLoginLimited(t, err, now.Add(5*time.Minute))
	appClock.Time = now.Add(5 * time.Minute)
	user, created, err := service.LoginFrom(context.Background(), "198.51.100.1", "account", "synthetic-correct")
	if err != nil || created || user.ID != 1 || passwords.verifications != 4 {
		t.Fatal("correct login did not recover at the window boundary")
	}
}

func TestLoginSourceLimitIncludesRegistrationAndDifferentAccounts(t *testing.T) {
	for _, registered := range []bool{false, true} {
		t.Run(fmt.Sprintf("registered=%v", registered), func(t *testing.T) {
			now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
			repo := &limitedLoginRepository{users: map[string]StoredUser{}}
			if registered {
				hash, err := hashPassword("synthetic-correct", 1)
				if err != nil {
					t.Fatal("prepare synthetic password")
				}
				for i := 0; i < 4; i++ {
					name := fmt.Sprintf("synthetic-%d", i)
					repo.users[name] = StoredUser{User: User{ID: i + 1, Nickname: name}, PasswordHash: hash}
				}
			}
			appClock := &clock.Fixed{Time: now}
			passwords := &observedPasswords{}
			service := NewService(repo).WithPasswordHashIterations(1).WithLoginLimits(LoginLimits{SourceAttempts: 3}, appClock)
			service.passwords = passwords
			for i := 0; i < 3; i++ {
				_, created, err := service.LoginFrom(context.Background(), "192.0.2.1", fmt.Sprintf("synthetic-%d", i), "synthetic-correct")
				if err != nil || created == registered {
					t.Fatalf("attempt %d did not complete", i)
				}
			}
			_, _, err := service.LoginFrom(context.Background(), "192.0.2.1", "synthetic-3", "synthetic-correct")
			assertLoginLimited(t, err, now.Add(5*time.Minute))
			if repo.lookups != 3 || passwords.hashes+passwords.verifications != 3 || (!registered && repo.creates != 3) {
				t.Fatal("source-limited request performed lookup, hashing or registration")
			}
			// A different source can still log in; successful logins did not reset the first source.
			if _, _, err := service.LoginFrom(context.Background(), "192.0.2.2", "synthetic-3", "synthetic-correct"); err != nil {
				t.Fatal("independent source was blocked")
			}
			appClock.Time = now.Add(5 * time.Minute)
			if _, _, err := service.LoginFrom(context.Background(), "192.0.2.1", "synthetic-0", "synthetic-correct"); err != nil {
				t.Fatal("source limit did not expire")
			}
		})
	}
}

func TestConcurrentAccountAttemptsCannotExceedBudget(t *testing.T) {
	hash, err := hashPassword("synthetic-correct", 1)
	if err != nil {
		t.Fatal("prepare synthetic password")
	}
	repo := &limitedLoginRepository{users: map[string]StoredUser{"account": {User: User{ID: 1, Nickname: "account"}, PasswordHash: hash}}}
	passwords := &observedPasswords{}
	service := NewService(repo).WithLoginLimits(LoginLimits{AccountAttempts: 5, MaxConcurrentHashes: 32}, nil)
	service.passwords = passwords
	start := make(chan struct{})
	results := make(chan error, 32)
	for i := 0; i < 32; i++ {
		go func(i int) {
			<-start
			_, _, err := service.LoginFrom(context.Background(), fmt.Sprintf("192.0.2.%d", i+1), "account", "synthetic-wrong")
			results <- err
		}(i)
	}
	close(start)
	unauthorized, limited := 0, 0
	for i := 0; i < 32; i++ {
		switch apperr.As(receiveLoginResult(t, results)).Status {
		case http.StatusUnauthorized:
			unauthorized++
		case http.StatusTooManyRequests:
			limited++
		default:
			t.Fatal("unexpected concurrent login result")
		}
	}
	if unauthorized != 5 || limited != 27 || passwords.verifications != 5 || repo.lookups != 5 {
		t.Fatalf("unauthorized=%d limited=%d verifications=%d lookups=%d", unauthorized, limited, passwords.verifications, repo.lookups)
	}
}

func TestLoginHashConcurrencyCoversEveryPasswordPath(t *testing.T) {
	for _, path := range []string{"verify", "register", "legacy"} {
		t.Run(path, func(t *testing.T) {
			repo := &limitedLoginRepository{users: map[string]StoredUser{}}
			hash, err := hashPassword("synthetic-correct", 1)
			if err != nil {
				t.Fatal("prepare synthetic password")
			}
			for i := 0; i < 13; i++ {
				if path == "register" {
					continue
				}
				name := fmt.Sprintf("synthetic-%d", i)
				user := StoredUser{User: User{ID: i + 1, Nickname: name}}
				if path == "verify" {
					user.PasswordHash = hash
				}
				repo.users[name] = user
			}
			entered, release := make(chan struct{}, 13), make(chan struct{})
			defer func() {
				select {
				case <-release:
				default:
					close(release)
				}
			}()
			passwords := &observedPasswords{entered: entered, release: release}
			service := NewService(repo).WithPasswordHashIterations(1).WithLoginLimits(LoginLimits{SourceAttempts: 100, AccountAttempts: 100, MaxConcurrentHashes: 2}, nil)
			service.passwords = passwords
			results := make(chan error, 12)
			for i := 0; i < 12; i++ {
				go func(i int) {
					_, _, err := service.LoginFrom(context.Background(), "192.0.2.1", fmt.Sprintf("synthetic-%d", i), "synthetic-correct")
					results <- err
				}(i)
			}
			for i := 0; i < 2; i++ {
				select {
				case <-entered:
				case <-time.After(5 * time.Second):
					t.Fatal("hash did not start")
				}
			}
			for i := 0; i < 10; i++ {
				assertLoginLimited(t, receiveLoginResult(t, results), time.Time{})
			}
			close(release)
			for i := 0; i < 2; i++ {
				if err := receiveLoginResult(t, results); err != nil {
					t.Fatal("admitted hash failed")
				}
			}
			if passwords.maxActive != 2 || passwords.hashes+passwords.verifications != 2 || repo.lookups != 2 {
				t.Fatalf("max hashes=%d total hashes=%d lookups=%d", passwords.maxActive, passwords.hashes+passwords.verifications, repo.lookups)
			}
			if _, _, err := service.LoginFrom(context.Background(), "192.0.2.1", "synthetic-12", "synthetic-correct"); err != nil {
				t.Fatal("hash slot was not released")
			}
		})
	}
}

func TestLoginReleasesHashSlotOnErrors(t *testing.T) {
	for _, failLookup := range []bool{false, true} {
		t.Run(fmt.Sprintf("lookupError=%v", failLookup), func(t *testing.T) {
			repo := &limitedLoginRepository{users: map[string]StoredUser{}}
			passwords := &observedPasswords{fail: true}
			if failLookup {
				repo.lookupErr = errors.New("synthetic repository failure")
			}
			service := NewService(repo).WithLoginLimits(LoginLimits{MaxConcurrentHashes: 1}, nil)
			service.passwords = passwords
			for i := 0; i < 2; i++ {
				_, _, err := service.LoginFrom(context.Background(), "192.0.2.1", "account", "synthetic-correct")
				if appErr := apperr.As(err); appErr == nil || appErr.Status != http.StatusInternalServerError {
					t.Fatal("failure did not release hash slot")
				}
			}
		})
	}
}

func TestLoginTrackingCapacityCannotEvictActiveLimits(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	for _, dimension := range []string{"source", "account"} {
		t.Run(dimension, func(t *testing.T) {
			appClock := &clock.Fixed{Time: now}
			guard := newLoginGuard(LoginLimits{SourceAttempts: 10, AccountAttempts: 10, MaxEntries: 2}, appClock)
			for i := 0; i < 2; i++ {
				source, account := fmt.Sprintf("source-%d", i), "account"
				if dimension == "account" {
					source, account = "source", fmt.Sprintf("account-%d", i)
				}
				release, err := guard.begin(source, account)
				if err != nil {
					t.Fatal("could not fill tracking state")
				}
				release()
			}
			_, err := guard.begin("new-source", "new-account")
			assertLoginLimited(t, err, now.Add(5*time.Minute))
			if len(guard.sources) > 2 || len(guard.accounts) > 2 {
				t.Fatal("tracking storage exceeded capacity")
			}
			appClock.Time = now.Add(5 * time.Minute)
			release, err := guard.begin("new-source", "new-account")
			if err != nil {
				t.Fatal("expired tracking state did not recover")
			}
			release()
			if len(guard.sources) != 1 || len(guard.accounts) != 1 {
				t.Fatal("expired state was not cleaned up")
			}
		})
	}
}

func TestLoginRetryTimeIncludesBudgetsConsumedByRejection(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	for _, busy := range []bool{false, true} {
		t.Run(fmt.Sprintf("hashBusy=%v", busy), func(t *testing.T) {
			appClock := &clock.Fixed{Time: now}
			guard := newLoginGuard(LoginLimits{SourceAttempts: 1, AccountAttempts: 1, MaxConcurrentHashes: 1}, appClock)
			release, err := guard.begin("first-source", "first-account")
			if err != nil {
				t.Fatal("first attempt was rejected")
			}
			defer release()
			appClock.Time = now.Add(time.Minute)
			account := "first-account"
			if busy {
				account = "second-account"
			}
			_, err = guard.begin("second-source", account)
			assertLoginLimited(t, err, now.Add(6*time.Minute))
		})
	}
}

func TestLoginRetryTimeIncludesCapacityWhenSourceIsAlreadyLimited(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	appClock := &clock.Fixed{Time: now}
	guard := newLoginGuard(LoginLimits{SourceAttempts: 2, AccountAttempts: 1, MaxEntries: 2}, appClock)
	admit := func(source, account string) {
		t.Helper()
		release, err := guard.begin(source, account)
		if err != nil {
			t.Fatal("could not prepare synthetic attempt windows")
		}
		release()
	}
	admit("first-source", "first-account")
	appClock.Time = now.Add(time.Minute)
	for i := 0; i < 2; i++ {
		if _, err := guard.begin("second-source", "first-account"); err == nil {
			t.Fatal("account limit was bypassed")
		}
	}
	appClock.Time = now.Add(5 * time.Minute)
	admit("third-source", "second-account")
	admit("third-source", "third-account")
	_, err := guard.begin("second-source", "new-account")
	assertLoginLimited(t, err, now.Add(10*time.Minute))
}

func assertLoginLimited(t *testing.T, err error, wantRetry time.Time) {
	t.Helper()
	var limited *RateLimitError
	if !errors.As(err, &limited) || limited.RetryAt.IsZero() || apperr.As(err).Status != http.StatusTooManyRequests || apperr.As(err).Code != apperr.CodeRateLimited {
		t.Fatal("expected a rate limit with a retry time")
	}
	if !wantRetry.IsZero() && !limited.RetryAt.Equal(wantRetry) {
		t.Fatalf("retry=%s want=%s", limited.RetryAt, wantRetry)
	}
}

func receiveLoginResult(t *testing.T, results <-chan error) error {
	t.Helper()
	select {
	case err := <-results:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("login waited instead of completing or rejecting")
		return nil
	}
}

// Synthetic repository and real, low-cost PBKDF2 wrapped with a blocking observer.
type limitedLoginRepository struct {
	mu        sync.Mutex
	users     map[string]StoredUser
	lookups   int
	creates   int
	lookupErr error
}

func (r *limitedLoginRepository) FindByNickname(_ context.Context, nickname string) (StoredUser, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lookups++
	if r.lookupErr != nil {
		return StoredUser{}, r.lookupErr
	}
	if user, ok := r.users[nickname]; ok {
		return user, nil
	}
	return StoredUser{}, ErrUserNotFound
}

func (r *limitedLoginRepository) FindByID(_ context.Context, _ int) (User, error) {
	return User{}, ErrUserNotFound
}

func (r *limitedLoginRepository) Create(_ context.Context, nickname, hash string) (User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.creates++
	user := User{ID: len(r.users) + 1, Nickname: nickname}
	r.users[nickname] = StoredUser{User: user, PasswordHash: hash}
	return user, nil
}

func (r *limitedLoginRepository) SetPasswordHash(_ context.Context, userID int, hash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, user := range r.users {
		if user.ID == userID {
			user.PasswordHash = hash
			r.users[name] = user
			return nil
		}
	}
	return ErrUserNotFound
}

type observedPasswords struct {
	mu            sync.Mutex
	hashes        int
	verifications int
	active        int
	maxActive     int
	entered       chan<- struct{}
	release       <-chan struct{}
	fail          bool
}

func (p *observedPasswords) observe(hash bool) func() {
	p.mu.Lock()
	if hash {
		p.hashes++
	} else {
		p.verifications++
	}
	p.active++
	if p.active > p.maxActive {
		p.maxActive = p.active
	}
	p.mu.Unlock()
	if p.entered != nil {
		p.entered <- struct{}{}
		<-p.release
	}
	return func() {
		p.mu.Lock()
		p.active--
		p.mu.Unlock()
	}
}

func (p *observedPasswords) hash(password string, iterations int) (string, error) {
	defer p.observe(true)()
	if p.fail {
		return "", errors.New("synthetic hash failure")
	}
	return hashPassword(password, iterations)
}

func (p *observedPasswords) verify(password, stored string) (bool, error) {
	defer p.observe(false)()
	return VerifyPassword(password, stored)
}
