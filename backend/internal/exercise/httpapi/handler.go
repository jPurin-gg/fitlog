package httpapi

import (
	"net/http"
	"strings"

	authhttp "github.com/jPurin-gg/myfitlog-backend/internal/auth/httpapi"
	"github.com/jPurin-gg/myfitlog-backend/internal/exercise"
	"github.com/jPurin-gg/myfitlog-backend/internal/httpx"
)

type Handler struct{ service *exercise.Service }

type alternativesRequest struct {
	Reason string `json:"reason"`
}

func NewHandler(service *exercise.Service) *Handler { return &Handler{service: service} }

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	filters := exercise.Filters{
		Name:      strings.TrimSpace(r.URL.Query().Get("name")),
		Muscle:    strings.TrimSpace(r.URL.Query().Get("muscle")),
		Equipment: splitCSV(r.URL.Query().Get("equipment")),
		Level:     strings.TrimSpace(r.URL.Query().Get("level")),
	}
	result, err := h.service.Search(r.Context(), filters)
	httpx.Respond(w, r, http.StatusOK, result, err)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	httpx.DecodeAndRespond(w, r, http.StatusCreated, func(request exercise.CreateInput) (exercise.Exercise, error) {
		return h.service.Create(r.Context(), request)
	})
}

func (h *Handler) Favorites(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Favorites(r.Context(), authhttp.UserID(r.Context()))
	httpx.Respond(w, r, http.StatusOK, result, err)
}

func (h *Handler) PutFavorite(w http.ResponseWriter, r *http.Request) {
	httpx.RespondNoContent(w, r, h.service.SetFavorite(r.Context(), authhttp.UserID(r.Context()), r.PathValue("exerciseID"), true))
}

func (h *Handler) DeleteFavorite(w http.ResponseWriter, r *http.Request) {
	httpx.RespondNoContent(w, r, h.service.SetFavorite(r.Context(), authhttp.UserID(r.Context()), r.PathValue("exerciseID"), false))
}

func (h *Handler) Recent(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Recent(r.Context(), authhttp.UserID(r.Context()))
	httpx.Respond(w, r, http.StatusOK, result, err)
}

func (h *Handler) Settings(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Settings(r.Context(), authhttp.UserID(r.Context()), r.PathValue("exerciseID"))
	httpx.Respond(w, r, http.StatusOK, result, err)
}

func (h *Handler) PutSettings(w http.ResponseWriter, r *http.Request) {
	httpx.DecodeAndRespond(w, r, http.StatusOK, func(request exercise.Settings) (exercise.Settings, error) {
		return h.service.SaveSettings(r.Context(), authhttp.UserID(r.Context()), r.PathValue("exerciseID"), request)
	})
}

func (h *Handler) Alternatives(w http.ResponseWriter, r *http.Request) {
	httpx.DecodeAndRespond(w, r, http.StatusOK, func(request alternativesRequest) (exercise.AlternativeResponse, error) {
		return h.service.Alternatives(r.Context(), r.PathValue("exerciseID"), request.Reason)
	})
}

func splitCSV(value string) []string {
	result := []string{}
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}
