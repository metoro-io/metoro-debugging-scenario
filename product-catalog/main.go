package main

import (
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
var faultConfig struct {
	Enabled        bool      `json:"enabled"`
	LatencyMs      int       `json:"latency_ms"`
	ErrorRate      float64   `json:"error_rate"`
	ExpirationTime time.Time `json:"expiration_time"`
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

	// Initialize fault configuration
	faultConfig.Enabled = false
	faultConfig.LatencyMs = 0
	faultConfig.ErrorRate = 0
}

// Middleware for handling faults
func faultMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip fault injection for metrics and health endpoints
		if c.Request.URL.Path == "/metrics" || c.Request.URL.Path == "/health" {
			c.Next()
			return
		}

		// Check if fault injection is enabled and not expired
		if faultConfig.Enabled && time.Now().Before(faultConfig.ExpirationTime) {
			// Latency injection
			if faultConfig.LatencyMs > 0 {
				time.Sleep(time.Duration(faultConfig.LatencyMs) * time.Millisecond)
			}

			// Error injection
			if faultConfig.ErrorRate > 0 {
				// Generate a random number between 0 and 1
				if rand.Float64() < faultConfig.ErrorRate {
					c.JSON(http.StatusInternalServerError, gin.H{
						"error": "Injected fault: server error",
					})
					c.Abort()
					return
				}
			}
		}

		c.Next()
	}
}

func main() {
	// Set up Gin
	router := gin.Default()

	// Add middleware
	router.Use(faultMiddleware())

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
		start := time.Now()

		category := c.Query("category")
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

		c.JSON(http.StatusOK, filteredProducts)

		duration := time.Since(start).Seconds()
		requestCount.WithLabelValues("GET", "/products", "200").Inc()
		responseTime.WithLabelValues("GET", "/products").Observe(duration)
	})

	// Get a specific product
	router.GET("/product/:id", func(c *gin.Context) {
		start := time.Now()

		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
			requestCount.WithLabelValues("GET", "/product/:id", "400").Inc()
			return
		}

		for _, p := range products {
			if p.ID == id {
				c.JSON(http.StatusOK, p)
				duration := time.Since(start).Seconds()
				requestCount.WithLabelValues("GET", "/product/:id", "200").Inc()
				responseTime.WithLabelValues("GET", "/product/:id").Observe(duration)
				return
			}
		}

		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		requestCount.WithLabelValues("GET", "/product/:id", "404").Inc()
	})

	// Fault injection endpoint
	router.POST("/fault", func(c *gin.Context) {
		var request struct {
			Enabled     bool    `json:"enabled"`
			LatencyMs   int     `json:"latency_ms"`
			ErrorRate   float64 `json:"error_rate"`
			DurationSec int     `json:"duration_sec"`
		}

		if err := c.BindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		faultConfig.Enabled = request.Enabled
		faultConfig.LatencyMs = request.LatencyMs
		faultConfig.ErrorRate = request.ErrorRate
		faultConfig.ExpirationTime = time.Now().Add(time.Duration(request.DurationSec) * time.Second)

		c.JSON(http.StatusOK, gin.H{
			"status": "Fault configuration updated",
			"config": faultConfig,
		})
	})

	// Get server port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("Product Catalog Service starting on port %s...\n", port)
	router.Run(":" + port)
}
