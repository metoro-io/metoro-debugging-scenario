# Microservice Debugging Scenarios

This document outlines several debugging scenarios that can be simulated with this microservice system. These scenarios are designed to demonstrate common issues in microservice architectures and how to debug them.

## Scenario 1: Latency Cascade

**Description**: One service experiences high latency, which cascades to other services and eventually affects the user-facing API.

**Setup**:
1. Inject latency into the Currency Service:
   ```bash
   ./inject-fault.sh currency-service latency 1000 300
   ```

**Symptoms**:
- Product detail pages load slowly (when currency conversion is enabled)
- Checkout process has high latency
- Load testing shows increasing response times across multiple services

**Debugging Steps**:
1. Identify the bottleneck using metrics
2. Check the Currency Service metrics
3. Examine requests from other services to Currency Service
4. Fix by improving caching or optimizing the Currency Service

## Scenario 2: Error Propagation

**Description**: One service starts returning errors, which propagates through the system.

**Setup**:
1. Inject errors into the Product Catalog Service:
   ```bash
   ./inject-fault.sh product-catalog error 0.5 300
   ```

**Symptoms**:
- Random failures in product listings
- Checkout process fails intermittently
- Error rate increases in all services

**Debugging Steps**:
1. Check error rates and status codes in logs
2. Trace failed requests through the system
3. Identify the Product Catalog as the source of errors
4. Fix by adding better error handling and retries

## Scenario 3: Resource Contention

**Description**: The Ad Service takes too many resources, affecting other services.

**Setup**:
1. Run the system with limited resources
2. Inject high latency in Ad Service causing resource buildup:
   ```bash
   ./inject-fault.sh ad-service both 800 600
   ```

**Symptoms**:
- Ad Service consumes excessive resources
- Other services become slower due to resource contention
- Eventually, the system might experience OOM errors

**Debugging Steps**:
1. Monitor resource usage across services
2. Identify the Ad Service as consuming excessive resources
3. Examine the Ad Service behavior under load
4. Fix by implementing proper resource limits and optimizing resource usage

## Scenario 4: Deadlock Between Services

**Description**: Two services enter a circular dependency, causing requests to hang.

**Setup**:
1. Inject latency into Product Catalog Service:
   ```bash
   ./inject-fault.sh product-catalog latency 500 300
   ```
2. Simultaneously inject latency into Checkout Service:
   ```bash
   ./inject-fault.sh checkout-service latency 500 300
   ```

**Symptoms**:
- Requests that involve both services hang indefinitely
- Connection pools get exhausted
- Timeouts occur after a long delay

**Debugging Steps**:
1. Identify hung requests using metrics
2. Trace the request flow between services
3. Detect the circular dependency
4. Fix by implementing proper timeouts and circuit breakers

## Scenario 5: Cache Inconsistency

**Description**: The Product Catalog Service has inconsistent data across instances.

**Setup**:
1. Run multiple instances of the Product Catalog Service
2. Manually update product data in one instance

**Symptoms**:
- Users see different product information on different requests
- Checkout might use incorrect pricing

**Debugging Steps**:
1. Compare product data across instances
2. Check caching mechanisms
3. Review data update procedures
4. Fix by implementing proper cache invalidation

## Scenario 6: Network Partition

**Description**: Network issues cause some services to be unreachable.

**Setup**:
1. Use network policy in Kubernetes to isolate a service:
   ```bash
   kubectl apply -f network-partition-policy.yaml
   ```

**Symptoms**:
- Some requests fail while others succeed
- Intermittent timeouts
- Retry logic might mask the issue

**Debugging Steps**:
1. Check connectivity between services
2. Use ping/curl tests between services
3. Examine network policies
4. Fix by implementing proper fallbacks and resilience patterns

## How to Run Scenarios with Helm

For Kubernetes environments, you can enable fault injection through Helm:

```bash
# Enable fault in Product Catalog service
helm upgrade microservice-demo ./helm-chart \
  --set productCatalog.faults.enabled=true \
  --set productCatalog.faults.latencyMs=500 \
  --set productCatalog.faults.errorRate=0.3 \
  --set productCatalog.faults.durationSec=300
```

This makes it easy to inject faults through configuration rather than runtime API calls. 