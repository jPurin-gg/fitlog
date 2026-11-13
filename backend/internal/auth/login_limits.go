package auth

import (
	"crypto/sha256"
	"net/http"
	"sync"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
)

// LoginLimits apply to all attempts, including successful logins and registrations.
// State and hash slots belong to one Service (one application process).
type LoginLimits struct {
	SourceAttempts      int
	AccountAttempts     int
	Window              time.Duration
	MaxConcurrentHashes int
	MaxEntries          int // Maximum entries in each of the source and account maps.
}

func DefaultLoginLimits() LoginLimits {
	return LoginLimits{SourceAttempts: 20, AccountAttempts: 5, Window: 5 * time.Minute, MaxConcurrentHashes: 2, MaxEntries: 10000}
}

// RateLimitError contains no request data. RetryAt is a retry time, not a reservation.
type RateLimitError struct {
	RetryAt time.Time
}

func (e *RateLimitError) Error() string { return apperr.CodeRateLimited }

func (e *RateLimitError) Unwrap() error {
	return apperr.New(http.StatusTooManyRequests, apperr.CodeRateLimited, "ログインの試行上限に達しました。時間をおいて再試行してください。")
}

type attemptWindow struct {
	attempts int
	expires  time.Time
}

type loginGuard struct {
	mu            sync.Mutex
	limits        LoginLimits
	clock         clock.Clock
	sources       map[[sha256.Size]byte]attemptWindow
	accounts      map[[sha256.Size]byte]attemptWindow
	nextExpiry    time.Time
	sourceExpiry  time.Time
	accountExpiry time.Time
	hashSlots     chan struct{}
}

func newLoginGuard(limits LoginLimits, appClock clock.Clock) *loginGuard {
	defaults := DefaultLoginLimits()
	if limits.SourceAttempts <= 0 {
		limits.SourceAttempts = defaults.SourceAttempts
	}
	if limits.AccountAttempts <= 0 {
		limits.AccountAttempts = defaults.AccountAttempts
	}
	if limits.Window <= 0 {
		limits.Window = defaults.Window
	}
	if limits.MaxConcurrentHashes <= 0 {
		limits.MaxConcurrentHashes = defaults.MaxConcurrentHashes
	}
	if limits.MaxEntries <= 0 {
		limits.MaxEntries = defaults.MaxEntries
	}
	if appClock == nil {
		appClock = clock.NewSystem(time.UTC)
	}
	return &loginGuard{
		limits: limits, clock: appClock,
		sources: make(map[[sha256.Size]byte]attemptWindow), accounts: make(map[[sha256.Size]byte]attemptWindow),
		hashSlots: make(chan struct{}, limits.MaxConcurrentHashes),
	}
}

// WithLoginLimits configures the service before it starts serving requests.
func (s *Service) WithLoginLimits(limits LoginLimits, appClock clock.Clock) *Service {
	s.loginGuard = newLoginGuard(limits, appClock)
	return s
}

func (g *loginGuard) begin(source, account string) (func(), error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.clock.Now()
	g.expire(now)
	sourceKey, accountKey := sha256.Sum256([]byte(source)), sha256.Sum256([]byte(account))
	sourceWindow, sourceExists := g.sources[sourceKey]
	accountWindow, accountExists := g.accounts[accountKey]
	if sourceWindow.attempts >= g.limits.SourceAttempts {
		return nil, g.limited(sourceWindow.expires, sourceWindow, accountWindow)
	}
	if !sourceExists && len(g.sources) >= g.limits.MaxEntries {
		return nil, g.limited(g.sourceExpiry, sourceWindow, accountWindow)
	}
	sourceWindow = g.count(sourceWindow, now)
	g.sources[sourceKey] = sourceWindow
	if g.sourceExpiry.IsZero() || sourceWindow.expires.Before(g.sourceExpiry) {
		g.sourceExpiry = sourceWindow.expires
	}
	// Account rejections also consume the source budget to bound account scanning.
	if accountWindow.attempts >= g.limits.AccountAttempts {
		return nil, g.limited(accountWindow.expires, sourceWindow, accountWindow)
	}
	if !accountExists && len(g.accounts) >= g.limits.MaxEntries {
		return nil, g.limited(g.accountExpiry, sourceWindow, accountWindow)
	}
	accountWindow = g.count(accountWindow, now)
	g.accounts[accountKey] = accountWindow
	if g.accountExpiry.IsZero() || accountWindow.expires.Before(g.accountExpiry) {
		g.accountExpiry = accountWindow.expires
	}

	// Fail immediately instead of accumulating a queue of expensive work. Keep the
	// slot through the entire login, including repository work, so every hashing
	// path (verification, registration and legacy password setup) is covered.
	select {
	case g.hashSlots <- struct{}{}:
		return func() { <-g.hashSlots }, nil
	default:
		return nil, g.limited(now.Add(time.Second), sourceWindow, accountWindow)
	}
}

func (g *loginGuard) limited(retryAt time.Time, source, account attemptWindow) error {
	if source.attempts >= g.limits.SourceAttempts && source.expires.After(retryAt) {
		retryAt = source.expires
	}
	if account.attempts >= g.limits.AccountAttempts && account.expires.After(retryAt) {
		retryAt = account.expires
	}
	if source.attempts == 0 && len(g.sources) >= g.limits.MaxEntries && g.sourceExpiry.After(retryAt) {
		retryAt = g.sourceExpiry
	}
	if account.attempts == 0 && len(g.accounts) >= g.limits.MaxEntries && g.accountExpiry.After(retryAt) {
		retryAt = g.accountExpiry
	}
	return &RateLimitError{RetryAt: retryAt}
}

func (g *loginGuard) count(window attemptWindow, now time.Time) attemptWindow {
	if window.attempts == 0 {
		window.expires = now.Add(g.limits.Window)
		if g.nextExpiry.IsZero() || window.expires.Before(g.nextExpiry) {
			g.nextExpiry = window.expires
		}
	}
	window.attempts++
	return window
}

// Active entries are never evicted to admit a new key: that would bypass limits.
// Scan only at the earliest expiry, keeping storage bounded without a goroutine.
func (g *loginGuard) expire(now time.Time) {
	if g.nextExpiry.IsZero() || now.Before(g.nextExpiry) {
		return
	}
	g.nextExpiry = time.Time{}
	for _, state := range []struct {
		entries map[[sha256.Size]byte]attemptWindow
		expiry  *time.Time
	}{{g.sources, &g.sourceExpiry}, {g.accounts, &g.accountExpiry}} {
		*state.expiry = time.Time{}
		for key, window := range state.entries {
			if !now.Before(window.expires) {
				delete(state.entries, key)
			} else {
				if state.expiry.IsZero() || window.expires.Before(*state.expiry) {
					*state.expiry = window.expires
				}
				if g.nextExpiry.IsZero() || window.expires.Before(g.nextExpiry) {
					g.nextExpiry = window.expires
				}
			}
		}
	}
}
