package httpapi

import (
	"net/http"
	"strconv"

	"github.com/jPurin-gg/myfitlog-backend/internal/apperr"
	authhttp "github.com/jPurin-gg/myfitlog-backend/internal/auth/httpapi"
	"github.com/jPurin-gg/myfitlog-backend/internal/httpx"
	"github.com/jPurin-gg/myfitlog-backend/internal/workout"
)

type Handler struct{ service *workout.Service }

func NewHandler(service *workout.Service) *Handler { return &Handler{service: service} }

func (h *Handler) RecordSet(w http.ResponseWriter, r *http.Request) {
	workoutID, err := positivePathID(r, "workoutID")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var request workout.SetInput
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	result, replayed, err := h.service.RecordSet(r.Context(), authhttp.UserID(r.Context()), workoutID, r.Header.Get("Idempotency-Key"), request)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	httpx.WriteJSON(w, status, result)
}

func (h *Handler) Recommendation(w http.ResponseWriter, r *http.Request) {
	workoutID, err := positivePathID(r, "workoutID")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	setID, err := positivePathID(r, "setID")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	result, err := h.service.Recommend(r.Context(), authhttp.UserID(r.Context()), workoutID, setID)
	httpx.Respond(w, r, http.StatusOK, result, err)
}

func (h *Handler) Finish(w http.ResponseWriter, r *http.Request) {
	workoutID, err := positivePathID(r, "workoutID")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	result, err := h.service.Finish(r.Context(), authhttp.UserID(r.Context()), workoutID)
	httpx.Respond(w, r, http.StatusOK, result, err)
}

func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	workoutID, err := positivePathID(r, "workoutID")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	result, err := h.service.Detail(r.Context(), authhttp.UserID(r.Context()), workoutID)
	httpx.Respond(w, r, http.StatusOK, result, err)
}

func (h *Handler) CalendarWorkout(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.CalendarWorkout(r.Context(), authhttp.UserID(r.Context()), r.PathValue("date"))
	httpx.Respond(w, r, http.StatusOK, result, err)
}

func (h *Handler) PutCalendarWorkout(w http.ResponseWriter, r *http.Request) {
	httpx.DecodeAndRespond(w, r, http.StatusOK, func(request workout.CalendarWorkoutInput) (workout.CalendarWorkout, error) {
		return h.service.SaveCalendarWorkout(r.Context(), authhttp.UserID(r.Context()), r.PathValue("date"), request)
	})
}

func positivePathID(r *http.Request, name string) (int, error) {
	value, err := strconv.Atoi(r.PathValue(name))
	if err != nil || value <= 0 {
		return 0, apperr.Validation("IDは正の整数で指定してください。", map[string]string{name: "must be positive"})
	}
	return value, nil
}
