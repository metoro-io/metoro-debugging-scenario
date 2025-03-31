package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	// Set up routes
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "UP",
		})
	})

	router.GET("/products", func(c *gin.Context) {
		c.JSON(http.StatusOK, products)
	})

	router.GET("/product/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
			return
		}

		for _, p := range products {
			if p.ID == id {
				c.JSON(http.StatusOK, p)
				return
			}
		}

		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
	})

	return router
}

func TestHealthCheck(t *testing.T) {
	router := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	if response["status"] != "UP" {
		t.Errorf("Expected status 'UP', got %s", response["status"])
	}
}

func TestGetProducts(t *testing.T) {
	// Initialize products for testing
	initProducts()

	router := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/products", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var response []Product
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	if len(response) != len(products) {
		t.Errorf("Expected %d products, got %d", len(products), len(response))
	}
}

func TestGetProduct(t *testing.T) {
	// Initialize products for testing
	initProducts()

	router := setupRouter()

	// Test valid product ID
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/product/1", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var product Product
	err := json.Unmarshal(w.Body.Bytes(), &product)
	if err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	if product.ID != 1 {
		t.Errorf("Expected product ID 1, got %d", product.ID)
	}

	// Test invalid product ID
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/product/999", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", http.StatusNotFound, w.Code)
	}
}
