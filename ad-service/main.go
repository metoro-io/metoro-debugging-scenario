package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
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

// Prometheus metrics
var (
	requestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ad_service_request_count",
			Help: "Number of requests received by the ad service",
		},
		[]string{"method", "endpoint", "status"},
	)
	responseTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ad_service_response_time",
			Help:    "Response time of the ad service",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)
)

// Ad represents an advertisement
type Ad struct {
	ID          string `json:"id"`
	RedirectURL string `json:"redirect_url"`
	Text        string `json:"text"`
	ImageURL    string `json:"image_url"`
	ProductID   int    `json:"product_id,omitempty"`
	Category    string `json:"category"`
}

// Initialize OpenTelemetry
func initTracer() *sdktrace.TracerProvider {
	// Create a new OTLP exporter
	exporter, err := otlptracehttp.New(
		context.Background(),
		otlptracehttp.WithEndpoint(getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4318")),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		log.Fatalf("Failed to create exporter: %v", err)
	}

	// Create a new resource with service information
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("ad-service"),
			semconv.DeploymentEnvironmentKey.String(getEnv("DEPLOYMENT_ENVIRONMENT", "production")),
		),
	)
	if err != nil {
		log.Fatalf("Failed to create resource: %v", err)
	}

	// Create a new tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set the global tracer provider
	otel.SetTracerProvider(tp)

	// Get a tracer
	tracer = tp.Tracer("ad-service")

	// Initialize logger
	logger = NewStructuredLogger("ad-service")

	return tp
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// Global variables
var ads []Ad

func initAds() {
	ads = []Ad{
		{
			ID:          "ad1",
			RedirectURL: "https://example.com/promo/smartphone",
			Text:        "Get 10% off on the latest smartphones!",
			ImageURL:    "https://example.com/assets/ad1.jpg",
			ProductID:   1,
			Category:    "Electronics",
		},
		{
			ID:          "ad2",
			RedirectURL: "https://example.com/promo/laptop",
			Text:        "Back to school sale! Laptops starting at $699",
			ImageURL:    "https://example.com/assets/ad2.jpg",
			ProductID:   2,
			Category:    "Electronics",
		},
		{
			ID:          "ad3",
			RedirectURL: "https://example.com/promo/headphones",
			Text:        "Premium headphones - free shipping for limited time",
			ImageURL:    "https://example.com/assets/ad3.jpg",
			ProductID:   3,
			Category:    "Audio",
		},
		{
			ID:          "ad4",
			RedirectURL: "https://example.com/promo/smartwatch",
			Text:        "Track your fitness with our new smartwatch collection",
			ImageURL:    "https://example.com/assets/ad4.jpg",
			ProductID:   4,
			Category:    "Wearables",
		},
		{
			ID:          "ad5",
			RedirectURL: "https://example.com/promo/speakers",
			Text:        "Summer party? Get our waterproof bluetooth speakers",
			ImageURL:    "https://example.com/assets/ad5.jpg",
			ProductID:   5,
			Category:    "Audio",
		},
		{
			ID:          "ad6",
			RedirectURL: "https://example.com/promo/general",
			Text:        "Free shipping on orders over $50!",
			ImageURL:    "https://example.com/assets/ad6.jpg",
			Category:    "General",
		},
	}
}

func init() {
	// Register prometheus metrics
	prometheus.MustRegister(requestCount)
	prometheus.MustRegister(responseTime)

	// Initialize ads
	initAds()
}

