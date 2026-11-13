package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"time"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	"github.com/jPurin-gg/myfitlog-backend/internal/auth"
	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
	"github.com/jPurin-gg/myfitlog-backend/internal/httpx"
)

type contextKey string

const userIDKey contextKey = "auth_user_id"

type Handler struct {
	service        *auth.Service
	signer         *auth.TokenSigner
	clock          clock.Clock
	cookieName     string
	cookieSecure   bool
	cookieTTL      time.Duration
	trustedProxies []netip.Prefix
}

func NewHandler(service *auth.Service, signer *auth.TokenSigner, appClock clock.Clock, cookieName string, secure bool, ttl time.Duration) *Handler {
	return &Handler{service: service, signer: signer, clock: appClock, cookieName: cookieName, cookieSecure: secure, cookieTTL: ttl}
}

type loginRequest struct {
	Nickname string `json:"nickname"`
	Password string `json:"password"`
}

type loginResponse struct {
	ID       int    `json:"id"`
	Nickname string `json:"nickname"`
	Created  bool   `json:"created"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	user, created, err := h.service.LoginFrom(r.Context(), h.loginSource(r), request.Nickname, request.Password)
	if err != nil {
		var limited *auth.RateLimitError
		if errors.As(err, &limited) {
			// HTTP-date has whole-second precision; never advertise an earlier retry.
			retryAt := limited.RetryAt.UTC()
			if retryAt.Nanosecond() != 0 {
				retryAt = retryAt.Truncate(time.Second).Add(time.Second)
			}
			w.Header().Set("Retry-After", retryAt.Format(http.TimeFormat))
			w.Header().Set("Cache-Control", "no-store")
		}
		httpx.WriteError(w, r, err)
		return
	}
	token, err := h.signer.Sign(user.ID, h.clock.Now())
	if err != nil {
		httpx.WriteError(w, r, apperr.Internal(err))
		return
	}
	h.setCookie(w, token)
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	httpx.WriteJSON(w, status, loginResponse{ID: user.ID, Nickname: user.Nickname, Created: created})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, err := h.service.Me(r.Context(), UserID(r.Context()))
	httpx.Respond(w, r, http.StatusOK, user, err)
}

func (h *Handler) Logout(w http.ResponseWriter, _ *http.Request) {
	h.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(h.cookieName)
		if err != nil {
			httpx.WriteError(w, r, apperr.Unauthenticated("ログインが必要です。"))
			return
		}
		userID, err := h.signer.Verify(cookie.Value, h.clock.Now())
		if err != nil {
			h.clearCookie(w)
			httpx.WriteError(w, r, apperr.Unauthenticated("セッションが無効または期限切れです。"))
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserID(ctx context.Context) int {
	userID, _ := ctx.Value(userIDKey).(int)
	return userID
}

func (h *Handler) setCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(h.cookieTTL.Seconds()),
		Expires:  h.clock.Now().Add(h.cookieTTL),
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: h.cookieName, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: true, Secure: h.cookieSecure, SameSite: http.SameSiteLaxMode})
}
