package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

// Prometheus metrics
var (
	requestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "product_catalog_request_count",
			Help: "Number of requests received by the product catalog service",
		},
		[]string{"method", "endpoint", "status"},
	)
	responseTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "product_catalog_response_time",
			Help:    "Response time of the product catalog service",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)
)

// Product represents a product in the catalog
type Product struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       float64  `json:"price"`
	Currency    string   `json:"currency"`
	ImageURL    string   `json:"image_url"`
	Categories  []string `json:"categories"`
}

// Global variables
var products []Product

var tracer trace.Tracer

func initOTelSDK(ctx context.Context) (*sdktrace.TracerProvider, error) {
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otlpEndpoint == "" {
		otlpEndpoint = "http://otel-collector:4318/v1/traces"
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(otlpEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	resources, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("product-catalog"),
			attribute.String("deployment.environment", os.Getenv("DEPLOYMENT_ENVIRONMENT")),
		),
	)
	if err != nil {
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resources),
	)
	otel.SetTracerProvider(tracerProvider)
	tracer = otel.Tracer("product-catalog")

	return tracerProvider, nil
}

func initProducts() {
	products = []Product{
		{
			ID:          1,
			Name:        "Smartphone Model X",
			Description: "Latest smartphone with advanced features",
			Price:       699.99,
			Currency:    "USD",
			ImageURL:    "https://example.com/smartphone.jpg",
			Categories:  []string{"Electronics", "Phones"},
		},
		{
			ID:          2,
			Name:        "Laptop Pro",
			Description: "High-performance laptop for professionals",
			Price:       1299.99,
			Currency:    "USD",
			ImageURL:    "https://example.com/laptop.jpg",
			Categories:  []string{"Electronics", "Computers"},
		},
		{
			ID:          3,
			Name:        "Wireless Headphones",
			Description: "Premium noise-cancelling headphones",
			Price:       249.99,
			Currency:    "USD",
			ImageURL:    "https://example.com/headphones.jpg",
			Categories:  []string{"Electronics", "Audio"},
		},
		{
			ID:          4,
			Name:        "Smart Watch Series 5",
			Description: "Fitness and health tracking smartwatch",
			Price:       349.99,
			Currency:    "USD",
			ImageURL:    "https://example.com/smartwatch.jpg",
			Categories:  []string{"Electronics", "Wearables"},
		},
		{
			ID:          5,
			Name:        "Bluetooth Speaker",
			Description: "Portable waterproof bluetooth speaker",
			Price:       79.99,
			Currency:    "USD",
			ImageURL:    "https://example.com/speaker.jpg",
			Categories:  []string{"Electronics", "Audio"},
		},
	}
}

func init() {
	// Register prometheus metrics
	prometheus.MustRegister(requestCount)
	prometheus.MustRegister(responseTime)

	// Initialize products
	initProducts()
}

func main() {
	ctx := context.Background()

	// Initialize OpenTelemetry
	tracerProvider, err := initOTelSDK(ctx)
	if err != nil {
		log.Fatalf("Error initializing OpenTelemetry: %v", err)
	}
	defer func() {
		if err := tracerProvider.Shutdown(ctx); err != nil {
			log.Fatalf("Error shutting down tracer provider: %v", err)
		}
	}()

	// Set up Gin
	router := gin.Default()

	// Add OpenTelemetry middleware
	router.Use(otelgin.Middleware("product-catalog"))

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "UP",
		})
	})

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Get all products
	router.GET("/products", func(c *gin.Context) {
		_, span := tracer.Start(c.Request.Context(), "get_products")
		defer span.End()

		start := time.Now()

		category := c.Query("category")
		if category != "" {
			span.SetAttributes(attribute.String("category", category))
		}

		var filteredProducts []Product

		if category != "" {
			for _, p := range products {
				for _, cat := range p.Categories {
					if cat == category {
						filteredProducts = append(filteredProducts, p)
						break
					}
				}
			}
		} else {
			filteredProducts = products
		}

		span.SetAttributes(attribute.Int("products_count", len(filteredProducts)))

		c.JSON(http.StatusOK, filteredProducts)

		duration := time.Since(start).Seconds()
		requestCount.WithLabelValues("GET", "/products", "200").Inc()
		responseTime.WithLabelValues("GET", "/products").Observe(duration)
	})

	// Get a specific product
	router.GET("/product/:id", func(c *gin.Context) {
		_, span := tracer.Start(c.Request.Context(), "get_product")
		defer span.End()

		start := time.Now()

		idStr := c.Param("id")
		span.SetAttributes(attribute.String("product_id", idStr))

		id, err := strconv.Atoi(idStr)

		if err != nil {
			span.SetAttributes(attribute.String("error", "invalid_product_id"))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
			requestCount.WithLabelValues("GET", "/product/:id", "400").Inc()
			return
		}

		for _, p := range products {
			if p.ID == id {
				span.SetAttributes(
					attribute.String("product_name", p.Name),
					attribute.Float64("price", p.Price),
				)
				c.JSON(http.StatusOK, p)
				duration := time.Since(start).Seconds()
				requestCount.WithLabelValues("GET", "/product/:id", "200").Inc()
				responseTime.WithLabelValues("GET", "/product/:id").Observe(duration)
				return
			}
		}

		span.SetAttributes(attribute.String("error", "product_not_found"))
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		requestCount.WithLabelValues("GET", "/product/:id", "404").Inc()
	})

	// Get server port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("Product Catalog Service starting on port %s...\n", port)
	router.Run(":" + port)
}
