package httpapi

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
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
	query := r.URL.Query()
	limit, err := nonNegativeQueryInt(query, "limit")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	offset, err := nonNegativeQueryInt(query, "offset")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	filters := exercise.Filters{
		Name:      strings.TrimSpace(query.Get("name")),
		Muscles:   splitCSV(query.Get("muscle")),
		Equipment: splitCSV(query.Get("equipment")),
		Level:     strings.TrimSpace(query.Get("level")),
		Limit:     limit,
		Offset:    offset,
	}
	result, err := h.service.Search(r.Context(), filters)
	httpx.Respond(w, r, http.StatusOK, result, err)
}

func (h *Handler) FilterOptions(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, h.service.FilterOptions())
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Get(r.Context(), r.PathValue("exerciseID"))
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

// nonNegativeQueryInt returns 0 for an absent parameter so that the service
// can apply its default.
func nonNegativeQueryInt(query url.Values, name string) (int, error) {
	raw := strings.TrimSpace(query.Get(name))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, apperr.Validation("limitとoffsetは0以上の整数で指定してください。", map[string]string{name: "must be a non-negative integer"})
	}
	return value, nil
}
