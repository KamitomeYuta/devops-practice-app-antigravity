package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newTestApp(cfg Config) *App {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return &App{
		Config: cfg,
		Logger: logger,
	}
}

func TestHealthzHandler(t *testing.T) {
	app := newTestApp(Config{
		Port: "8080",
		Env:  "development",
	})
	handler := app.Routes()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}

	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %q", response["status"])
	}
}

func TestReadyHandler(t *testing.T) {
	app := newTestApp(Config{
		Port: "8080",
		Env:  "development",
	})
	handler := app.Routes()

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}

	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if response["status"] != "ready" {
		t.Errorf("expected status 'ready', got %q", response["status"])
	}
}

func TestInfoHandler(t *testing.T) {
	expectedEnv := "production"
	expectedVersion := "2.1.0"
	expectedHostname, _ := os.Hostname()

	app := newTestApp(Config{
		Port:    "8080",
		Env:     expectedEnv,
		Version: expectedVersion,
	})
	handler := app.Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/info", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}

	var response InfoResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if response.Version != expectedVersion {
		t.Errorf("expected version %q, got %q", expectedVersion, response.Version)
	}
	if response.Environment != expectedEnv {
		t.Errorf("expected environment %q, got %q", expectedEnv, response.Environment)
	}
	if response.Hostname != expectedHostname && expectedHostname != "" {
		t.Errorf("expected hostname %q, got %q", expectedHostname, response.Hostname)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	app := newTestApp(Config{Port: "8080", Env: "development"})
	handler := app.Routes()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"POST to healthz", http.MethodPost, "/healthz"},
		{"DELETE to ready", http.MethodDelete, "/ready"},
		{"PUT to info", http.MethodPut, "/api/v1/info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
			}
		})
	}
}

func TestNotFound(t *testing.T) {
	app := newTestApp(Config{Port: "8080", Env: "development"})
	handler := app.Routes()

	req := httptest.NewRequest(http.MethodGet, "/non-existent", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestLoadConfig(t *testing.T) {
	t.Run("Default configuration", func(t *testing.T) {
		_ = os.Unsetenv("PORT")
		_ = os.Unsetenv("APP_ENV")
		_ = os.Unsetenv("APP_VERSION")
		_ = os.Unsetenv("SHUTDOWN_TIMEOUT_SECONDS")

		cfg := LoadConfig()

		if cfg.Port != "8080" {
			t.Errorf("expected default port 8080, got %q", cfg.Port)
		}
		if cfg.Env != "development" {
			t.Errorf("expected default env 'development', got %q", cfg.Env)
		}
		if cfg.Version != "1.0.0" {
			t.Errorf("expected default version '1.0.0', got %q", cfg.Version)
		}
		if cfg.ShutdownTimeout != 10*time.Second {
			t.Errorf("expected default shutdown timeout 10s, got %v", cfg.ShutdownTimeout)
		}
	})

	t.Run("Environment overrides", func(t *testing.T) {
		t.Setenv("PORT", "3000")
		t.Setenv("APP_ENV", "staging")
		t.Setenv("APP_VERSION", "3.0.0")
		t.Setenv("SHUTDOWN_TIMEOUT_SECONDS", "15")

		cfg := LoadConfig()

		if cfg.Port != "3000" {
			t.Errorf("expected port 3000, got %q", cfg.Port)
		}
		if cfg.Env != "staging" {
			t.Errorf("expected env 'staging', got %q", cfg.Env)
		}
		if cfg.Version != "3.0.0" {
			t.Errorf("expected version '3.0.0', got %q", cfg.Version)
		}
		if cfg.ShutdownTimeout != 15*time.Second {
			t.Errorf("expected shutdown timeout 15s, got %v", cfg.ShutdownTimeout)
		}
	})
}

func TestServerGracefulShutdown(t *testing.T) {
	// Pick an available port dynamically
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to bind test listener: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	cfg := Config{
		Port:            strconv.Itoa(port),
		Env:             "test",
		Version:         "test-version",
		ShutdownTimeout: 2 * time.Second,
	}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, cfg, logger)
	}()

	// Allow server to initialize and start listening
	time.Sleep(100 * time.Millisecond)

	// Trigger graceful shutdown
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run returned error during shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server shutdown timed out")
	}
}
