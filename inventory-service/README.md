# Inventory Service

A Go microservice that manages product inventory with built-in race condition bug.

## Bug Description

The service has a race condition in the inventory reservation logic:

1. The `reserved` map is accessed without proper synchronization in several places:
   - Line 126: Reading reserved without lock
   - Line 144: Writing to reserved without lock  
   - Line 171: Another race condition when releasing inventory

2. Under concurrent load, this causes:
   - Data corruption where reserved quantities exceed total inventory
   - Panic with error: "Data corruption detected: reserved (X) > total (Y)"
   - 500 errors returned to clients

## How to Trigger

Run multiple concurrent reservation requests:

```python
# Use the included test script
python test_race_condition.py
```

Or manually with curl:
```bash
# Send concurrent requests
for i in {1..50}; do
  curl -X POST http://localhost:8085/inventory/reserve \
    -H "Content-Type: application/json" \
    -d '{"product_id":"GGOEAFKA087499","quantity":10}' &
done
```

## Log Output

When the bug triggers, you'll see logs like:
```json
{
  "level": "ERROR",
  "msg": "CRITICAL: Reserved quantity exceeds total inventory!",
  "product_id": "GGOEAFKA087499",
  "total_inventory": 100,
  "reserved": 115,
  "trace_id": "abc123..."
}
```

Followed by a panic and 500 error.