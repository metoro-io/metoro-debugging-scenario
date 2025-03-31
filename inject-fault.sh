#!/bin/bash

SERVICE=$1
FAULT_TYPE=$2
FAULT_VALUE=$3
DURATION=$4

# Default values
if [ -z "$SERVICE" ]; then
    echo "Usage: $0 <service> <fault-type> <value> <duration-sec>"
    echo "Services: gateway, product-catalog, currency-service, ad-service, checkout-service"
    echo "Fault types: latency, error, both"
    echo "Value: latency in ms or error rate (0.0-1.0)"
    echo "Duration: seconds"
    exit 1
fi

if [ -z "$FAULT_TYPE" ]; then
    FAULT_TYPE="both"
fi

if [ -z "$FAULT_VALUE" ]; then
    FAULT_VALUE=500
fi

if [ -z "$DURATION" ]; then
    DURATION=60
fi

# Service port mapping
case $SERVICE in
    "gateway")
        PORT=8080
        ;;
    "product-catalog")
        PORT=8081
        ;;
    "currency-service")
        PORT=8082
        ;;
    "ad-service")
        PORT=8083
        ;;
    "checkout-service")
        PORT=8084
        ;;
    *)
        echo "Unknown service: $SERVICE"
        exit 1
        ;;
esac

# Fault config based on type
LATENCY=0
ERROR_RATE=0.0

case $FAULT_TYPE in
    "latency")
        LATENCY=$FAULT_VALUE
        ;;
    "error")
        ERROR_RATE=$FAULT_VALUE
        ;;
    "both")
        LATENCY=$FAULT_VALUE
        ERROR_RATE=0.3
        ;;
    *)
        echo "Unknown fault type: $FAULT_TYPE"
        exit 1
        ;;
esac

# Create fault injection payload
PAYLOAD="{\"enabled\": true, \"latency_ms\": $LATENCY, \"error_rate\": $ERROR_RATE, \"duration_sec\": $DURATION}"

echo "Injecting fault into $SERVICE (Port $PORT):"
echo "  Fault type: $FAULT_TYPE"
echo "  Latency: $LATENCY ms"
echo "  Error rate: $ERROR_RATE"
echo "  Duration: $DURATION seconds"

# Send fault injection request
curl -X POST "http://localhost:$PORT/fault" \
     -H "Content-Type: application/json" \
     -d "$PAYLOAD"

echo "" 