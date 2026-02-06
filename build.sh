#!/usr/bin/env bash

echo "Building all services..."

# Function to build a service
build_service() {
    echo "Building $1 service..."
    docker build -t $1:latest ./$1
    if [ $? -eq 0 ]; then
        echo "✅ Successfully built $1 service"
    else
        echo "❌ Failed to build $1 service"
        exit 1
    fi
}

# Build each service
build_service "gateway"
build_service "product-catalog"
build_service "currency-service"
build_service "ad-service"
build_service "checkout-service"
build_service "load-generator"
build_service "inventory-service"

echo "All services built successfully!"
echo "To run the services, use: docker-compose up" 