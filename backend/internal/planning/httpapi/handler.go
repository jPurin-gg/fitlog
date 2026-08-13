package httpapi

import (
	"net/http"

	authhttp "github.com/jPurin-gg/myfitlog-backend/internal/auth/httpapi"
	"github.com/jPurin-gg/myfitlog-backend/internal/httpx"
	"github.com/jPurin-gg/myfitlog-backend/internal/planning"
)

type Handler struct{ service *planning.Service }

func NewHandler(service *planning.Service) *Handler { return &Handler{service: service} }

func (h *Handler) MonthlyList(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.MonthlyList(r.Context(), authhttp.UserID(r.Context()))
	httpx.Respond(w, r, http.StatusOK, result, err)
}

func (h *Handler) Monthly(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Monthly(r.Context(), authhttp.UserID(r.Context()), r.PathValue("month"))
	httpx.Respond(w, r, http.StatusOK, result, err)
}

func (h *Handler) PutMonthly(w http.ResponseWriter, r *http.Request) {
	httpx.DecodeAndRespond(w, r, http.StatusOK, func(request planning.MonthlyPlanInput) (planning.MonthlyPlan, error) {
		return h.service.SaveMonthly(r.Context(), authhttp.UserID(r.Context()), r.PathValue("month"), request)
	})
}

func (h *Handler) GenerateMonthly(w http.ResponseWriter, r *http.Request) {
	httpx.DecodeAndRespond(w, r, http.StatusOK, func(request planning.GenerateMonthlyInput) (planning.MonthlyPlan, error) {
		return h.service.GenerateMonthly(r.Context(), authhttp.UserID(r.Context()), r.PathValue("month"), request)
	})
}

func (h *Handler) Daily(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Daily(r.Context(), authhttp.UserID(r.Context()), r.PathValue("date"))
	httpx.Respond(w, r, http.StatusOK, result, err)
}

func (h *Handler) PutDaily(w http.ResponseWriter, r *http.Request) {
	httpx.DecodeAndRespond(w, r, http.StatusOK, func(request planning.WorkoutPlan) (planning.PlanSession, error) {
		return h.service.SaveDaily(r.Context(), authhttp.UserID(r.Context()), r.PathValue("date"), request)
	})
}

func (h *Handler) StartDaily(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.StartDaily(r.Context(), authhttp.UserID(r.Context()), r.PathValue("date"))
	httpx.Respond(w, r, http.StatusOK, result, err)
}