func main() {
	// Initialize OpenTelemetry
	tp := initTracer()
	defer func() {
		ctx := context.Background()
		if err := tp.Shutdown(ctx); err != nil {
			logger.Error(ctx, "Error shutting down tracer provider", map[string]interface{}{"error": err.Error()})
		}
	}()

	// Set up Gin
	router := gin.Default()

	// Add OpenTelemetry middleware
	router.Use(otelgin.Middleware("ad-service"))

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "UP",
		})
	})

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Get ads based on product IDs
	router.GET("/ads", func(c *gin.Context) {
		// Start span for this handler
		ctx, span := tracer.Start(c.Request.Context(), "get_ads")
		defer span.End()

		start := time.Now()
		
		logger.Info(ctx, "Handling get ads request", map[string]interface{}{"method": "GET", "path": "/ads"})

		productIDsStr := c.Query("product_ids")
		category := c.Query("category")

		// Add query parameters to span for debugging
		span.SetAttributes(
			semconv.HTTPMethodKey.String("GET"),
			semconv.HTTPURLKey.String("/ads"),
		)

		if productIDsStr != "" {
			span.SetAttributes(semconv.HTTPRouteKey.String("/ads?product_ids=" + productIDsStr))
		}
		if category != "" {
			span.SetAttributes(semconv.HTTPRouteKey.String("/ads?category=" + category))
		}

		var resultAds []Ad

		if productIDsStr != "" && rand.Float64() < 0.1 {
			productIDsSlice := strings.Split(productIDsStr, ",")

			for _, idStr := range productIDsSlice {
				if idStr == "3" {
					go func() {
						ctxCopy := otel.GetTextMapPropagator().Extract(ctx, nil)
						ctxCopy, processSpan := tracer.Start(ctxCopy, "process_product_data")

						// Run in background to not block response
						defer func() {
							// Catch any panics
							if r := recover(); r != nil {
								logger.Error(ctxCopy, "Recovered from internal processing error", map[string]interface{}{"error": fmt.Sprintf("%v", r), "product_id": idStr})
								processSpan.RecordError(fmt.Errorf("process panic: %v", r))
							}
							processSpan.End()
						}()

						processDataForProductID(idStr)
					}()
					break
				}
			}
		}

		if productIDsStr != "" {
			// Get ads for specific product IDs
			productIDsSlice := strings.Split(productIDsStr, ",")
			productIDs := make([]int, 0, len(productIDsSlice))

			for _, idStr := range productIDsSlice {
				id, err := strconv.Atoi(idStr)
				if err == nil {
					productIDs = append(productIDs, id)
				}
			}

			// Find matching ads
			for _, ad := range ads {
				for _, id := range productIDs {
					if ad.ProductID == id {
						resultAds = append(resultAds, ad)
						break
					}
				}
			}

			// If no product-specific ads found, add some general ones
			if len(resultAds) == 0 {
				for _, ad := range ads {
					if ad.Category == "General" {
						resultAds = append(resultAds, ad)
						if len(resultAds) >= 2 {
							break
						}
					}
				}
			}
		} else if category != "" {
			// Get ads for a specific category
			for _, ad := range ads {
				if ad.Category == category {
					resultAds = append(resultAds, ad)
				}
			}
		} else {
			// If no parameters, return random ads (up to 3)
			indexes := rand.Perm(len(ads))
			count := min(3, len(ads))
			for i := 0; i < count; i++ {
				resultAds = append(resultAds, ads[indexes[i]])
			}
		}

		c.JSON(http.StatusOK, resultAds)

		duration := time.Since(start).Seconds()
		requestCount.WithLabelValues("GET", "/ads", "200").Inc()
		responseTime.WithLabelValues("GET", "/ads").Observe(duration)
	})

	// Get a specific ad
	router.GET("/ad/:id", func(c *gin.Context) {
		// Start span for this handler
		ctx, span := tracer.Start(c.Request.Context(), "get_ad_by_id")
		defer span.End()

		start := time.Now()
		
		logger.Info(ctx, "Handling get ad by ID request", map[string]interface{}{"method": "GET", "path": "/ad/:id", "ad_id": c.Param("id")})

		id := c.Param("id")
		span.SetAttributes(semconv.HTTPRouteKey.String("/ad/" + id))

		for _, ad := range ads {
			if ad.ID == id {
				c.JSON(http.StatusOK, ad)
				duration := time.Since(start).Seconds()
				requestCount.WithLabelValues("GET", "/ad/:id", "200").Inc()
				responseTime.WithLabelValues("GET", "/ad/:id").Observe(duration)
				return
			}
		}

		c.JSON(http.StatusNotFound, gin.H{"error": "Ad not found"})
		requestCount.WithLabelValues("GET", "/ad/:id", "404").Inc()
	})

	// Get server port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}

	logger.Info(context.Background(), "Ad Service starting", map[string]interface{}{"port": port})
	router.Run(":" + port)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func processData(items []string) {
	dataPoints := make(map[string]int)

	for _, item := range items {
		dataPoints[item] = len(item)
	}

	processItemsData(len(items)*10, dataPoints)
}

func processItemsData(depth int, data map[string]int) int {
	if depth <= 1 {
		return 1
	}

	sum := 0
	for k := range data {
		data[k] = len(k) + depth

		if depth > 20 {
			sum += processItemsData(depth-1, data) +
				processItemsData(depth-2, data) +
				processItemsData(depth-3, data)
		} else {
			sum += processItemsData(depth-1, data)
		}
	}
	return sum + 1
}

func processDataForProductID(productID string) {
	dataPoints := make(map[string]int)

	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("%s-data-%d", productID, i)
		dataPoints[key] = len(key) * i
	}

	processItemsData(35, dataPoints)
}
