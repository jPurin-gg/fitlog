package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("secret-password")
	if err != nil || !strings.HasPrefix(hash, "pbkdf2_sha256$120000$") {
		t.Fatalf("HashPassword() = %q, %v; want the production iteration count", hash, err)
	}
	hash, err = hashPassword("secret-password", 1)
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}
	valid, err := VerifyPassword("secret-password", hash)
	if err != nil || !valid {
		t.Fatalf("VerifyPassword() = %v, %v; want true, nil", valid, err)
	}
	valid, err = VerifyPassword("wrong-password", hash)
	if err != nil || valid {
		t.Fatalf("VerifyPassword(wrong) = %v, %v; want false, nil", valid, err)
	}
}

// Expected values were cross-checked against Python's hashlib.pbkdf2_hmac("sha256", ...).
func TestPBKDF2SHA256MatchesKnownVectors(t *testing.T) {
	tests := []struct {
		name       string
		password   string
		salt       string
		iterations int
		keyLen     int
		want       string
	}{
		{name: "one iteration", password: "password", salt: "salt", iterations: 1, keyLen: 32, want: "120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b"},
		{name: "two iterations", password: "password", salt: "salt", iterations: 2, keyLen: 32, want: "ae4d0c95af6b46d32d0adff928f06dd02a303f8ef3c251dfd6e2d85a95474c43"},
		{name: "4096 iterations", password: "password", salt: "salt", iterations: 4096, keyLen: 32, want: "c5e478d59288c841aa530db6845c4c8d962893a001ce4e11a4963873aa98134a"},
		{name: "two output blocks", password: "passwordPASSWORDpassword", salt: "saltSALTsaltSALTsaltSALTsaltSALTsalt", iterations: 4096, keyLen: 40, want: "348c89dbcbd32b2f32d814b8116e84cf2b17347ebc1800181c4e2a1fb8dd53e1c635518c7dac47e9"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := hex.EncodeToString(pbkdf2SHA256([]byte(test.password), []byte(test.salt), test.iterations, test.keyLen)); got != test.want {
				t.Fatalf("pbkdf2SHA256() = %s, want %s", got, test.want)
			}
		})
	}
}

func TestTokenSignerAcceptsValidTokenUntilExpiry(t *testing.T) {
	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	secret := []byte("0123456789abcdef0123456789abcdef")
	signer := NewTokenSigner(secret, time.Hour)
	token, err := signer.Sign(42, now)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	userID, err := signer.Verify(token, now.Add(30*time.Minute))
	if err != nil || userID != 42 {
		t.Fatalf("Verify() = %d, %v; want 42, nil", userID, err)
	}
	userID, err = signer.Verify(token, now.Add(time.Hour-time.Second))
	if err != nil || userID != 42 {
		t.Fatalf("Verify(expiry - 1s) = %d, %v; want 42, nil", userID, err)
	}
	handSigned := signPayload(secret, fmt.Sprintf(`{"v":1,"uid":42,"exp":%d}`, now.Add(time.Hour).Unix()))
	userID, err = signer.Verify(handSigned, now)
	if err != nil || userID != 42 {
		t.Fatalf("Verify(hand-signed) = %d, %v; want 42, nil", userID, err)
	}
}

func TestTokenSignerVerifyRejectsForgedAndMalformedTokens(t *testing.T) {
	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	secret := []byte("0123456789abcdef0123456789abcdef")
	signer := NewTokenSigner(secret, time.Hour)
	token, err := signer.Sign(42, now)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		t.Fatalf("Sign() token parts = %d, want 2", len(parts))
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var payload tokenPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil || payload.UserID != 42 {
		t.Fatalf("payload = %#v, %v; want uid 42", payload, err)
	}
	payload.UserID = 7
	forgedPayload, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	forged := base64.RawURLEncoding.EncodeToString(forgedPayload) + "." + parts[1]

	otherSigner := NewTokenSigner([]byte("fedcba9876543210fedcba9876543210"), time.Hour)
	otherToken, err := otherSigner.Sign(42, now)
	if err != nil {
		t.Fatalf("other Sign() error = %v", err)
	}
	expires := now.Add(time.Hour).Unix()

	tests := []struct {
		name  string
		token string
		now   time.Time
	}{
		{name: "forged uid with original signature", token: forged, now: now},
		{name: "signed by different secret", token: otherToken, now: now},
		{name: "no dot", token: parts[0] + parts[1], now: now},
		{name: "three parts", token: token + "." + parts[1], now: now},
		{name: "empty payload", token: "." + parts[1], now: now},
		{name: "non-base64 signature", token: parts[0] + ".!!!", now: now},
		{name: "payload is not JSON", token: signPayload(secret, "not json"), now: now},
		{name: "unsupported version", token: signPayload(secret, fmt.Sprintf(`{"v":2,"uid":42,"exp":%d}`, expires)), now: now},
		{name: "zero uid", token: signPayload(secret, fmt.Sprintf(`{"v":1,"uid":0,"exp":%d}`, expires)), now: now},
		{name: "negative uid", token: signPayload(secret, fmt.Sprintf(`{"v":1,"uid":-1,"exp":%d}`, expires)), now: now},
		{name: "exactly at expiry", token: token, now: now.Add(time.Hour)},
		{name: "after expiry", token: token, now: now.Add(2 * time.Hour)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if userID, err := signer.Verify(test.token, test.now); err == nil {
				t.Fatalf("Verify(%q) = %d, nil; want error", test.token, userID)
			}
		})
	}
}

