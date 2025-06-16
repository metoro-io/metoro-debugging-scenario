#!/bin/bash

echo "Testing structured logging implementation..."

# Test Go service logs
echo "Testing ad-service logger output:"
cd ad-service
go run . &
AD_PID=$!
sleep 2
curl -s http://localhost:8083/health
curl -s "http://localhost:8083/ads?product_ids=1,2,3"
kill $AD_PID
cd ..

echo ""
echo "Testing product-catalog logger output:"
cd product-catalog
go run . &
PC_PID=$!
sleep 2
curl -s http://localhost:8081/health
curl -s http://localhost:8081/products
kill $PC_PID
cd ..

echo ""
echo "The services should have printed structured JSON logs with trace_id and span_id fields."
echo "When integrated with OpenTelemetry collector, these logs will automatically include trace context."