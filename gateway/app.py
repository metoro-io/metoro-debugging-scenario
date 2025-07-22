import os
import json
import requests
from flask import Flask, request, jsonify
import prometheus_client
from prometheus_client import Counter
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
    ResourceAttributes.SERVICE_NAME: "gateway",
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
logger = StructuredLogger("gateway")

app = Flask(__name__)

# Instrument Flask
FlaskInstrumentor().instrument_app(app)

# Instrument requests library
RequestsInstrumentor().instrument()

app.register_blueprint(healthz, url_prefix="/healthz")

# Service URLs from environment variables with defaults for local development
PRODUCT_CATALOG_SERVICE = os.getenv('PRODUCT_CATALOG_SERVICE', 'http://localhost:8081')
CURRENCY_SERVICE = os.getenv('CURRENCY_SERVICE', 'http://localhost:8082')
AD_SERVICE = os.getenv('AD_SERVICE', 'http://localhost:8083')
CHECKOUT_SERVICE = os.getenv('CHECKOUT_SERVICE', 'http://localhost:8084')
INVENTORY_SERVICE = os.getenv('INVENTORY_SERVICE', 'http://localhost:8085')

# Prometheus metrics
REQUEST_COUNT = Counter('request_count', 'App Request Count', ['method', 'endpoint', 'http_status'])

def healthz_status():
    return True

app.config["HEALTHZ"] = {
    "live": healthz_status,
    "ready": healthz_status,
}

@app.route('/')
def home():
    logger.info("Handling home request", method="GET", path="/")
    return jsonify({
        "status": "success",
        "message": "Gateway Service is running"
    })

@app.route('/products', methods=['GET'])
def get_products():
    try:
        with tracer.start_as_current_span("get_products") as span:
            logger.info("Handling get products request", method="GET", path="/products")
            category = request.args.get('category')
            if category:
                span.set_attribute("category", category)
            
            # Get products from catalog service
            response = requests.get(f"{PRODUCT_CATALOG_SERVICE}/products", params=request.args)
            response.raise_for_status()
            products = response.json()
            
            # Get currency info
            currency = request.args.get('currency', 'USD')
            span.set_attribute("currency", currency)
            
            if currency != 'USD':
                curr_response = requests.get(f"{CURRENCY_SERVICE}/convert", 
                    params={"from": "USD", "to": currency})
                curr_response.raise_for_status()
                rate = curr_response.json().get('rate', 1.0)
                
                # Apply currency conversion to each product price
                for product in products:
                    product['price'] = round(float(product['price']) * rate, 2)
                    product['currency'] = currency
            
            # Get ads for these products
            ad_response = requests.get(f"{AD_SERVICE}/ads", 
                params={"product_ids": ",".join([str(p['id']) for p in products[:3]])})
            
            if ad_response.status_code == 200:
                ads = ad_response.json()
            else:
                ads = []
            
            # Check inventory for products
            inventory_data = []
            for product in products[:5]:  # Check inventory for first 5 products
                try:
                    inv_response = requests.get(f"{INVENTORY_SERVICE}/inventory/{str(product['id'])}")
                    if inv_response.status_code == 200:
                        inventory_data.append(inv_response.json())
                    else:
                        logger.warning("Failed to get inventory", product_id=product['id'], status_code=inv_response.status_code)
                except Exception as e:
                    logger.error("Error getting inventory", product_id=product['id'], error=str(e))
            
            response_data = {
                "products": products,
                "ads": ads,
                "inventory": inventory_data
            }
                
            REQUEST_COUNT.labels('get', '/products', 200).inc()
            return jsonify(response_data)
        
    except requests.RequestException as e:
        logger.error("Error handling get products request", error=str(e), exception_type=type(e).__name__)
        REQUEST_COUNT.labels('get', '/products', 500).inc()
        return jsonify({"error": str(e)}), 500

