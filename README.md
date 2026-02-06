# Microservice Debugging Demo

This project provides a simple microservice-based shopping website for demonstrating microservice debugging techniques. The system is composed of multiple services written in Go and Python that communicate via HTTP.

## Services

The system consists of the following services:

- **Gateway (Python)**: Front-facing API gateway that routes requests to appropriate services
- **Product Catalog (Go)**: Manages product information
- **Currency Service (Python)**: Handles currency conversion
- **Ad Service (Go)**: Provides advertisements
- **Checkout Service (Python)**: Processes orders
- **Load Generator (Python)**: Simulates user traffic
- **Instabook Cache (Go)**: Session cache with admin UI for token toggle
- **Instabook (Go)**: Booking service that calls the cache with Bearer token authentication
- **Load Generator Instabook (Python)**: Simulates booking traffic

## Architecture

The services interact in the following manner:
1. The gateway service receives all external requests
2. It routes requests to the appropriate backend services
3. Some services call other services (e.g. checkout calls product catalog and currency services)
4. The load generator creates simulated user traffic

## Running Locally with Docker Compose

### Building Images Locally

To build and run all services locally:

```bash
# Build and start all services with local images
docker-compose up --build
```

### Using Pre-built Images from Quay.io

You can also use the pre-built images from Quay.io:

```bash
# Run using images from Quay.io
docker-compose -f docker-compose-quay.yaml up
```

The services will be available at:
- Gateway: http://localhost:8080
- Product Catalog: http://localhost:8081
- Currency Service: http://localhost:8082
- Ad Service: http://localhost:8083
- Checkout Service: http://localhost:8084
- Instabook Cache: http://localhost:8086
- Instabook Cache Admin UI: http://localhost:8086/admin
- Instabook: http://localhost:8087
- Load Generator Metrics: http://localhost:8099/metrics
- Load Generator Instabook Metrics: http://localhost:8098/metrics

### API Endpoints

- List products: `GET /products`
- View product: `GET /product/{id}`
- Checkout: `POST /checkout`
- Currency conversion: `GET /convert?from=USD&to=EUR&amount=10`
- Advertisements: `GET /ads?product_ids=1,2,3`

## Instabook Debugging Scenario

The instabook services form a chain for demonstrating authentication failure debugging:

```
load-generator-instabook → instabook (8087) → instabook-cache (8086)
```

### How it works

1. **Normal flow**: The load generator creates and reads booking sessions through the instabook service, which calls the cache with a Bearer token.

2. **Token toggle**: Visit http://localhost:8086/admin to toggle API token authentication on/off.

3. **Failure scenario**: When token authentication is disabled:
   - instabook-cache returns 401 for all `/cache/*` requests
   - instabook receives the 401 and returns 500 with "Internal service authentication failure"
   - load-generator-instabook logs errors for the 500 responses

This scenario demonstrates debugging distributed authentication failures across service boundaries.

## Building and Pushing to Container Registry

The application uses a single repository `quay.io/metoro/metoro-demo-applications` with different tags for each service, following the pattern `<service>-<version>` (e.g., `gateway-1.0.1`).

### Multi-Architecture Support

All images are built for both `amd64` (x86_64) and `arm64` (Apple Silicon/M1/M2) architectures, allowing them to run natively on different platforms.

### Building and Pushing Images

To build all service images and push them to the Quay.io registry:

```bash
# Login to Quay.io first
docker login quay.io

# Ensure Docker buildx is installed and set up
docker buildx version

# Build and push multi-architecture images for all services
./build-and-push.sh
```

This script will:
1. Build each service image for both amd64 and arm64 architectures
2. Tag each image with version-specific (e.g., `gateway-1.0.1`) and latest tags (e.g., `gateway-latest`)
3. Push all tags to the Quay.io registry

The resulting images will be:
- `quay.io/metoro/metoro-demo-applications:gateway-1.0.1`
- `quay.io/metoro/metoro-demo-applications:product-catalog-1.0.1`
- `quay.io/metoro/metoro-demo-applications:currency-service-1.0.1`
- `quay.io/metoro/metoro-demo-applications:ad-service-1.0.1`
- `quay.io/metoro/metoro-demo-applications:checkout-service-1.0.1`
- `quay.io/metoro/metoro-demo-applications:load-generator-1.0.1`
- `quay.io/metoro/metoro-demo-applications:instabook-cache-1.0.1`
- `quay.io/metoro/metoro-demo-applications:instabook-1.0.1`
- `quay.io/metoro/metoro-demo-applications:load-generator-instabook-1.0.1`

And their corresponding `-latest` versions.

### Version History

- **1.0.1**: Fixed Werkzeug compatibility issues in Python services by pinning werkzeug==2.0.3
- **1.0.0**: Initial release

## Deploying with Helm

A Helm chart is provided for Kubernetes deployment:

```bash
# From the project root
cd helm-chart

# Install the chart (using Quay.io images by default)
helm install microservice-demo .

# Or specify a specific version
helm install microservice-demo . --set global.tag=1.0.1
```

### Image Pull Policy

All containers in the Helm chart use `imagePullPolicy: Always` to ensure the latest images are always pulled when pods are created. This is especially important when using the "latest" tag or when rapidly iterating on development.

### Requirements
- Docker and Docker Compose for local development
- Docker Buildx for multi-architecture builds
- Kubernetes and Helm for deployment
- Go 1.17+ and Python 3.9+ for local development

### Building Individual Services

To build and run individual services:

```bash
# Gateway
cd gateway
pip install -r requirements.txt
python app.py

# Product Catalog
cd product-catalog
go mod download
go run main.go
``` 