package router

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/example/insightflow/src/backend/handlers"
)

// NewRouter creates and configures the HTTP router for the InsightFlow API.
// It sets up common middleware (request ID, logging, panic recovery, timeout, CORS)
// and registers all endpoint routes.
//
// The returned http.Handler can be passed directly to http.ListenAndServe or
// used by any HTTP server implementation.
func NewRouter() http.Handler {
	r := chi.NewRouter()

	// -------------------------------------------------------------------------
	// Middleware stack
	// -------------------------------------------------------------------------
	// Assign a unique request ID to each request (X-Request-ID header)
	r.Use(middleware.RequestID)

	// Populate the RemoteIP field on the request context
	r.Use(middleware.RealIP)

	// Log the start and end of each request with method, path, status, latency
	r.Use(middleware.Logger)

	// Recover from panics and return a 500 status code instead of crashing the process
	r.Use(middleware.Recoverer)

	// Enforce a maximum request processing time
	r.Use(middleware.Timeout(60 * time.Second))

	// Enable CORS for the frontend (adjust origins in production)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// -------------------------------------------------------------------------
	// Health check endpoint
	// -------------------------------------------------------------------------
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// -------------------------------------------------------------------------
	// API version 1 routes
	// -------------------------------------------------------------------------
	r.Route("/api/v1", func(api chi.Router) {
		// KPI CRUD and aggregation endpoints
		api.Post("/kpis", handlers.CreateKPI)               // Create a new KPI record
		api.Get("/kpis/{id}", handlers.GetKPI)             // Retrieve a single KPI by UUID
		api.Get("/kpis", handlers.ListKPIs)               // List KPIs with optional filters/pagination
		api.Get("/kpis/{id}/aggregate", handlers.AggregateKPI) // Return aggregated metrics for a KPI

		// Additional feature routes could be added here (alerts, reports, etc.)
	})

	return r
}