package main

import (
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
)

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
	// Set up Gin
	router := gin.Default()

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
		start := time.Now()

		productIDsStr := c.Query("product_ids")
		category := c.Query("category")

		var resultAds []Ad

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
		start := time.Now()

		id := c.Param("id")

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

	log.Printf("Ad Service starting on port %s...\n", port)
	router.Run(":" + port)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
