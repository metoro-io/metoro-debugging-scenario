package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Logger
var logger *StructuredLogger

// HTTP client
var httpClient *http.Client

// Configuration
var (
	cacheServiceURL string
	apiToken        string
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
			Name: "instabook_request_count",
			Help: "Number of requests received by the instabook service",
		},
		[]string{"method", "endpoint", "status"},
	)
	responseTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "instabook_response_time",
			Help:    "Response time of the instabook service",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)
	cacheErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "instabook_cache_errors",
			Help: "Number of errors from cache service",
		},
		[]string{"error_type"},
	)
)

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func init() {
	prometheus.MustRegister(requestCount)
	prometheus.MustRegister(responseTime)
	prometheus.MustRegister(cacheErrors)

	cacheServiceURL = getEnv("INSTABOOK_CACHE_SERVICE", "http://localhost:8086")
	apiToken = getEnv("INSTABOOK_API_TOKEN", "instabook-secret-token-2024")
	logger = NewStructuredLogger("instabook")

	httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}
}

// callCache makes a request to the cache service with proper auth
func callCache(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	url := cacheServiceURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return httpClient.Do(req)
}

func main() {
	router := gin.Default()

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "UP"})
	})

	// Metrics
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Get booking session
	router.GET("/booking/session/:id", func(c *gin.Context) {
		ctx := c.Request.Context()
		start := time.Now()
		id := c.Param("id")

		logger.Info(ctx, "Getting booking session", map[string]interface{}{
			"session_id": id,
		})

		// Call cache service
		resp, err := callCache(ctx, "GET", "/cache/session/"+id, nil)
		if err != nil {
			logger.Error(ctx, "Failed to call cache service", map[string]interface{}{
				"session_id": id,
				"error":      err.Error(),
			})
			cacheErrors.WithLabelValues("connection_error").Inc()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal service error"})
			requestCount.WithLabelValues("GET", "/booking/session/:id", "500").Inc()
			return
		}
		defer resp.Body.Close()

		// Handle 401 from cache (token authentication disabled)
		if resp.StatusCode == http.StatusUnauthorized {
			logger.Error(ctx, "Cache authentication failed", map[string]interface{}{
				"session_id":  id,
				"status_code": resp.StatusCode,
			})
			cacheErrors.WithLabelValues("auth_failure").Inc()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal service authentication failure"})
			requestCount.WithLabelValues("GET", "/booking/session/:id", "500").Inc()
			return
		}

		// Handle 404 from cache
		if resp.StatusCode == http.StatusNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			requestCount.WithLabelValues("GET", "/booking/session/:id", "404").Inc()
			return
		}

		// Handle other errors
		if resp.StatusCode >= 400 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			logger.Error(ctx, "Cache service returned error", map[string]interface{}{
				"session_id":  id,
				"status_code": resp.StatusCode,
				"response":    string(bodyBytes),
			})
			cacheErrors.WithLabelValues("cache_error").Inc()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal service error"})
			requestCount.WithLabelValues("GET", "/booking/session/:id", "500").Inc()
			return
		}

		// Parse and return session
		var session Session
		if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
			logger.Error(ctx, "Failed to decode cache response", map[string]interface{}{
				"session_id": id,
				"error":      err.Error(),
			})
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal service error"})
			requestCount.WithLabelValues("GET", "/booking/session/:id", "500").Inc()
			return
		}

		c.JSON(http.StatusOK, session)

		duration := time.Since(start).Seconds()
		requestCount.WithLabelValues("GET", "/booking/session/:id", "200").Inc()
		responseTime.WithLabelValues("GET", "/booking/session/:id").Observe(duration)
	})

	// Create booking session
	router.POST("/booking/session", func(c *gin.Context) {
		ctx := c.Request.Context()
		start := time.Now()

		var session Session
		if err := c.ShouldBindJSON(&session); err != nil {
			logger.Error(ctx, "Failed to parse session data", map[string]interface{}{
				"error": err.Error(),
			})
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session data"})
			requestCount.WithLabelValues("POST", "/booking/session", "400").Inc()
			return
		}

		logger.Info(ctx, "Creating booking session", map[string]interface{}{
			"session_id": session.ID,
			"user_id":    session.UserID,
		})

		// Call cache service to store session
		resp, err := callCache(ctx, "POST", "/cache/session", session)
		if err != nil {
			logger.Error(ctx, "Failed to call cache service", map[string]interface{}{
				"session_id": session.ID,
				"error":      err.Error(),
			})
			cacheErrors.WithLabelValues("connection_error").Inc()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal service error"})
			requestCount.WithLabelValues("POST", "/booking/session", "500").Inc()
			return
		}
		defer resp.Body.Close()

		// Handle 401 from cache (token authentication disabled)
		if resp.StatusCode == http.StatusUnauthorized {
			logger.Error(ctx, "Cache authentication failed", map[string]interface{}{
				"session_id":  session.ID,
				"status_code": resp.StatusCode,
			})
			cacheErrors.WithLabelValues("auth_failure").Inc()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal service authentication failure"})
			requestCount.WithLabelValues("POST", "/booking/session", "500").Inc()
			return
		}

		// Handle other errors
		if resp.StatusCode >= 400 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			logger.Error(ctx, "Cache service returned error", map[string]interface{}{
				"session_id":  session.ID,
				"status_code": resp.StatusCode,
				"response":    string(bodyBytes),
			})
			cacheErrors.WithLabelValues("cache_error").Inc()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal service error"})
			requestCount.WithLabelValues("POST", "/booking/session", "500").Inc()
			return
		}

		// Parse and return created session
		var createdSession Session
		if err := json.NewDecoder(resp.Body).Decode(&createdSession); err != nil {
			logger.Error(ctx, "Failed to decode cache response", map[string]interface{}{
				"session_id": session.ID,
				"error":      err.Error(),
			})
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal service error"})
			requestCount.WithLabelValues("POST", "/booking/session", "500").Inc()
			return
		}

		c.JSON(http.StatusCreated, createdSession)

		duration := time.Since(start).Seconds()
		requestCount.WithLabelValues("POST", "/booking/session", "201").Inc()
		responseTime.WithLabelValues("POST", "/booking/session").Observe(duration)
	})

	port := getEnv("PORT", "8087")
	logger.Info(context.Background(), "Instabook Service starting", map[string]interface{}{
		"port":              port,
		"cache_service_url": cacheServiceURL,
	})
	router.Run(":" + port)
}
