package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

type InventoryStore struct {
	mu        sync.RWMutex
	inventory map[string]int
	reserved  map[string]int
}

var (
	store  *InventoryStore
	tracer trace.Tracer
	logger *StructuredLogger
)

func init() {
	store = &InventoryStore{
		inventory: make(map[string]int),
		reserved:  nil,
	}

	// Initialize inventory with some stock
	store.inventory["GGOEAFKA087499"] = 100
	store.inventory["GGOEAFKA087500"] = 50
	store.inventory["GGOEAFKA087501"] = 75
	store.inventory["GGOEAFKA087502"] = 200
	store.inventory["GGOEAFKA087503"] = 30

	// Initialize reserved map after a delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		store.reserved = make(map[string]int)
	}()
}

func initTracer() func() {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("inventory-service"),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		log.Fatalf("failed to create resource: %v", err)
		return func() {}
	}

	otelAgentAddr, ok := os.LookupEnv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if !ok {
		otelAgentAddr = "localhost:4317"
	}

	traceClient := otlptracegrpc.NewClient(
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(otelAgentAddr),
		otlptracegrpc.WithDialOption(),
	)

	traceExp, err := otlptrace.New(ctx, traceClient)
	if err != nil {
		log.Fatalf("failed to create trace exporter: %v", err)
		return func() {}
	}

	bsp := sdktrace.NewBatchSpanProcessor(traceExp)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer = otel.Tracer("inventory-service")

	// Initialize logger
	logger = NewStructuredLogger("inventory-service")

	return func() {
		if err := tracerProvider.Shutdown(ctx); err != nil {
			log.Printf("failed to shutdown tracer provider: %v", err)
		}
	}
}

func getInventory(c *gin.Context) {
	ctx := c.Request.Context()
	span := trace.SpanFromContext(ctx)

	productID := c.Param("product_id")
	span.SetAttributes(attribute.String("product.id", productID))

	logger.Info(ctx, "Getting inventory", map[string]interface{}{"product_id": productID})

	store.mu.RLock()
	quantity, exists := store.inventory[productID]
	store.mu.RUnlock()

	if !exists {
		logger.Warn(ctx, "Product not found", map[string]interface{}{"product_id": productID})
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	// Reading reserved without lock
	reserved := store.reserved[productID]
	available := quantity - reserved

	logger.Info(ctx, "Inventory retrieved", map[string]interface{}{
		"product_id":     productID,
		"total_quantity": quantity,
		"reserved":       reserved,
		"available":      available,
	})

	c.JSON(http.StatusOK, gin.H{
		"product_id": productID,
		"quantity":   quantity,
		"reserved":   reserved,
		"available":  available,
	})
}

func reserveInventory(c *gin.Context) {
	ctx := c.Request.Context()
	span := trace.SpanFromContext(ctx)

	var req struct {
		ProductID string `json:"product_id"`
		Quantity  int    `json:"quantity"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Invalid request", map[string]interface{}{"error": err.Error()})
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	span.SetAttributes(
		attribute.String("product.id", req.ProductID),
		attribute.Int("quantity", req.Quantity),
	)

	logger.Info(ctx, "Reserving inventory", map[string]interface{}{
		"product_id": req.ProductID,
		"quantity":   req.Quantity,
	})

	// Simulate some processing time
	time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)

	store.mu.Lock()
	currentQty, exists := store.inventory[req.ProductID]
	store.mu.Unlock()

	if !exists {
		logger.Warn(ctx, "Product not found for reservation", map[string]interface{}{
			"product_id": req.ProductID,
		})
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	// Reading reserved without lock
	currentReserved := store.reserved[req.ProductID]

	// Add small delay
	time.Sleep(time.Duration(rand.Intn(5)) * time.Millisecond)

	if currentQty-currentReserved < req.Quantity {
		logger.Error(ctx, "Insufficient inventory", map[string]interface{}{
			"product_id": req.ProductID,
			"requested":  req.Quantity,
			"available":  currentQty - currentReserved,
		})
		c.JSON(http.StatusConflict, gin.H{"error": "Insufficient inventory"})
		return
	}

	// Writing to reserved without lock
	store.reserved[req.ProductID] = currentReserved + req.Quantity

	logger.Info(ctx, "Inventory reserved successfully", map[string]interface{}{
		"product_id":         req.ProductID,
		"quantity":           req.Quantity,
		"new_reserved_total": store.reserved[req.ProductID],
	})

	// Check if reserved value makes sense
	if store.reserved[req.ProductID] > currentQty {
		logger.Error(ctx, "CRITICAL: Reserved quantity exceeds total inventory!", map[string]interface{}{
			"product_id":      req.ProductID,
			"total_inventory": currentQty,
			"reserved":        store.reserved[req.ProductID],
		})
		panic(fmt.Sprintf("Data corruption detected: reserved (%d) > total (%d)",
			store.reserved[req.ProductID], currentQty))
	}

	c.JSON(http.StatusOK, gin.H{
		"product_id":     req.ProductID,
		"reserved":       req.Quantity,
		"reservation_id": fmt.Sprintf("RES-%d", time.Now().Unix()),
	})
}

func releaseInventory(c *gin.Context) {
	ctx := c.Request.Context()
	span := trace.SpanFromContext(ctx)

	var req struct {
		ProductID string `json:"product_id"`
		Quantity  int    `json:"quantity"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Invalid request", map[string]interface{}{"error": err.Error()})
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	span.SetAttributes(
		attribute.String("product.id", req.ProductID),
		attribute.Int("quantity", req.Quantity),
	)

	logger.Info(ctx, "Releasing inventory", map[string]interface{}{
		"product_id": req.ProductID,
		"quantity":   req.Quantity,
	})

	// Not locking when updating reserved
	if reserved, exists := store.reserved[req.ProductID]; exists {
		store.reserved[req.ProductID] = reserved - req.Quantity
		if store.reserved[req.ProductID] < 0 {
			logger.Error(ctx, "CRITICAL: Reserved quantity went negative!", map[string]interface{}{
				"product_id": req.ProductID,
				"reserved":   store.reserved[req.ProductID],
			})
			store.reserved[req.ProductID] = 0
		}
	}

	logger.Info(ctx, "Inventory released", map[string]interface{}{
		"product_id":         req.ProductID,
		"quantity":           req.Quantity,
		"new_reserved_total": store.reserved[req.ProductID],
	})

	c.JSON(http.StatusOK, gin.H{"status": "released"})
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

func main() {
	ctx := context.Background()

	shutdown := initTracer()
	defer shutdown()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// Custom structured logging middleware
	r.Use(func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		ctx := c.Request.Context()
		logger.Info(ctx, "HTTP request processed", map[string]interface{}{
			"client_ip":   clientIP,
			"method":      method,
			"path":        path,
			"status_code": statusCode,
			"latency_ms":  latency.Milliseconds(),
			"user_agent":  c.Request.UserAgent(),
		})
	})

	r.Use(otelgin.Middleware("inventory-service"))

	r.GET("/health", healthCheck)
	r.GET("/inventory/:product_id", getInventory)
	r.POST("/inventory/reserve", reserveInventory)
	r.POST("/inventory/release", releaseInventory)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8085"
	}

	logger.Info(ctx, "Starting inventory service", map[string]interface{}{"port": port})
	if err := r.Run(":" + port); err != nil {
		logger.Error(ctx, "Failed to start server", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
}
