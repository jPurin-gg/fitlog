package auth

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
)

func TestConcurrentLegacyLogins(t *testing.T) {
	for _, test := range []struct {
		name          string
		passwords     [2]string
		wantSuccesses int
	}{
		{name: "different passwords", passwords: [2]string{"first-password", "second-password"}, wantSuccesses: 1},
		{name: "same password", passwords: [2]string{"same-password", "same-password"}, wantSuccesses: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			legacy := User{ID: 99, Nickname: "Legacy"}
			repository := &concurrentLegacyRepository{
				memoryRepository: &memoryRepository{users: map[string]StoredUser{"Legacy": {User: legacy}}},
				ready:            make(chan struct{}, 2),
				release:          make(chan struct{}),
			}
			service := NewService(repository).WithPasswordHashIterations(1)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			type loginResult struct {
				index   int
				user    User
				created bool
				err     error
			}
			results := make(chan loginResult, 2)
			for index, password := range test.passwords {
				go func() {
					user, created, err := service.Login(ctx, "Legacy", password)
					results <- loginResult{index: index, user: user, created: created, err: err}
				}()
			}
			// Both requests must read the empty hash before either may claim it.
			for range test.passwords {
				select {
				case <-repository.ready:
				case <-ctx.Done():
					t.Fatal("concurrent logins did not read the legacy user")
				}
			}
			close(repository.release)
			succeeded := [2]bool{}
			successes := 0
			for range test.passwords {
				var result loginResult
				select {
				case result = <-results:
				case <-ctx.Done():
					t.Fatal("concurrent logins did not complete")
				}
				if result.err == nil {
					if result.created || result.user != legacy {
						t.Fatalf("Login() returned user = %+v, created = %v", result.user, result.created)
					}
					succeeded[result.index] = true
					successes++
				} else if appErr := apperr.As(result.err); appErr.Status != http.StatusUnauthorized || appErr.Code != apperr.CodeUnauthenticated {
					t.Fatalf("Login() error = %v; want unauthenticated", result.err)
				}
			}
			if successes != test.wantSuccesses {
				t.Fatalf("successful logins = %d; want %d", successes, test.wantSuccesses)
			}
			for index, password := range test.passwords {
				user, created, err := service.Login(ctx, "Legacy", password)
				if succeeded[index] {
					if err != nil || created || user != legacy {
						t.Fatalf("subsequent winning Login() = %+v, %v, %v", user, created, err)
					}
				} else if err == nil || apperr.As(err).Status != http.StatusUnauthorized {
					t.Fatalf("subsequent losing Login() error = %v; want unauthenticated", err)
				}
			}
		})
	}
}

type concurrentLegacyRepository struct {
	*memoryRepository
	mu      sync.Mutex
	ready   chan struct{}
	release chan struct{}
}

func (r *concurrentLegacyRepository) FindByNickname(ctx context.Context, nickname string) (StoredUser, error) {
	r.mu.Lock()
	user, err := r.memoryRepository.FindByNickname(ctx, nickname)
	r.mu.Unlock()
	if err != nil || user.PasswordHash != "" {
		return user, err
	}
	select {
	case r.ready <- struct{}{}:
	case <-ctx.Done():
		return StoredUser{}, ctx.Err()
	}
	select {
	case <-r.release:
		return user, nil
	case <-ctx.Done():
		return StoredUser{}, ctx.Err()
	}
}

func (r *concurrentLegacyRepository) SetPasswordHashIfEmpty(ctx context.Context, userID int, passwordHash string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.memoryRepository.SetPasswordHashIfEmpty(ctx, userID, passwordHash)
}
