import os
import time
import json
import uuid
import random
import requests
from flask import Flask, request, jsonify
import prometheus_client
from prometheus_client import Counter, Histogram
from flask_healthz import healthz

# OpenTelemetry imports
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.resources import Resource
from opentelemetry.semconv.resource import ResourceAttributes
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.flask import FlaskInstrumentor
from opentelemetry.instrumentation.requests import RequestsInstrumentor

# Import structured logger
from structured_logger import StructuredLogger

# Initialize OpenTelemetry
resource = Resource.create({
    ResourceAttributes.SERVICE_NAME: "checkout-service",
    ResourceAttributes.DEPLOYMENT_ENVIRONMENT: os.getenv("DEPLOYMENT_ENVIRONMENT", "production")
})

# Configure the tracer provider
trace_provider = TracerProvider(resource=resource)
trace.set_tracer_provider(trace_provider)

# Get the OTLP endpoint from environment or use the default collector endpoint
otlp_endpoint = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel-collector:4318/v1/traces")

# Create an OTLP exporter and add it to the processor
otlp_exporter = OTLPSpanExporter(endpoint=otlp_endpoint)
span_processor = BatchSpanProcessor(otlp_exporter)
trace_provider.add_span_processor(span_processor)

# Create the tracer
tracer = trace.get_tracer(__name__)

# Initialize structured logger
logger = StructuredLogger("checkout-service")

app = Flask(__name__)

# Instrument Flask
FlaskInstrumentor().instrument_app(app)

# Instrument requests library
RequestsInstrumentor().instrument()

app.register_blueprint(healthz, url_prefix="/healthz")

# Prometheus metrics
REQUEST_COUNT = Counter('checkout_request_count', 'Checkout Service Request Count', ['method', 'endpoint', 'http_status'])
REQUEST_LATENCY = Histogram('checkout_request_latency_seconds', 'Checkout Service Request Latency', ['method', 'endpoint'])

# Service URLs from environment variables with defaults for local development
PRODUCT_CATALOG_SERVICE = os.getenv('PRODUCT_CATALOG_SERVICE', 'http://localhost:8081')
CURRENCY_SERVICE = os.getenv('CURRENCY_SERVICE', 'http://localhost:8082')

# In-memory order storage (in a real app, this would be a database)
orders = {}

def healthz_status():
    return True

app.config["HEALTHZ"] = {
    "live": healthz_status,
    "ready": healthz_status,
}

@app.before_request
def before_request():
    from flask import g
    request_id = request.headers.get('X-Request-ID')
    if request_id:
        g.request_id = request_id

@app.route('/')
def home():
    logger.info("Handling home request", method="GET", path="/")
    return jsonify({
        "service": "Checkout Service",
        "status": "UP"
    })

