package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

// Tracer
var tracer trace.Tracer

// Logger
var logger *StructuredLogger

// Token configuration
var (
	tokenEnabled = true
	tokenMutex   sync.RWMutex
	apiToken     string
)

// Session storage
var (
	sessions     = make(map[string]*Session)
	sessionMutex sync.RWMutex
)

// Session represents a booking session
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	BookingID string    `json:"booking_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	Data      string    `json:"data"`
}

// Prometheus metrics
var (
	requestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "instabook_cache_request_count",
			Help: "Number of requests received by the instabook cache service",
		},
		[]string{"method", "endpoint", "status"},
	)
	responseTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "instabook_cache_response_time",
			Help:    "Response time of the instabook cache service",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)
)

// Initialize OpenTelemetry
func initTracer() *sdktrace.TracerProvider {
	exporter, err := otlptracehttp.New(
		context.Background(),
		otlptracehttp.WithEndpoint(getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4318")),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		log.Fatalf("Failed to create exporter: %v", err)
	}

	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("instabook-cache"),
			semconv.DeploymentEnvironmentKey.String(getEnv("DEPLOYMENT_ENVIRONMENT", "production")),
		),
	)
	if err != nil {
		log.Fatalf("Failed to create resource: %v", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	tracer = tp.Tracer("instabook-cache")
	logger = NewStructuredLogger("instabook-cache")

	return tp
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func init() {
	prometheus.MustRegister(requestCount)
	prometheus.MustRegister(responseTime)
	apiToken = getEnv("INSTABOOK_API_TOKEN", "instabook-secret-token-2024")
}

// Admin HTML page
const adminHTML = `<!DOCTYPE html>
<html>
<head>
    <title>Instabook Cache Admin</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            max-width: 600px;
            margin: 50px auto;
            padding: 20px;
            background: #f5f5f5;
        }
        .container {
            background: white;
            padding: 30px;
            border-radius: 10px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        h1 {
            color: #333;
            margin-bottom: 30px;
        }
        .status {
            padding: 15px 20px;
            border-radius: 8px;
            margin-bottom: 20px;
            font-size: 18px;
            font-weight: 500;
        }
        .enabled {
            background: #d4edda;
            color: #155724;
            border: 1px solid #c3e6cb;
        }
        .disabled {
            background: #f8d7da;
            color: #721c24;
            border: 1px solid #f5c6cb;
        }
        button {
            padding: 12px 24px;
            font-size: 16px;
            cursor: pointer;
            border: none;
            border-radius: 6px;
            background: #007bff;
            color: white;
            transition: background 0.2s;
        }
        button:hover {
            background: #0056b3;
        }
        .info {
            margin-top: 20px;
            padding: 15px;
            background: #e9ecef;
            border-radius: 6px;
            font-size: 14px;
            color: #666;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Instabook Cache Admin</h1>
        <div id="status" class="status">Loading...</div>
        <button id="toggleBtn" onclick="toggleToken()">Toggle Token Authentication</button>
        <div class="info">
            <strong>API Token Authentication</strong><br>
            When enabled, all /cache/* endpoints require a valid Bearer token.<br>
            When disabled, all /cache/* endpoints return 401 Unauthorized.
        </div>
    </div>
    <script>
        async function fetchStatus() {
            try {
                const resp = await fetch('/admin/token');
                const data = await resp.json();
                const statusEl = document.getElementById('status');
                if (data.enabled) {
                    statusEl.className = 'status enabled';
                    statusEl.textContent = 'Token Authentication: ENABLED';
                } else {
                    statusEl.className = 'status disabled';
                    statusEl.textContent = 'Token Authentication: DISABLED (all cache requests will fail with 401)';
                }
            } catch (e) {
                console.error('Error fetching status:', e);
            }
        }

        async function toggleToken() {
            try {
                await fetch('/admin/token', { method: 'POST' });
                await fetchStatus();
            } catch (e) {
                console.error('Error toggling token:', e);
            }
        }

        fetchStatus();
    </script>
</body>
</html>`

// Authorization middleware for cache endpoints
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		tokenMutex.RLock()
		enabled := tokenEnabled
		tokenMutex.RUnlock()

		if !enabled {
			logger.Warn(ctx, "Token authentication is disabled, rejecting request", map[string]interface{}{
				"path":   c.Request.URL.Path,
				"method": c.Request.Method,
			})
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API token authentication is disabled"})
			c.Abort()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			logger.Warn(ctx, "Missing Authorization header", map[string]interface{}{
				"path":   c.Request.URL.Path,
				"method": c.Request.Method,
			})
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing Authorization header"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			logger.Warn(ctx, "Invalid Authorization header format", map[string]interface{}{
				"path":   c.Request.URL.Path,
				"method": c.Request.Method,
			})
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization header format"})
			c.Abort()
			return
		}

		if parts[1] != apiToken {
			logger.Warn(ctx, "Invalid API token", map[string]interface{}{
				"path":   c.Request.URL.Path,
				"method": c.Request.Method,
			})
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API token"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func main() {
	tp := initTracer()
	defer func() {
		ctx := context.Background()
		if err := tp.Shutdown(ctx); err != nil {
			logger.Error(ctx, "Error shutting down tracer provider", map[string]interface{}{"error": err.Error()})
		}
	}()

	router := gin.Default()
	router.Use(otelgin.Middleware("instabook-cache"))

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "UP"})
	})

	// Metrics
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Admin UI
	router.GET("/admin", func(c *gin.Context) {
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, adminHTML)
	})

	// Token status endpoint
	router.GET("/admin/token", func(c *gin.Context) {
		tokenMutex.RLock()
		enabled := tokenEnabled
		tokenMutex.RUnlock()
		c.JSON(http.StatusOK, gin.H{"enabled": enabled})
	})

	// Token toggle endpoint
	router.POST("/admin/token", func(c *gin.Context) {
		ctx := c.Request.Context()
		tokenMutex.Lock()
		tokenEnabled = !tokenEnabled
		newState := tokenEnabled
		tokenMutex.Unlock()

		logger.Info(ctx, "Token authentication toggled", map[string]interface{}{
			"enabled": newState,
		})

		c.JSON(http.StatusOK, gin.H{"enabled": newState})
	})

	// Cache endpoints with auth middleware
	cache := router.Group("/cache")
	cache.Use(authMiddleware())
	{
		// Get session
		cache.GET("/session/:id", func(c *gin.Context) {
			ctx, span := tracer.Start(c.Request.Context(), "get_session")
			defer span.End()

			start := time.Now()
			id := c.Param("id")

			logger.Info(ctx, "Getting session from cache", map[string]interface{}{
				"session_id": id,
			})

			sessionMutex.RLock()
			session, exists := sessions[id]
			sessionMutex.RUnlock()

			if !exists {
				c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
				requestCount.WithLabelValues("GET", "/cache/session/:id", "404").Inc()
				return
			}

			c.JSON(http.StatusOK, session)

			duration := time.Since(start).Seconds()
			requestCount.WithLabelValues("GET", "/cache/session/:id", "200").Inc()
			responseTime.WithLabelValues("GET", "/cache/session/:id").Observe(duration)
		})

		// Create session
		cache.POST("/session", func(c *gin.Context) {
			ctx, span := tracer.Start(c.Request.Context(), "create_session")
			defer span.End()

			start := time.Now()

			var session Session
			if err := c.ShouldBindJSON(&session); err != nil {
				logger.Error(ctx, "Failed to parse session data", map[string]interface{}{
					"error": err.Error(),
				})
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session data"})
				requestCount.WithLabelValues("POST", "/cache/session", "400").Inc()
				return
			}

			session.CreatedAt = time.Now()

			logger.Info(ctx, "Creating session in cache", map[string]interface{}{
				"session_id": session.ID,
				"user_id":    session.UserID,
			})

			sessionMutex.Lock()
			sessions[session.ID] = &session
			sessionMutex.Unlock()

			c.JSON(http.StatusCreated, session)

			duration := time.Since(start).Seconds()
			requestCount.WithLabelValues("POST", "/cache/session", "201").Inc()
			responseTime.WithLabelValues("POST", "/cache/session").Observe(duration)
		})
	}

	port := getEnv("PORT", "8086")
	logger.Info(context.Background(), "Instabook Cache Service starting", map[string]interface{}{"port": port})
	router.Run(":" + port)
}
