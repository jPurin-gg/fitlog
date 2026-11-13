package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
)

const passwordHashIterations = 120000

var (
	ErrUserNotFound  = errors.New("user not found")
	ErrNicknameTaken = errors.New("nickname already exists")
)

type User struct {
	ID       int    `json:"id"`
	Nickname string `json:"nickname"`
}

type StoredUser struct {
	User
	PasswordHash string
}

type Repository interface {
	FindByNickname(ctx context.Context, nickname string) (StoredUser, error)
	// FindSimilarNicknames returns the users whose nickname equals the given one once letter case and
	// character width (full-width/half-width) are ignored, exact matches included. Callers only tell
	// none, one and several apart, so implementations may stop after two.
	FindSimilarNicknames(ctx context.Context, nickname string) ([]StoredUser, error)
	FindByID(ctx context.Context, userID int) (User, error)
	Create(ctx context.Context, nickname, passwordHash string) (User, error)
	SetPasswordHash(ctx context.Context, userID int, passwordHash string) error
}

type Service struct {
	repository     Repository
	hashIterations int
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, hashIterations: passwordHashIterations}
}

// WithPasswordHashIterations lowers the PBKDF2 cost for tests; at the default every login takes ~0.6s under -race.
func (s *Service) WithPasswordHashIterations(iterations int) *Service {
	s.hashIterations = iterations
	return s
}

// Login authenticates an existing user. It never creates one: an unknown nickname gets the same
// answer as a wrong password, so a mistyped nickname cannot silently become a new, empty account.
func (s *Service) Login(ctx context.Context, nickname, password string) (User, error) {
	nickname, err := validateCredentials(nickname, password)
	if err != nil {
		return User{}, err
	}

	stored, err := s.findForLogin(ctx, nickname)
	if errors.Is(err, ErrUserNotFound) {
		return User{}, invalidCredentials()
	}
	if err != nil {
		return User{}, apperr.Internal(err)
	}
	if stored.PasswordHash == "" {
		hash, hashErr := hashPassword(password, s.hashIterations)
		if hashErr != nil {
			return User{}, apperr.Internal(hashErr)
		}
		if updateErr := s.repository.SetPasswordHash(ctx, stored.ID, hash); updateErr != nil {
			return User{}, apperr.Internal(updateErr)
		}
		return stored.User, nil
	}
	valid, verifyErr := VerifyPassword(password, stored.PasswordHash)
	if verifyErr != nil {
		return User{}, apperr.Internal(verifyErr)
	}
	if !valid {
		return User{}, invalidCredentials()
	}
	return stored.User, nil
}

// findForLogin prefers the exact nickname, so accounts that already differ only by case or width
// stay reachable. Otherwise it accepts the one account whose nickname differs only by letter case
// or character width; several such accounts are ambiguous and count as unknown.
func (s *Service) findForLogin(ctx context.Context, nickname string) (StoredUser, error) {
	stored, err := s.repository.FindByNickname(ctx, nickname)
	if !errors.Is(err, ErrUserNotFound) {
		return stored, err
	}
	similar, err := s.repository.FindSimilarNicknames(ctx, nickname)
	if err != nil {
		return StoredUser{}, err
	}
	if len(similar) != 1 {
		return StoredUser{}, ErrUserNotFound
	}
	return similar[0], nil
}

// Register creates a new user. A nickname that is already in use is a conflict, even when the
// password matches: signing in to an existing account is Login's job. A nickname that differs from
// an existing one only by letter case or character width is also taken, so a phone keyboard that
// capitalizes or widens the input cannot fork a second account.
func (s *Service) Register(ctx context.Context, nickname, password string) (User, error) {
	nickname, err := validateCredentials(nickname, password)
	if err != nil {
		return User{}, err
	}

	similar, err := s.repository.FindSimilarNicknames(ctx, nickname)
	if err != nil {
		return User{}, apperr.Internal(err)
	}
	if len(similar) > 0 {
		return User{}, nicknameTaken()
	}

	hash, err := hashPassword(password, s.hashIterations)
	if err != nil {
		return User{}, apperr.Internal(err)
	}
	user, err := s.repository.Create(ctx, nickname, hash)
	if errors.Is(err, ErrNicknameTaken) {
		return User{}, nicknameTaken()
	}
	if err != nil {
		return User{}, apperr.Internal(err)
	}
	return user, nil
}

