# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a microservices-based e-commerce demo application designed for debugging practice and demonstrating observability patterns. It consists of multiple services written in Go and Python that communicate via REST APIs.
The purpose of these services is to have bugs that can be debugged and to demonstrate observability features such as distributed tracing, structured logging, and metrics collection.

## Architecture

### Services
- **gateway** (Python/Flask): API gateway, front-facing service on port 8080
- **product-catalog** (Go/Gin): Product management service on port 8081
- **currency-service** (Python/Flask): Currency conversion on port 8082
- **ad-service** (Go/Gin): Advertisement service on port 8083
- **checkout-service** (Python/Flask): Order processing on port 8084
- **load-generator** (Python): Traffic simulation service

### Key Technical Details
- All services use OpenTelemetry for distributed tracing
- Structured JSON logging with automatic trace context injection
- Prometheus metrics exposed on each service
- Multi-architecture Docker images (amd64/arm64)

## Essential Commands

### Development
```bash
# Run all services locally
docker-compose up

# Run with pre-built images from Quay.io
docker-compose -f docker-compose-quay.yaml up

# Build all Docker images locally
./build.sh

# Build and push multi-arch images to registry
./build-and-push.sh
```

### Testing
```bash
# Go services (run in service directory)
go test ./...

# Python services (run in service directory)
pytest test_app.py

# Test structured logging implementation
./test_structured_logging.sh
```

### Individual Service Development
```bash
# Go services (e.g., product-catalog)
cd product-catalog
go mod download
go run main.go

# Python services (e.g., gateway)
cd gateway
pip install -r requirements.txt
python app.py
```

### Kubernetes Deployment
```bash
# Deploy to Kubernetes
helm install microservice-demo ./helm-chart

# Update deployment
helm upgrade microservice-demo ./helm-chart

# Uninstall
helm uninstall microservice-demo