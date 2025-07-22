# Inventory Service

A Go microservice that manages product inventory for the e-commerce demo application.

## Overview

The inventory service provides REST APIs for:
- Checking product inventory levels
- Reserving inventory for orders
- Releasing reserved inventory

## API Endpoints

- `GET /inventory/:product_id` - Get inventory status for a product
- `POST /inventory/reserve` - Reserve inventory for an order
- `POST /inventory/release` - Release previously reserved inventory
- `GET /health` - Health check endpoint

## Features

- Structured JSON logging with trace context
- OpenTelemetry instrumentation for distributed tracing
- Prometheus metrics
- Concurrent request handling

## Running Locally

```bash
go mod download
go run .
```

The service runs on port 8085 by default (configurable via PORT environment variable).

## Testing

Use the included test script to simulate concurrent load:
```bash
python test_race_condition.py
```