// validateCredentials returns the trimmed nickname shared by Login and Register.
func validateCredentials(nickname, password string) (string, error) {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" || password == "" {
		return "", apperr.Validation("ニックネームとパスワードを入力してください。", map[string]string{
			"nickname": "required",
			"password": "required",
		})
	}
	fields := map[string]string{}
	if len([]rune(nickname)) > 80 {
		fields["nickname"] = "must be at most 80 characters"
	}
	if len(password) > 256 {
		fields["password"] = "must be at most 256 bytes"
	}
	if len(fields) > 0 {
		return "", apperr.Validation("ニックネームまたはパスワードが長すぎます。", fields)
	}
	return nickname, nil
}

func invalidCredentials() error {
	return apperr.Unauthenticated("ニックネームまたはパスワードが違います。")
}

func nicknameTaken() error {
	return apperr.Conflict("そのニックネームは既に使われています（大文字小文字・全角半角の違いは区別しません）。ログインするか、別のニックネームを選んでください。")
}

func (s *Service) Me(ctx context.Context, userID int) (User, error) {
	user, err := s.repository.FindByID(ctx, userID)
	if errors.Is(err, ErrUserNotFound) {
		return User{}, apperr.Unauthenticated("セッションのユーザーが存在しません。")
	}
	if err != nil {
		return User{}, apperr.Internal(err)
	}
	return user, nil
}

type TokenSigner struct {
	secret []byte
	ttl    time.Duration
}

type tokenPayload struct {
	Version int   `json:"v"`
	UserID  int   `json:"uid"`
	Expires int64 `json:"exp"`
}

func NewTokenSigner(secret []byte, ttl time.Duration) *TokenSigner {
	return &TokenSigner{secret: append([]byte(nil), secret...), ttl: ttl}
}

func (s *TokenSigner) Sign(userID int, now time.Time) (string, error) {
	payload, err := json.Marshal(tokenPayload{Version: 1, UserID: userID, Expires: now.Add(s.ttl).Unix()})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encoded + "." + signature, nil
}

func (s *TokenSigner) Verify(token string, now time.Time) (int, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return 0, errors.New("invalid token format")
	}
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(parts[0]))
	expected := mac.Sum(nil)
	actual, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(actual, expected) {
		return 0, errors.New("invalid token signature")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, errors.New("invalid token payload")
	}
	var payload tokenPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil || payload.Version != 1 || payload.UserID <= 0 {
		return 0, errors.New("invalid token payload")
	}
	if now.Unix() >= payload.Expires {
		return 0, errors.New("token expired")
	}
	return payload.UserID, nil
}

func HashPassword(password string) (string, error) {
	return hashPassword(password, passwordHashIterations)
}

func hashPassword(password string, iterations int) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := pbkdf2SHA256([]byte(password), salt, iterations, 32)
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", iterations, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func VerifyPassword(password, stored string) (bool, error) {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false, nil
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 || iterations > 1_000_000 {
		return false, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false, err
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false, err
	}
	if len(salt) < 8 || len(salt) > 64 || len(expected) < 16 || len(expected) > 64 {
		return false, nil
	}
	actual := pbkdf2SHA256([]byte(password), salt, iterations, len(expected))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func pbkdf2SHA256(password, salt []byte, iterations, keyLen int) []byte {
	hashLen := sha256.Size
	blocks := (keyLen + hashLen - 1) / hashLen
	output := make([]byte, 0, blocks*hashLen)
	for block := 1; block <= blocks; block++ {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for iteration := 1; iteration < iterations; iteration++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for index := range t {
				t[index] ^= u[index]
			}
		}
		output = append(output, t...)
	}
	return output[:keyLen]
}
