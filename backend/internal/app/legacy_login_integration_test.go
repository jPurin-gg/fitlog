//go:build integration

package app_test

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	"github.com/jPurin-gg/myfitlog-backend/internal/auth"
	authpostgres "github.com/jPurin-gg/myfitlog-backend/internal/auth/postgres"
)

func TestConcurrentLegacyLogins(t *testing.T) {
	for _, legacy := range []struct {
		name string
		hash any
	}{
		{name: "empty hash", hash: ""},
		{name: "null hash", hash: nil},
	} {
		for _, samePassword := range []bool{false, true} {
			name := "different passwords"
			if samePassword {
				name = "same password"
			}
			t.Run(legacy.name+"/"+name, func(t *testing.T) {
				f := newManualAPIFixture(t, nil)
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				var userID int
				if err := f.db.QueryRowContext(ctx, `INSERT INTO users (username, password_hash) VALUES ($1, $2) RETURNING id`, "legacy-user", legacy.hash).Scan(&userID); err != nil {
					t.Fatal(err)
				}
				repository := authpostgres.New(f.db)
				barrier := &legacyLoginBarrierRepository{Repository: repository, bothRead: make(chan struct{})}
				service := auth.NewService(barrier).WithPasswordHashIterations(1)
				passwords := []string{"first-test-password", "second-test-password"}
				if samePassword {
					passwords[1] = passwords[0]
				}
				type result struct {
					index   int
					user    auth.User
					created bool
					err     error
				}
				results := make(chan result, len(passwords))
				for index, password := range passwords {
					go func() {
						user, created, err := service.Login(ctx, "legacy-user", password)
						results <- result{index: index, user: user, created: created, err: err}
					}()
				}
				succeeded := make([]bool, len(passwords))
				successes := 0
				for range passwords {
					select {
					case got := <-results:
						if got.err == nil {
							if got.created || got.user.ID != userID {
								t.Fatalf("legacy login created=%v user ID=%d, want false/%d", got.created, got.user.ID, userID)
							}
							succeeded[got.index] = true
							successes++
						} else if appErr := apperr.As(got.err); appErr.Status != http.StatusUnauthorized || appErr.Code != apperr.CodeUnauthenticated {
							t.Fatalf("concurrent login error = %v", got.err)
						}
					case <-ctx.Done():
						t.Fatal("concurrent legacy logins did not finish")
					}
				}
				wantSuccesses := 1
				if samePassword {
					wantSuccesses = 2
				}
				if successes != wantSuccesses {
					t.Fatalf("successful concurrent logins = %d, want %d", successes, wantSuccesses)
				}
				stored, err := repository.FindByNickname(ctx, "legacy-user")
				if err != nil || stored.PasswordHash == "" {
					t.Fatalf("read migrated user: error=%v, hash set=%v", err, stored.PasswordHash != "")
				}
				for index, password := range passwords {
					valid, err := auth.VerifyPassword(password, stored.PasswordHash)
					if err != nil || valid != succeeded[index] {
						t.Fatalf("saved hash matches login %d = %v, error=%v; want %v", index, valid, err, succeeded[index])
					}
					user, created, err := service.Login(ctx, "legacy-user", password)
					if succeeded[index] {
						if err != nil || created || user.ID != userID {
							t.Fatalf("repeat winning login: user ID=%d created=%v error=%v", user.ID, created, err)
						}
					} else if appErr := apperr.As(err); err == nil || appErr.Status != http.StatusUnauthorized || appErr.Code != apperr.CodeUnauthenticated {
						t.Fatalf("repeat losing login error = %v", err)
					}
				}
				after, err := repository.FindByNickname(ctx, "legacy-user")
				if err != nil || after.PasswordHash != stored.PasswordHash {
					t.Fatalf("repeat logins changed the saved hash; error=%v", err)
				}
			})
		}
	}
}

type legacyLoginBarrierRepository struct {
	auth.Repository
	reads    atomic.Int32
	bothRead chan struct{}
}

func (r *legacyLoginBarrierRepository) FindByNickname(ctx context.Context, nickname string) (auth.StoredUser, error) {
	stored, err := r.Repository.FindByNickname(ctx, nickname)
	if err != nil || stored.PasswordHash != "" {
		return stored, err
	}
	// Both requests must observe the legacy hash before either can migrate it.
	if r.reads.Add(1) == 2 {
		close(r.bothRead)
	}
	select {
	case <-r.bothRead:
		return stored, nil
	case <-ctx.Done():
		return auth.StoredUser{}, ctx.Err()
	}
}
