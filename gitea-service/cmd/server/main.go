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

        "github.com/devplatform/gitea-service/internal/auth"
        "github.com/devplatform/gitea-service/internal/config"
        "github.com/devplatform/gitea-service/internal/gitea"
        "github.com/devplatform/gitea-service/internal/graphql"
        "github.com/devplatform/gitea-service/internal/ldap"
        "github.com/devplatform/gitea-service/internal/prometheus"
        gql "github.com/graphql-go/graphql"
        promclient "github.com/prometheus/client_golang/prometheus"
        "github.com/prometheus/client_golang/prometheus/promauto"
        "github.com/prometheus/client_golang/prometheus/promhttp"
        "github.com/sirupsen/logrus"
)

var (
        requestsTotal = promauto.NewCounterVec(
                promclient.CounterOpts{
                        Name: "gitea_service_requests_total",
                        Help: "Total number of HTTP requests",
                },
                []string{"method", "path", "status"},
        )

        requestDuration = promauto.NewHistogramVec(
                promclient.HistogramOpts{
                        Name:    "gitea_service_request_duration_seconds",
                        Help:    "HTTP request duration in seconds",
                        Buckets: promclient.DefBuckets,
                },
                []string{"method", "path"},
        )

        giteaOperations = promauto.NewCounterVec(
                promclient.CounterOpts{
                        Name: "gitea_operations_total",
                        Help: "Total number of Gitea operations",
                },
                []string{"operation", "status"},
        )
)

func main() {
        // Load configuration
        cfg := config.Load()

        // Setup logger
        logger := setupLogger(cfg)
        logger.Info("Starting Gitea Microservice")

        // Get or generate Gitea token
        giteaToken := cfg.GiteaToken
        if giteaToken == "" {
                logger.Info("GITEA_TOKEN not set, auto-generating...")
                initClient := gitea.NewInitClient(&gitea.InitConfig{
                        GiteaURL:      cfg.GiteaURL,
                        AdminUser:     cfg.GiteaAdminUser,
                        AdminPassword: cfg.GiteaAdminPassword,
                        AdminEmail:    cfg.GiteaAdminEmail,
                        TokenName:     "gitea-service-token",
                }, logger)

                var err error
                giteaToken, err = initClient.EnsureAdminToken()
                if err != nil {
                        logger.WithError(err).Fatal("Failed to initialize Gitea token")
                }
        }

        // Initialize Gitea client
        logger.Info("Initializing Gitea client")
        giteaClient := gitea.NewClient(cfg.GiteaURL, giteaToken, logger)

        // Test Gitea connection
        if err := giteaClient.HealthCheck(); err != nil {
                logger.WithError(err).Warn("Initial Gitea health check failed")
        } else {
                logger.Info("Gitea connection successful")
        }

        // Initialize LDAP Manager client
        logger.Info("Initializing LDAP Manager client")
        ldapClient := ldap.NewClient(cfg.LDAPManagerURL, cfg.HTTPClientTimeout, logger)

        // Test LDAP Manager connection
        ctx := context.Background()
        if err := ldapClient.HealthCheck(ctx); err != nil {
                logger.WithError(err).Warn("Initial LDAP Manager health check failed")
        } else {
                logger.Info("LDAP Manager connection successful")
        }

        // Initialize business-level Prometheus metrics
        logger.Info("Initializing Prometheus metrics")
        prometheus.Init()

        // Initialize Gitea service
        logger.Info("Initializing Gitea service")
        giteaService := gitea.NewService(giteaClient, ldapClient, logger)

        // Wrap service with Prometheus collector for metrics
        instrumentedService := prometheus.NewGiteaCollector(giteaService)

        // Initialize GraphQL schema
        logger.Info("Initializing GraphQL schema")
        gqlSchema := graphql.NewSchema(instrumentedService, ldapClient, giteaClient, nil, cfg, logger)

        // Setup HTTP server
        srv := setupHTTPServer(cfg, gqlSchema, giteaClient, ldapClient, logger)

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
        waitForShutdown(srv, cfg, logger)
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

func setupHTTPServer(cfg *config.Config, gqlSchema *graphql.Schema, giteaClient *gitea.Client, ldapClient *ldap.Client, logger *logrus.Logger) *http.Server {
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

                readyStatus := map[string]interface{}{
                        "status":      "ready",
                        "gitea":       true,
                        "ldapManager": true,
                }

                // Test Gitea connection
                if err := giteaClient.HealthCheck(); err != nil {
                        logger.WithError(err).Warn("Gitea readiness check failed")
                        readyStatus["gitea"] = false
                        readyStatus["status"] = "unavailable"
                }

                // Test LDAP Manager connection
                if err := ldapClient.HealthCheck(ctx); err != nil {
                        logger.WithError(err).Warn("LDAP Manager readiness check failed")
                        readyStatus["ldapManager"] = false
                        readyStatus["status"] = "unavailable"
                }

                w.Header().Set("Content-Type", "application/json")
                if readyStatus["status"] == "unavailable" {
                        w.WriteHeader(http.StatusServiceUnavailable)
                } else {
                        w.WriteHeader(http.StatusOK)
                }
                json.NewEncoder(w).Encode(readyStatus)
        })

        // Apply middleware
        jwksProvider := auth.NewJWKSProvider(cfg.KeycloakURL, cfg.KeycloakRealm, logger)
        authMw := auth.NewMiddleware(jwksProvider, logger)
        handler := corsMiddleware(cfg)(mux)
        handler = loggingMiddleware(logger)(handler)
        handler = metricsMiddleware()(handler)
        handler = authMw.ExtractToken(handler)

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
                                "method":      r.Method,
                                "path":        r.URL.Path,
                                "status":      rw.statusCode,
                                "duration":    time.Since(start).Milliseconds(),
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


// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
        http.ResponseWriter
        statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
        rw.statusCode = code
        rw.ResponseWriter.WriteHeader(code)
}

func waitForShutdown(srv *http.Server, cfg *config.Config, logger *logrus.Logger) {
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

        logger.Info("Shutdown complete")
}
