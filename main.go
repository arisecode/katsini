package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// ResponseWriter helper to standardize JSON responses
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON: %v", err)
	}
}

// ErrorResponse helper for consistent error responses
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// Middleware for logging
func loggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// Middleware for panic recovery
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				writeError(w, http.StatusInternalServerError, "Internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func handleGooglePlayStore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	query := r.URL.Query()
	bundleID := query.Get("bundleId")
	lang := query.Get("lang")
	country := query.Get("country")

	if bundleID == "" {
		writeError(w, http.StatusBadRequest, "Please provide an app bundleId")
		return
	}

	app, err := GooglePlayStore(bundleID, lang, country)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"bundleId":  app.bundleID,
		"url":       app.url,
		"title":     app.title,
		"version":   app.version,
		"updated":   app.updated,
		"developer": app.developer,
	})
}

func handleAppleAppStore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	query := r.URL.Query()
	appID := query.Get("appId")
	bundleID := query.Get("bundleId")
	country := query.Get("country")

	if appID == "" && bundleID == "" {
		writeError(w, http.StatusBadRequest, "Please provide an app appId or bundleId")
		return
	}

	app, err := AppleAppStore(appID, bundleID, country)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"appId":     app.appID,
		"bundleId":  app.bundleID,
		"url":       app.url,
		"title":     app.title,
		"version":   app.version,
		"updated":   app.updated,
		"developer": app.developer,
	})
}

func handleHuaweiAppGallery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	appID := r.URL.Query().Get("appId")
	if appID == "" {
		writeError(w, http.StatusBadRequest, "Please provide an app appId")
		return
	}

	app, err := HuaweiAppGallery(appID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"appId":     app.appID,
		"bundleId":  app.bundleID,
		"url":       app.url,
		"title":     app.title,
		"version":   app.version,
		"updated":   app.updated,
		"developer": app.developer,
	})
}

func main() {
	// Create a new mux router
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/playstore", handleGooglePlayStore)
	mux.HandleFunc("/appstore", handleAppleAppStore)
	mux.HandleFunc("/appgallery", handleHuaweiAppGallery)

	// Apply middleware
	handler := loggerMiddleware(recoveryMiddleware(mux))

	// Start server
	log.Println("Server starting on :8080")
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	// Server run context
	baseCtx := context.Background()
	ctx, stop := signal.NotifyContext(baseCtx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start server
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()

	// Shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(baseCtx, 30*time.Second) //nolint:mnd // 30 seconds timeout
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited properly")
}