func TestLoginRegistersAndClaimsLegacyUser(t *testing.T) {
	repository := &memoryRepository{users: map[string]StoredUser{}, nextID: 1}
	service := NewService(repository).WithPasswordHashIterations(1)

	created, wasCreated, err := service.Login(context.Background(), " Mitsuki ", "password")
	if err != nil || !wasCreated || created.Nickname != "Mitsuki" {
		t.Fatalf("first Login() = %#v, %v, %v", created, wasCreated, err)
	}
	loggedIn, wasCreated, err := service.Login(context.Background(), "Mitsuki", "password")
	if err != nil || wasCreated || loggedIn.ID != created.ID {
		t.Fatalf("second Login() = %#v, %v, %v", loggedIn, wasCreated, err)
	}
	_, _, err = service.Login(context.Background(), "Mitsuki", "wrong")
	if appErr := apperr.As(err); appErr.Status != http.StatusUnauthorized || appErr.Code != apperr.CodeUnauthenticated {
		t.Fatalf("Login(wrong password) error = %#v", appErr)
	}

	repository.users["Legacy"] = StoredUser{User: User{ID: 99, Nickname: "Legacy"}}
	legacy, wasCreated, err := service.Login(context.Background(), "Legacy", "claimed-password")
	if err != nil || wasCreated || legacy.ID != 99 || repository.users["Legacy"].PasswordHash == "" {
		t.Fatalf("legacy Login() = %#v, %v, %v", legacy, wasCreated, err)
	}
}

func TestLoginValidatesNicknameAndPassword(t *testing.T) {
	tests := []struct {
		name     string
		nickname string
		password string
		field    string
	}{
		{name: "empty nickname", nickname: "", password: "password", field: "nickname"},
		{name: "empty password", nickname: "Mitsuki", password: "", field: "password"},
		{name: "whitespace-only nickname", nickname: " \t\n ", password: "password", field: "nickname"},
		{name: "81 rune nickname", nickname: strings.Repeat("あ", 81), password: "password", field: "nickname"},
		{name: "257 byte password", nickname: "Mitsuki", password: strings.Repeat("p", 257), field: "password"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &memoryRepository{users: map[string]StoredUser{}, nextID: 1}
			_, _, err := NewService(repository).Login(context.Background(), test.nickname, test.password)
			appErr := apperr.As(err)
			if appErr.Status != http.StatusBadRequest || appErr.Code != apperr.CodeValidation || appErr.Fields[test.field] == "" || len(repository.users) != 0 {
				t.Fatalf("Login() error = %#v; users = %d", appErr, len(repository.users))
			}
		})
	}
}

func TestLoginCountsNicknameLengthInRunes(t *testing.T) {
	repository := &memoryRepository{users: map[string]StoredUser{}, nextID: 1}
	nickname := strings.Repeat("あ", 80)
	user, wasCreated, err := NewService(repository).WithPasswordHashIterations(1).Login(context.Background(), nickname, "password")
	if err != nil || !wasCreated || user.Nickname != nickname {
		t.Fatalf("Login(80 runes) = %#v, %v, %v", user, wasCreated, err)
	}
}

func TestLoginReportsConflictWhenNicknameIsTakenDuringRegistration(t *testing.T) {
	repository := &memoryRepository{
		users:            map[string]StoredUser{"Mitsuki": {User: User{ID: 1, Nickname: "Mitsuki"}, PasswordHash: "taken"}},
		nextID:           2,
		notFoundOnLookup: "Mitsuki",
	}
	_, _, err := NewService(repository).WithPasswordHashIterations(1).Login(context.Background(), "Mitsuki", "password")
	if appErr := apperr.As(err); appErr.Status != http.StatusConflict || appErr.Code != apperr.CodeConflict {
		t.Fatalf("Login(raced nickname) error = %#v", appErr)
	}
}

func TestMeReturnsUserOrUnauthenticated(t *testing.T) {
	repository := &memoryRepository{users: map[string]StoredUser{"Mitsuki": {User: User{ID: 1, Nickname: "Mitsuki"}}}, nextID: 2}
	service := NewService(repository)
	user, err := service.Me(context.Background(), 1)
	if err != nil || user != (User{ID: 1, Nickname: "Mitsuki"}) {
		t.Fatalf("Me(1) = %#v, %v", user, err)
	}
	_, err = service.Me(context.Background(), 2)
	if appErr := apperr.As(err); appErr.Status != http.StatusUnauthorized || appErr.Code != apperr.CodeUnauthenticated {
		t.Fatalf("Me(unknown) error = %#v", appErr)
	}
}

// signPayload mirrors TokenSigner.Sign so tests can sign payloads Sign itself would never produce.
func signPayload(secret []byte, payload string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(payload))
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

type memoryRepository struct {
	users            map[string]StoredUser
	nextID           int
	notFoundOnLookup string
}

func (r *memoryRepository) FindByNickname(_ context.Context, nickname string) (StoredUser, error) {
	user, ok := r.users[nickname]
	if !ok || nickname == r.notFoundOnLookup {
		return StoredUser{}, ErrUserNotFound
	}
	return user, nil
}

func (r *memoryRepository) FindByID(_ context.Context, userID int) (User, error) {
	for _, user := range r.users {
		if user.ID == userID {
			return user.User, nil
		}
	}
	return User{}, ErrUserNotFound
}

func (r *memoryRepository) Create(_ context.Context, nickname, passwordHash string) (User, error) {
	if _, exists := r.users[nickname]; exists {
		return User{}, ErrNicknameTaken
	}
	user := User{ID: r.nextID, Nickname: nickname}
	r.nextID++
	r.users[nickname] = StoredUser{User: user, PasswordHash: passwordHash}
	return user, nil
}

func (r *memoryRepository) SetPasswordHash(_ context.Context, userID int, passwordHash string) error {
	for nickname, user := range r.users {
		if user.ID == userID {
			user.PasswordHash = passwordHash
			r.users[nickname] = user
			return nil
		}
	}
	return ErrUserNotFound
}
