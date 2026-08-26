package app

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/jPurin-gg/myfitlog-backend/internal/ai/openaicompat"
	authcore "github.com/jPurin-gg/myfitlog-backend/internal/auth"
	authhttp "github.com/jPurin-gg/myfitlog-backend/internal/auth/httpapi"
	authpostgres "github.com/jPurin-gg/myfitlog-backend/internal/auth/postgres"
	"github.com/jPurin-gg/myfitlog-backend/internal/clock"
	"github.com/jPurin-gg/myfitlog-backend/internal/config"
	exercisecore "github.com/jPurin-gg/myfitlog-backend/internal/exercise"
	exercisehttp "github.com/jPurin-gg/myfitlog-backend/internal/exercise/httpapi"
	exercisepostgres "github.com/jPurin-gg/myfitlog-backend/internal/exercise/postgres"
	"github.com/jPurin-gg/myfitlog-backend/internal/httpx"
	planningcore "github.com/jPurin-gg/myfitlog-backend/internal/planning"
	planninghttp "github.com/jPurin-gg/myfitlog-backend/internal/planning/httpapi"
	planningpostgres "github.com/jPurin-gg/myfitlog-backend/internal/planning/postgres"
	profilecore "github.com/jPurin-gg/myfitlog-backend/internal/profile"
	profilehttp "github.com/jPurin-gg/myfitlog-backend/internal/profile/httpapi"
	profilepostgres "github.com/jPurin-gg/myfitlog-backend/internal/profile/postgres"
	"github.com/jPurin-gg/myfitlog-backend/internal/prompt"
	reportingcore "github.com/jPurin-gg/myfitlog-backend/internal/reporting"
	reportinghttp "github.com/jPurin-gg/myfitlog-backend/internal/reporting/httpapi"
	reportingpostgres "github.com/jPurin-gg/myfitlog-backend/internal/reporting/postgres"
	workoutcore "github.com/jPurin-gg/myfitlog-backend/internal/workout"
	workouthttp "github.com/jPurin-gg/myfitlog-backend/internal/workout/httpapi"
	workoutpostgres "github.com/jPurin-gg/myfitlog-backend/internal/workout/postgres"
)

func NewHandler(db *sql.DB, cfg config.Config, logger *slog.Logger) http.Handler {
	appClock := clock.NewSystem(cfg.Timezone)
	promptRenderer := prompt.NewRenderer(cfg.PromptDir)
	aiClient := openaicompat.New(cfg.AI, logger)

	authRepository := authpostgres.New(db)
	authService := authcore.NewService(authRepository)
	authHandler := authhttp.NewHandler(authService, authcore.NewTokenSigner(cfg.SessionSecret, cfg.SessionTTL), appClock, cfg.SessionCookieName, cfg.SessionCookieSecure, cfg.SessionTTL)

	profileService := profilecore.NewService(profilepostgres.New(db))
	profileHandler := profilehttp.NewHandler(profileService)
	exerciseService := exercisecore.NewService(exercisepostgres.New(db), aiClient, promptRenderer).WithLogger(logger)
	exerciseHandler := exercisehttp.NewHandler(exerciseService)
	planningService := planningcore.NewService(planningpostgres.New(db), profileService, aiClient, promptRenderer, appClock, cfg.AI.OptionalTimeout).WithLogger(logger)
	planningHandler := planninghttp.NewHandler(planningService)
	workoutService := workoutcore.NewService(workoutpostgres.New(db), aiClient, promptRenderer, appClock, cfg.AI.OptionalTimeout).WithLogger(logger)
	workoutHandler := workouthttp.NewHandler(workoutService)
	reportingService := reportingcore.NewService(reportingpostgres.New(db), appClock)
	reportingHandler := reportinghttp.NewHandler(reportingService)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.Handle("GET /api/auth/me", authHandler.Authenticate(http.HandlerFunc(authHandler.Me)))
	mux.Handle("DELETE /api/auth/session", authHandler.Authenticate(http.HandlerFunc(authHandler.Logout)))

	protected := func(handler http.HandlerFunc) http.Handler { return authHandler.Authenticate(handler) }
	mux.Handle("GET /api/dashboard", protected(reportingHandler.Dashboard))
	mux.Handle("GET /api/calendar", protected(reportingHandler.Calendar))
	mux.Handle("GET /api/preferences", protected(profileHandler.Get))
	mux.Handle("PUT /api/preferences", protected(profileHandler.Put))

	mux.Handle("GET /api/exercises", protected(exerciseHandler.Search))
	mux.Handle("POST /api/exercises", protected(exerciseHandler.Create))
	mux.Handle("GET /api/exercises/recent", protected(exerciseHandler.Recent))
	mux.Handle("GET /api/exercises/favorites", protected(exerciseHandler.Favorites))
	mux.Handle("PUT /api/exercises/{exerciseID}/favorite", protected(exerciseHandler.PutFavorite))
	mux.Handle("DELETE /api/exercises/{exerciseID}/favorite", protected(exerciseHandler.DeleteFavorite))
	mux.Handle("GET /api/exercises/{exerciseID}/settings", protected(exerciseHandler.Settings))
	mux.Handle("PUT /api/exercises/{exerciseID}/settings", protected(exerciseHandler.PutSettings))
	mux.Handle("POST /api/exercises/{exerciseID}/alternatives", protected(exerciseHandler.Alternatives))

	mux.Handle("GET /api/monthly-plans", protected(planningHandler.MonthlyList))
	mux.Handle("GET /api/monthly-plans/{month}", protected(planningHandler.Monthly))
	mux.Handle("PUT /api/monthly-plans/{month}", protected(planningHandler.PutMonthly))
	mux.Handle("POST /api/monthly-plans/{month}/generate", protected(planningHandler.GenerateMonthly))
	mux.Handle("GET /api/workout-plans/{date}", protected(planningHandler.Daily))
	mux.Handle("PUT /api/workout-plans/{date}", protected(planningHandler.PutDaily))
	mux.Handle("POST /api/workout-plans/{date}/start", protected(planningHandler.StartDaily))

	mux.Handle("GET /api/workouts/by-date/{date}", protected(workoutHandler.CalendarWorkout))
	mux.Handle("PUT /api/workouts/by-date/{date}", protected(workoutHandler.PutCalendarWorkout))
	mux.Handle("GET /api/workouts/{workoutID}", protected(workoutHandler.Detail))
	mux.Handle("POST /api/workouts/{workoutID}/sets", protected(workoutHandler.RecordSet))
	mux.Handle("POST /api/workouts/{workoutID}/sets/{setID}/recommendation", protected(workoutHandler.Recommendation))
	mux.Handle("POST /api/workouts/{workoutID}/finish", protected(workoutHandler.Finish))
	mux.Handle("POST /api/workouts/{workoutID}/summary-comment", protected(workoutHandler.SummaryComment))

	var handler http.Handler = httpx.NormalizeRoutingErrors(mux)
	handler = httpx.CORS(cfg.FrontendURL, handler)
	handler = httpx.Recover(logger, handler)
	handler = httpx.LogRequests(logger, handler)
	handler = httpx.WithRequestID(handler)
	return handler
}