@app.route('/product/<product_id>', methods=['GET'])
def get_product(product_id):
    try:
        with tracer.start_as_current_span("get_product") as span:
            logger.info("Handling get product request", method="GET", path=f"/product/{product_id}", product_id=product_id)
            span.set_attribute("product_id", product_id)
            
            response = requests.get(f"{PRODUCT_CATALOG_SERVICE}/product/{product_id}")
            response.raise_for_status()
            product = response.json()
            
            # Get currency info
            currency = request.args.get('currency', 'USD')
            span.set_attribute("currency", currency)
            
            if currency != 'USD':
                curr_response = requests.get(f"{CURRENCY_SERVICE}/convert", 
                    params={"from": "USD", "to": currency})
                curr_response.raise_for_status()
                rate = curr_response.json().get('rate', 1.0)
                
                # Apply currency conversion
                product['price'] = round(float(product['price']) * rate, 2)
                product['currency'] = currency
            
            # Get ad for this product
            ad_response = requests.get(f"{AD_SERVICE}/ads", 
                params={"product_ids": product_id})
            
            if ad_response.status_code == 200:
                ads = ad_response.json()
            else:
                ads = []
            
            # Get inventory for this product
            inventory_info = None
            try:
                inv_response = requests.get(f"{INVENTORY_SERVICE}/inventory/{product_id}")
                if inv_response.status_code == 200:
                    inventory_info = inv_response.json()
                else:
                    logger.warning("Failed to get inventory", product_id=product_id, status_code=inv_response.status_code)
            except Exception as e:
                logger.error("Error getting inventory", product_id=product_id, error=str(e))
            
            response_data = {
                "product": product,
                "related_ads": ads,
                "inventory": inventory_info
            }
                
            REQUEST_COUNT.labels('get', f'/product/{product_id}', 200).inc()
            return jsonify(response_data)
        
    except requests.RequestException as e:
        logger.error("Error handling get product request", error=str(e), exception_type=type(e).__name__, product_id=product_id)
        REQUEST_COUNT.labels('get', f'/product/{product_id}', 500).inc()
        return jsonify({"error": str(e)}), 500

@app.route('/checkout', methods=['POST'])
def checkout():
    try:
        with tracer.start_as_current_span("checkout") as span:
            logger.info("Handling checkout request", method="POST", path="/checkout")
            # Forward the request to the checkout service
            checkout_data = request.get_json()
            items = checkout_data.get('items', [])
            span.set_attribute("items_count", len(items))
            
            # Reserve inventory for each item
            reservations = []
            for item in items:
                try:
                    reserve_response = requests.post(f"{INVENTORY_SERVICE}/inventory/reserve", 
                        json={
                            "product_id": item['product_id'],
                            "quantity": item['quantity']
                        })
                    if reserve_response.status_code == 200:
                        reservations.append(reserve_response.json())
                    else:
                        logger.error("Failed to reserve inventory", 
                            product_id=item['product_id'], 
                            status_code=reserve_response.status_code,
                            response=reserve_response.text)
                        # Continue with checkout even if reservation fails
                except Exception as e:
                    logger.error("Error reserving inventory", 
                        product_id=item['product_id'], 
                        error=str(e))
            
            # Add reservations to checkout data
            checkout_data['reservations'] = reservations
            
            response = requests.post(f"{CHECKOUT_SERVICE}/process", json=checkout_data)
            response.raise_for_status()
            
            REQUEST_COUNT.labels('post', '/checkout', 200).inc()
            return jsonify(response.json())
        
    except requests.RequestException as e:
        logger.error("Error handling checkout request", error=str(e), exception_type=type(e).__name__)
        REQUEST_COUNT.labels('post', '/checkout', 500).inc()
        return jsonify({"error": str(e)}), 500

@app.route('/metrics')
def metrics():
    return prometheus_client.generate_latest()

if __name__ == '__main__':
    logger.info("Gateway service starting", port=8080)
    app.run(host='0.0.0.0', port=8080) 