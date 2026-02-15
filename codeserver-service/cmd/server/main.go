package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/devplatform/codeserver-service/internal/auth"
	"github.com/devplatform/codeserver-service/internal/config"
	"github.com/devplatform/codeserver-service/internal/gitea"
	"github.com/devplatform/codeserver-service/internal/graphql"
	"github.com/devplatform/codeserver-service/internal/kubernetes"
	"github.com/graphql-go/handler"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

var (
	version   = "dev"
	buildTime = "unknown"

	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "codeserver_service_requests_total",
			Help: "Total number of requests",
		},
		[]string{"method", "path", "status"},
	)
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "codeserver_service_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
	panicsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "codeserver_service_panics_total",
			Help: "Total number of panics recovered",
		},
	)
)

func init() {
	prometheus.MustRegister(requestsTotal, requestDuration, panicsTotal)
}

func main() {
	cfg := config.Load()
	logger := setupLogger(cfg)

	logger.WithFields(logrus.Fields{
		"version":   version,
		"buildTime": buildTime,
	}).Info("Starting codeserver-service")

	k8sClient, err := kubernetes.NewClient(cfg, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create Kubernetes client")
	}

	giteaClient := gitea.NewClient(cfg, logger)
	gqlSchema := graphql.NewSchema(k8sClient, giteaClient, cfg, logger)
	authMiddleware := auth.NewMiddleware(logger)

	mux := http.NewServeMux()

	schema := gqlSchema.GetSchema()
	gqlHandler := handler.New(&handler.Config{
		Schema:   &schema,
		Pretty:   cfg.IsDevelopment(),
		GraphiQL: cfg.IsDevelopment(),
	})

	mux.Handle("/graphql", gqlHandler)
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ready", readyHandler(k8sClient, giteaClient))

	// Middleware chain (order matters - outermost first)
	var finalHandler http.Handler = mux
	finalHandler = authMiddleware.ExtractToken(finalHandler)
	finalHandler = metricsMiddleware(finalHandler)
	finalHandler = loggingMiddleware(logger)(finalHandler)
	finalHandler = recoveryMiddleware(logger)(finalHandler) // Panic recovery
	finalHandler = corsMiddleware(cfg.CORSOrigins)(finalHandler)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      finalHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go startMetricsServer(cfg, logger)

	go func() {
		logger.WithField("port", cfg.Port).Info("Starting HTTP server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("HTTP server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ShutdownTimeout)*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.WithError(err).Error("Server forced to shutdown")
	}

	logger.Info("Server stopped")
}

func setupLogger(cfg *config.Config) *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "level",
			logrus.FieldKeyMsg:   "message",
		},
	})

	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	return logger
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func readyHandler(k8s *kubernetes.Client, giteaClient *gitea.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := k8s.HealthCheck(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"not ready","error":"kubernetes unavailable"}`))
			return
		}

		if err := giteaClient.HealthCheck(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"not ready","error":"gitea-service unavailable"}`))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	}
}

func startMetricsServer(cfg *config.Config, logger *logrus.Logger) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.MetricsPort),
		Handler: mux,
	}

	logger.WithField("port", cfg.MetricsPort).Info("Starting metrics server")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.WithError(err).Error("Metrics server failed")
	}
}

// recoveryMiddleware recovers from panics and returns 500 error
func recoveryMiddleware(logger *logrus.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Log the panic with stack trace
					stack := debug.Stack()
					logger.WithFields(logrus.Fields{
						"error":  err,
						"stack":  string(stack),
						"method": r.Method,
						"path":   r.URL.Path,
					}).Error("Panic recovered")

					// Increment panic counter
					panicsTotal.Inc()

					// Return 500 error
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"error":"internal server error"}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func corsMiddleware(origins string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if origins == "*" {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				} else {
					for _, allowed := range strings.Split(origins, ",") {
						if strings.TrimSpace(allowed) == origin {
							w.Header().Set("Access-Control-Allow-Origin", origin)
							break
						}
					}
				}
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func loggingMiddleware(logger *logrus.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			logger.WithFields(logrus.Fields{
				"method":   r.Method,
				"path":     r.URL.Path,
				"status":   wrapped.statusCode,
				"duration": time.Since(start).String(),
				"ip":       r.RemoteAddr,
			}).Info("Request handled")
		})
	}
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start).Seconds()
		requestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", wrapped.statusCode)).Inc()
		requestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
