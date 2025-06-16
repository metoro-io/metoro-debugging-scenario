# Structured Logging Implementation

This document describes the structured logging implementation across all microservices in the system.

## Overview

All services now use structured logging with automatic trace context injection from OpenTelemetry. This ensures that all log entries include:
- `trace_id`: The distributed trace ID from OpenTelemetry
- `span_id`: The current span ID from OpenTelemetry
- Structured fields for better log analysis

## Implementation Details

### Go Services (ad-service, product-catalog)

The Go services use a custom `StructuredLogger` that:
- Extracts trace context from OpenTelemetry spans
- Outputs JSON-formatted logs to stdout
- Includes service name, timestamp, log level, and custom fields

Example log entry:
```json
{
  "timestamp": "2024-01-10T10:30:45Z",
  "level": "INFO",
  "service_name": "ad-service",
  "trace_id": "7c3f8b9d4e5f6a7b8c9d0e1f2a3b4c5d",
  "span_id": "a1b2c3d4e5f67890",
  "message": "Handling get ads request",
  "fields": {
    "method": "GET",
    "path": "/ads"
  }
}
```

Usage in Go services:
```go
logger.Info(ctx, "Processing request", map[string]interface{}{
    "user_id": userID,
    "action": "checkout",
})
```

### Python Services (gateway, checkout-service, currency-service)

The Python services use a custom `StructuredLogger` class that:
- Extracts trace context from the current OpenTelemetry span
- Outputs JSON-formatted logs to stdout
- Supports keyword arguments for additional fields

Example log entry:
```json
{
  "timestamp": "2024-01-10T10:30:45.123456Z",
  "level": "INFO",
  "service_name": "gateway",
  "trace_id": "7c3f8b9d4e5f6a7b8c9d0e1f2a3b4c5d",
  "span_id": "a1b2c3d4e5f67890",
  "message": "Handling checkout request",
  "fields": {
    "method": "POST",
    "path": "/checkout",
    "items_count": 3
  }
}
```

Usage in Python services:
```python
logger.info("Processing order", order_id=order_id, total=order_total)
logger.error("Service error", error=str(e), service="product-catalog")
```

## Benefits

1. **Trace Correlation**: All logs automatically include trace and span IDs, making it easy to correlate logs with distributed traces
2. **Structured Data**: JSON format enables easy parsing and analysis in log aggregation systems
3. **Consistent Format**: All services use the same log format regardless of language
4. **Enhanced Debugging**: Trace IDs allow following a request across all services

## Integration with Observability Stack

The structured logs are designed to work with:
- **OpenTelemetry Collector**: Collects and exports traces
- **Log Aggregation Systems**: Can parse JSON logs and index trace_id fields
- **Trace Visualization Tools**: Can link logs to traces using trace_id

## Log Levels

All services support the following log levels:
- `DEBUG`: Detailed information for debugging
- `INFO`: General informational messages
- `WARN`/`WARNING`: Warning messages for potentially harmful situations
- `ERROR`: Error messages for serious problems

## Next Steps

To fully utilize the structured logging:
1. Configure your log aggregation system to parse JSON logs
2. Set up indexing on `trace_id` and `span_id` fields
3. Configure trace-to-log correlation in your observability platform
4. Use trace IDs to debug issues across the entire system