package httpapi

import (
	"net/http"
	"strconv"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	authhttp "github.com/jPurin-gg/myfitlog-backend/internal/auth/httpapi"
	"github.com/jPurin-gg/myfitlog-backend/internal/body"
	"github.com/jPurin-gg/myfitlog-backend/internal/httpx"
)

type Handler struct{ service *body.Service }

func NewHandler(service *body.Service) *Handler { return &Handler{service: service} }

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	measurements, err := h.service.List(r.Context(), authhttp.UserID(r.Context()))
	httpx.Respond(w, r, http.StatusOK, measurements, err)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	httpx.DecodeAndRespond(w, r, http.StatusCreated, func(request body.MeasurementInput) (body.Measurement, error) {
		return h.service.Create(r.Context(), authhttp.UserID(r.Context()), request)
	})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("measurementID"))
	if err != nil {
		httpx.WriteError(w, r, apperr.Validation("記録の指定が不正です。", nil))
		return
	}
	httpx.DecodeAndRespond(w, r, http.StatusOK, func(request body.MeasurementInput) (body.Measurement, error) {
		return h.service.Update(r.Context(), authhttp.UserID(r.Context()), id, request)
	})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("measurementID"))
	if err != nil {
		httpx.WriteError(w, r, apperr.Validation("記録の指定が不正です。", nil))
		return
	}
	httpx.RespondNoContent(w, r, h.service.Delete(r.Context(), authhttp.UserID(r.Context()), id))
}