@app.route('/process', methods=['POST'])
def process_checkout():
    with tracer.start_as_current_span("process_checkout") as span:
        start_time = time.time()
        logger.info("Processing checkout request", method="POST", path="/process")
        
        try:
            checkout_data = request.get_json()
            if not checkout_data:
                REQUEST_COUNT.labels('post', '/process', 400).inc()
                return jsonify({"error": "No checkout data provided"}), 400
            
            # Set relevant span attributes
            span.set_attribute("user_id", checkout_data.get('user_id', 'unknown'))
            span.set_attribute("user_currency", checkout_data.get('user_currency', 'USD'))
            span.set_attribute("items_count", len(checkout_data.get('items', [])))
            
            # Check if required fields are present
            required_fields = ['user_id', 'user_currency', 'address', 'email', 'items']
            for field in required_fields:
                if field not in checkout_data:
                    REQUEST_COUNT.labels('post', '/process', 400).inc()
                    return jsonify({"error": f"Missing required field: {field}"}), 400
            
            # Get product information for each item in the cart
            items = checkout_data['items']
            products = []
            
            with tracer.start_as_current_span("fetch_product_details") as items_span:
                items_span.set_attribute("items_count", len(items))
                
                for item in items:
                    if 'product_id' not in item or 'quantity' not in item:
                        REQUEST_COUNT.labels('post', '/process', 400).inc()
                        return jsonify({"error": "Invalid item format"}), 400
                    
                    # Get product details from product catalog service
                    try:
                        with tracer.start_as_current_span(f"get_product_{item['product_id']}") as product_span:
                            product_span.set_attribute("product_id", item['product_id'])
                            product_span.set_attribute("quantity", item['quantity'])
                            
                            product_response = requests.get(f"{PRODUCT_CATALOG_SERVICE}/product/{item['product_id']}")
                            product_response.raise_for_status()
                            product = product_response.json()
                            
                            # Apply currency conversion if needed
                            if checkout_data['user_currency'] != 'USD':
                                try:
                                    with tracer.start_as_current_span("convert_currency") as currency_span:
                                        currency_span.set_attribute("from_currency", "USD")
                                        currency_span.set_attribute("to_currency", checkout_data['user_currency'])
                                        currency_span.set_attribute("amount", product['price'])
                                        
                                        currency_response = requests.get(
                                            f"{CURRENCY_SERVICE}/convert",
                                            params={
                                                "from": "USD",
                                                "to": checkout_data['user_currency'],
                                                "amount": product['price']
                                            }
                                        )
                                        currency_response.raise_for_status()
                                        conversion = currency_response.json()
                                        product['price'] = conversion['converted']
                                        product['currency'] = checkout_data['user_currency']
                                except requests.RequestException as e:
                                    logger.error("Currency conversion error", error=str(e), product_id=item['product_id'])
                                    # Continue with USD if currency service fails
                            
                            # Calculate total for this item
                            item_total = product['price'] * item['quantity']
                            
                            # Add to products list
                            products.append({
                                'id': product['id'],
                                'name': product['name'],
                                'price': product['price'],
                                'currency': product['currency'],
                                'quantity': item['quantity'],
                                'item_total': item_total
                            })
                            
                    except requests.RequestException as e:
                        logger.error("Product catalog service error", error=str(e), product_id=item['product_id'])
                        REQUEST_COUNT.labels('post', '/process', 500).inc()
                        return jsonify({"error": f"Failed to retrieve product information: {str(e)}"}), 500
            
            # Calculate order total
            order_total = sum(p['item_total'] for p in products)
            span.set_attribute("order_total", order_total)
            
            # Generate order ID
            order_id = str(uuid.uuid4())
            span.set_attribute("order_id", order_id)
            
            # Create order
            order = {
                'order_id': order_id,
                'user_id': checkout_data['user_id'],
                'user_currency': checkout_data['user_currency'],
                'address': checkout_data['address'],
                'email': checkout_data['email'],
                'products': products,
                'total': order_total,
                'status': 'PROCESSED',
                'timestamp': time.time()
            }
            
            # Store order (in a real app, this would be in a database)
            orders[order_id] = order
            
            # Return success response
            response = {
                'order_id': order_id,
                'total': order_total,
                'currency': checkout_data['user_currency'],
                'status': 'PROCESSED'
            }
            
            duration = time.time() - start_time
            REQUEST_COUNT.labels('post', '/process', 200).inc()
            REQUEST_LATENCY.labels('post', '/process').observe(duration)
            
            return jsonify(response)
            
        except Exception as e:
            logger.error("Checkout processing error", error=str(e), exception_type=type(e).__name__)
            REQUEST_COUNT.labels('post', '/process', 500).inc()
            return jsonify({"error": f"Checkout processing failed: {str(e)}"}), 500

@app.route('/order/<order_id>', methods=['GET'])
def get_order(order_id):
    with tracer.start_as_current_span("get_order") as span:
        span.set_attribute("order_id", order_id)
        
        start_time = time.time()
        logger.info("Getting order details", method="GET", path=f"/order/{order_id}", order_id=order_id)
        
        if order_id not in orders:
            logger.warning("Order not found", order_id=order_id)
            REQUEST_COUNT.labels('get', '/order/<order_id>', 404).inc()
            return jsonify({"error": "Order not found"}), 404
        
        duration = time.time() - start_time
        REQUEST_COUNT.labels('get', '/order/<order_id>', 200).inc()
        REQUEST_LATENCY.labels('get', '/order/<order_id>').observe(duration)
        
        return jsonify(orders[order_id])

@app.route('/metrics')
def metrics():
    return prometheus_client.generate_latest()

if __name__ == '__main__':
    port = os.getenv('PORT', '8084')
    logger.info("Checkout service starting", port=int(port))
    app.run(host='0.0.0.0', port=int(port)) 