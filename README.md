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

## Architecture

The services interact in the following manner:
1. The gateway service receives all external requests
2. It routes requests to the appropriate backend services
3. Some services call other services (e.g. checkout calls product catalog and currency services)
4. The load generator creates simulated user traffic

## Running Locally with Docker Compose

To run the entire system locally:

```
# Build and start all services
docker-compose up --build
```

The services will be available at:
- Gateway: http://localhost:8080
- Product Catalog: http://localhost:8081
- Currency Service: http://localhost:8082
- Ad Service: http://localhost:8083
- Checkout Service: http://localhost:8084
- Load Generator Metrics: http://localhost:8099/metrics

### API Endpoints

- List products: `GET /products`
- View product: `GET /product/{id}`
- Checkout: `POST /checkout`
- Currency conversion: `GET /convert?from=USD&to=EUR&amount=10`
- Advertisements: `GET /ads?product_ids=1,2,3`

## Deploying with Helm

A Helm chart is provided for Kubernetes deployment:

```
# From the project root
cd helm-chart

# Install the chart
helm install microservice-demo .
```

### Enabling Fault Injection

The Helm chart supports fault injection for debugging purposes:

```
# Enable faults for the product catalog service
helm upgrade microservice-demo . --set productCatalog.faults.enabled=true \
  --set productCatalog.faults.latencyMs=500 \
  --set productCatalog.faults.errorRate=0.3 \
  --set productCatalog.faults.durationSec=300
```

You can inject the following faults:
- **Latency**: Add artificial delay to responses
- **Errors**: Return error responses at a specified rate
- **Both**: Combine latency and errors

## Debugging Techniques

This demo is designed to showcase various debugging techniques:

1. **Distributed Tracing**: Trace requests across multiple services
2. **Error Injection**: Simulate failures to test resiliency
3. **Latency Injection**: Simulate slow services to test timeout behavior
4. **Metrics Analysis**: Use Prometheus metrics to identify bottlenecks

## Common Issues for Debugging Practice

Some interesting scenarios to debug:
1. Network latency affecting checkout service
2. Error rate spikes in currency service
3. Resource contention in the ad service
4. Bad requests from the load generator

## Development

### Requirements
- Docker and Docker Compose for local development
- Kubernetes and Helm for deployment
- Go 1.17+ and Python 3.9+ for local development

### Building Individual Services

To build and run individual services:

```
# Gateway
cd gateway
pip install -r requirements.txt
python app.py

# Product Catalog
cd product-catalog
go mod download
go run main.go
``` 