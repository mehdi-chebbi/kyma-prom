package main

import (
        "context"
        "encoding/json"
        "fmt"
        "net/http"
        "os"
        "os/signal"
        "syscall"
        "time"

        "github.com/devplatform/ldap-manager/internal/auth"
        "github.com/devplatform/ldap-manager/internal/config"
        "github.com/devplatform/ldap-manager/internal/graphql"
        "github.com/devplatform/ldap-manager/internal/ldap"
        "github.com/devplatform/ldap-manager/internal/prometheus"
        gql "github.com/graphql-go/graphql"
        promclient "github.com/prometheus/client_golang/prometheus"
        "github.com/prometheus/client_golang/prometheus/promauto"
        "github.com/prometheus/client_golang/prometheus/promhttp"
        "github.com/sirupsen/logrus"
)

var (
        requestsTotal = promauto.NewCounterVec(
                promclient.CounterOpts{
                        Name: "ldap_manager_requests_total",
                        Help: "Total number of HTTP requests",
                },
                []string{"method", "path", "status"},
        )

        requestDuration = promauto.NewHistogramVec(
                promclient.HistogramOpts{
                        Name:    "ldap_manager_request_duration_seconds",
                        Help:    "HTTP request duration in seconds",
                        Buckets: promclient.DefBuckets,
                },
                []string{"method", "path"},
        )

        ldapOperations = promauto.NewCounterVec(
                promclient.CounterOpts{
                        Name: "ldap_manager_operations_total",
                        Help: "Total number of LDAP operations",
                },
                []string{"operation", "status"},
        )
)

func main() {
        // Load configuration
        cfg := config.Load()

        // Setup logger
        logger := setupLogger(cfg)
        logger.Info("Starting LDAP Manager Service")

        // Initialize business-level Prometheus metrics
        logger.Info("Initializing Prometheus metrics")
        prometheus.Init()

        // Initialize LDAP manager
        logger.Info("Initializing LDAP connection pool")
        ldapMgr, err := ldap.NewManager(cfg, logger)
        if err != nil {
                logger.WithError(err).Fatal("Failed to initialize LDAP manager")
        }
        defer ldapMgr.Close()

        // Test LDAP connection
        ctx := context.Background()
        if err := ldapMgr.HealthCheck(ctx); err != nil {
                logger.WithError(err).Warn("Initial LDAP health check failed")
        } else {
                logger.Info("LDAP connection successful")
        }

        // Wrap LDAP manager with metrics collector
        instrumentedMgr := prometheus.NewLDAPCollector(ldapMgr)
        logger.Info("LDAP manager wrapped with Prometheus metrics collector")

        // Initialize GraphQL schema
        logger.Info("Initializing GraphQL schema")
        gqlSchema := graphql.NewSchema(instrumentedMgr, cfg, logger)

        // Setup HTTP server
        srv := setupHTTPServer(cfg, gqlSchema, ldapMgr, logger)

        // Start metrics server in background
        go startMetricsServer(cfg, logger)

        // Start main server in background
        go func() {
                logger.WithField("port", cfg.Port).Info("Starting HTTP server")
                if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
                        logger.WithError(err).Fatal("Server failed to start")
                }
        }()

        // Wait for shutdown signal
        waitForShutdown(srv, ldapMgr, cfg, logger)
}

func setupLogger(cfg *config.Config) *logrus.Logger {
        logger := logrus.New()
        logger.SetFormatter(&logrus.JSONFormatter{
                TimestampFormat: time.RFC3339,
                FieldMap: logrus.FieldMap{
                        logrus.FieldKeyTime:  "timestamp",
                        logrus.FieldKeyLevel: "level",
                        logrus.FieldKeyMsg:   "message",
                },
        })
        logger.SetOutput(os.Stdout)

        // Set log level based on environment
        if cfg.LogLevel == "debug" {
                logger.SetLevel(logrus.DebugLevel)
        } else {
                logger.SetLevel(logrus.InfoLevel)
        }

        return logger
}

