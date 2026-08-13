package httpapi

import (
	"net/http"

	authhttp "github.com/jPurin-gg/myfitlog-backend/internal/auth/httpapi"
	"github.com/jPurin-gg/myfitlog-backend/internal/httpx"
	"github.com/jPurin-gg/myfitlog-backend/internal/profile"
)

type Handler struct{ service *profile.Service }

func NewHandler(service *profile.Service) *Handler { return &Handler{service: service} }

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	preferences, err := h.service.Get(r.Context(), authhttp.UserID(r.Context()))
	httpx.Respond(w, r, http.StatusOK, preferences, err)
}

func (h *Handler) Put(w http.ResponseWriter, r *http.Request) {
	httpx.DecodeAndRespond(w, r, http.StatusOK, func(request profile.Preferences) (profile.Preferences, error) {
		return h.service.Save(r.Context(), authhttp.UserID(r.Context()), request)
	})
}
