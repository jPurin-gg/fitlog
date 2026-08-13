package httpapi

import (
	"net/http"
	"strconv"

	authhttp "github.com/jPurin-gg/myfitlog-backend/internal/auth/httpapi"
	"github.com/jPurin-gg/myfitlog-backend/internal/httpx"
	"github.com/jPurin-gg/myfitlog-backend/internal/reporting"
)

type Handler struct{ service *reporting.Service }

func NewHandler(service *reporting.Service) *Handler { return &Handler{service: service} }

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Dashboard(r.Context(), authhttp.UserID(r.Context()))
	httpx.Respond(w, r, http.StatusOK, result, err)
}

func (h *Handler) Calendar(w http.ResponseWriter, r *http.Request) {
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	month, _ := strconv.Atoi(r.URL.Query().Get("month"))
	result, err := h.service.Calendar(r.Context(), authhttp.UserID(r.Context()), year, month)
	httpx.Respond(w, r, http.StatusOK, result, err)
}