func setupHTTPServer(cfg *config.Config, gqlSchema *graphql.Schema, ldapMgr *ldap.Manager, logger *logrus.Logger) *http.Server {
        mux := http.NewServeMux()

        // GraphQL endpoint
        mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
                // Handle CORS preflight
                if r.Method == "OPTIONS" {
                        w.WriteHeader(http.StatusOK)
                        return
                }

                // Parse request
                var params struct {
                        Query         string                 `json:"query"`
                        OperationName string                 `json:"operationName"`
                        Variables     map[string]interface{} `json:"variables"`
                }

                if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
                        http.Error(w, err.Error(), http.StatusBadRequest)
                        return
                }

                // Execute GraphQL query
                result := gql.Do(gql.Params{
                        Schema:         gqlSchema.GetSchema(),
                        RequestString:  params.Query,
                        VariableValues: params.Variables,
                        OperationName:  params.OperationName,
                        Context:        r.Context(),
                })

                // Write response
                w.Header().Set("Content-Type", "application/json")
                if len(result.Errors) > 0 {
                        logger.WithField("errors", result.Errors).Warn("GraphQL errors")
                }
                json.NewEncoder(w).Encode(result)
        })

        // Health endpoint (liveness probe)
        mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
                w.Header().Set("Content-Type", "application/json")
                w.WriteHeader(http.StatusOK)
                json.NewEncoder(w).Encode(map[string]string{
                        "status": "ok",
                })
        })

        // Readiness endpoint (readiness probe)
        mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
                ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
                defer cancel()

                // Test LDAP connection
                if err := ldapMgr.HealthCheck(ctx); err != nil {
                        logger.WithError(err).Warn("Readiness check failed")
                        w.WriteHeader(http.StatusServiceUnavailable)
                        json.NewEncoder(w).Encode(map[string]string{
                                "status": "unavailable",
                                "error":  err.Error(),
                        })
                        return
                }

                w.Header().Set("Content-Type", "application/json")
                w.WriteHeader(http.StatusOK)
                json.NewEncoder(w).Encode(map[string]string{
                        "status": "ready",
                })
        })

        // Apply middleware
        authMw := auth.NewMiddleware(logger)
        handler := corsMiddleware(cfg)(mux)
        handler = loggingMiddleware(logger)(handler)
        handler = metricsMiddleware()(handler)
        handler = authMw.ExtractToken(handler)
        handler = injectDependencies(handler, gqlSchema, logger)

        return &http.Server{
                Addr:         fmt.Sprintf(":%d", cfg.Port),
                Handler:      handler,
                ReadTimeout:  15 * time.Second,
                WriteTimeout: 15 * time.Second,
                IdleTimeout:  60 * time.Second,
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

// Middleware

func corsMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
        return func(next http.Handler) http.Handler {
                return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                        origin := r.Header.Get("Origin")

                        // Allow configured origins or all in dev
                        allowedOrigin := "*"
                        if len(cfg.CORSOrigins) > 0 {
                                for _, allowed := range cfg.CORSOrigins {
                                        if origin == allowed {
                                                allowedOrigin = origin
                                                break
                                        }
                                }
                        }

                        w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
                        w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
                        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
                        w.Header().Set("Access-Control-Max-Age", "3600")

                        next.ServeHTTP(w, r)
                })
        }
}

func loggingMiddleware(logger *logrus.Logger) func(http.Handler) http.Handler {
        return func(next http.Handler) http.Handler {
                return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                        start := time.Now()

                        // Wrap response writer to capture status code
                        rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

                        next.ServeHTTP(rw, r)

                        logger.WithFields(logrus.Fields{
                                "method":     r.Method,
                                "path":       r.URL.Path,
                                "status":     rw.statusCode,
                                "duration":   time.Since(start).Milliseconds(),
                                "remote_addr": r.RemoteAddr,
                        }).Info("HTTP request")
                })
        }
}

func metricsMiddleware() func(http.Handler) http.Handler {
        return func(next http.Handler) http.Handler {
                return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                        start := time.Now()
                        rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

                        next.ServeHTTP(rw, r)

                        duration := time.Since(start).Seconds()
                        requestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", rw.statusCode)).Inc()
                        requestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
                })
        }
}

func injectDependencies(next http.Handler, gqlSchema *graphql.Schema, logger *logrus.Logger) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                // Inject dependencies into context
                ctx := r.Context()
                if _, ok := ctx.Value("ldapManager").(*ldap.Manager); !ok {
                        // Note: This is a workaround - in production, use proper dependency injection
                        // For now, the manager is accessed via the gqlSchema
                }
                next.ServeHTTP(w, r)
        })
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
        http.ResponseWriter
        statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
        rw.statusCode = code
        rw.ResponseWriter.WriteHeader(code)
}

func waitForShutdown(srv *http.Server, ldapMgr *ldap.Manager, cfg *config.Config, logger *logrus.Logger) {
        // Create channel to listen for interrupt signals
        quit := make(chan os.Signal, 1)
        signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

        // Block until signal received
        sig := <-quit
        logger.WithField("signal", sig.String()).Info("Shutdown signal received")

        // Create context with timeout for shutdown
        timeout := 30 * time.Second
        if cfg.ShutdownTimeout > 0 {
                timeout = time.Duration(cfg.ShutdownTimeout) * time.Second
        }

        ctx, cancel := context.WithTimeout(context.Background(), timeout)
        defer cancel()

        // Shutdown HTTP server
        logger.Info("Shutting down HTTP server...")
        if err := srv.Shutdown(ctx); err != nil {
                logger.WithError(err).Error("Server shutdown failed")
        }

        // Close LDAP connection pool
        logger.Info("Closing LDAP connections...")
        ldapMgr.Close()

        logger.Info("Shutdown complete")
}
