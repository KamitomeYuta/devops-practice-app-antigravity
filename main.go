package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var (
	// Version is populated at build time using -ldflags "-X main.Version=x.y.z",
	// or falls back to the default or APP_VERSION environment variable.
	Version = "1.0.0"
)

// Config holds runtime configuration loaded strictly from environment variables.
type Config struct {
	Port            string
	Env             string
	Version         string
	ShutdownTimeout time.Duration
}

// LoadConfig reads configuration from the environment with production-grade defaults.
func LoadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	appVersion := os.Getenv("APP_VERSION")
	if appVersion == "" {
		appVersion = Version
	}

	shutdownTimeoutSeconds := 10
	if rawTimeout := os.Getenv("SHUTDOWN_TIMEOUT_SECONDS"); rawTimeout != "" {
		if parsed, err := strconv.Atoi(rawTimeout); err == nil && parsed > 0 {
			shutdownTimeoutSeconds = parsed
		}
	}

	return Config{
		Port:            port,
		Env:             env,
		Version:         appVersion,
		ShutdownTimeout: time.Duration(shutdownTimeoutSeconds) * time.Second,
	}
}

// App encapsulates dependencies for the HTTP service.
type App struct {
	Config Config
	Logger *slog.Logger
}

// statusWriter is an http.ResponseWriter wrapper to capture the HTTP status code.
type statusWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// loggingMiddleware logs incoming HTTP requests as structured JSON using slog.
func (a *App) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(sw, r)

		a.Logger.Info("http request completed",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("remote_addr", r.RemoteAddr),
			slog.String("user_agent", r.UserAgent()),
			slog.Int("status", sw.statusCode),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

// writeJSON writes a standard JSON response with appropriate headers.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode response JSON", "error", err)
	}
}

// HealthzHandler responds with 200 OK for Kubernetes liveness probes.
func (a *App) HealthzHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

// ReadyHandler responds with 200 OK for Kubernetes readiness probes.
func (a *App) ReadyHandler(w http.ResponseWriter, r *http.Request) {
	// In production, add external readiness checks here (e.g. database ping, cache connection).
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ready",
	})
}

// InfoResponse defines the payload returned by the /api/v1/info endpoint.
type InfoResponse struct {
	Version     string `json:"version"`
	Environment string `json:"environment"`
	Hostname    string `json:"hostname"`
}

// InfoHandler returns metadata about application version, environment, and pod/host.
func (a *App) InfoHandler(w http.ResponseWriter, r *http.Request) {
	hostname, err := os.Hostname()
	if err != nil {
		a.Logger.Warn("unable to determine hostname", "error", err)
		hostname = "unknown"
	}

	writeJSON(w, http.StatusOK, InfoResponse{
		Version:     a.Config.Version,
		Environment: a.Config.Env,
		Hostname:    hostname,
	})
}

// Routes constructs and returns the HTTP routing multiplexer.
func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()

	// Kubernetes Probes
	mux.HandleFunc("GET /healthz", a.HealthzHandler)
	mux.HandleFunc("GET /ready", a.ReadyHandler)

	// API Endpoints
	mux.HandleFunc("GET /api/v1/info", a.InfoHandler)

	// Global Middleware
	return a.loggingMiddleware(mux)
}

// Run boots up the HTTP server and manages graceful shutdown on SIGINT/SIGTERM.
func Run(ctx context.Context, cfg Config, logger *slog.Logger) error {
	app := &App{
		Config: cfg,
		Logger: logger,
	}

	server := &http.Server{
		Addr:              net.JoinHostPort("", cfg.Port),
		Handler:           app.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)

	go func() {
		logger.Info("starting HTTP server",
			slog.String("port", cfg.Port),
			slog.String("env", cfg.Env),
			slog.String("version", cfg.Version),
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server failed to start or listen: %w", err)
	case <-ctx.Done():
		logger.Info("shutdown signal received, initiating graceful shutdown",
			slog.Duration("timeout", cfg.ShutdownTimeout),
		)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return fmt.Errorf("failed to gracefully shut down server: %w", err)
	}

	logger.Info("server shutdown complete")
	return nil
}

func main() {
	// Initialize structured JSON logger using slog
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := LoadConfig()

	// Intercept SIGINT and SIGTERM for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := Run(ctx, cfg, logger); err != nil {
		logger.Error("application exited with error", "error", err)
		os.Exit(1)
	}
}
