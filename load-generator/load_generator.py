import os
import time
import random
import json
import uuid
import logging
import threading
import requests
from prometheus_client import start_http_server, Counter, Histogram

# OpenTelemetry imports
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.resources import Resource
from opentelemetry.semconv.resource import ResourceAttributes
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.requests import RequestsInstrumentor

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger("LoadGenerator")

# Initialize OpenTelemetry
resource = Resource.create({
    ResourceAttributes.SERVICE_NAME: "load-generator",
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

# Instrument requests library
RequestsInstrumentor().instrument()

# Gateway service URL from environment variables with default for local development
GATEWAY_SERVICE = os.getenv('GATEWAY_SERVICE', 'http://localhost:8080')

# Prometheus metrics
REQUEST_COUNT = Counter('load_generator_request_count', 'Load Generator Request Count', ['method', 'endpoint', 'status'])
REQUEST_LATENCY = Histogram('load_generator_request_latency_seconds', 'Load Generator Request Latency', ['method', 'endpoint'])

# Constants
USER_IDS = [f"user-{i}" for i in range(1, 11)]
CURRENCIES = ["USD", "EUR", "GBP", "JPY", "CAD"]
PRODUCT_CATEGORIES = ["Electronics", "Computers", "Audio", "Wearables"]

# Simulated user addresses
ADDRESSES = [
    {"street": "123 Main St", "city": "New York", "state": "NY", "country": "USA", "zip": "10001"},
    {"street": "456 Market St", "city": "San Francisco", "state": "CA", "country": "USA", "zip": "94103"},
    {"street": "789 King St", "city": "London", "country": "UK", "zip": "SW1A 1AA"},
    {"street": "321 Queen St", "city": "Toronto", "state": "ON", "country": "Canada", "zip": "M5V 2A9"},
    {"street": "654 Tokyo Ave", "city": "Tokyo", "country": "Japan", "zip": "100-0001"},
]

# Simulated user emails
EMAILS = [f"user{i}@example.com" for i in range(1, 11)]

def get_products():
    """Get all products from the catalog via gateway"""
    with tracer.start_as_current_span("load_gen_get_products") as span:
        start_time = time.time()
        try:
            response = requests.get(f"{GATEWAY_SERVICE}/products")
            response.raise_for_status()
            duration = time.time() - start_time
            REQUEST_COUNT.labels('get', '/products', response.status_code).inc()
            REQUEST_LATENCY.labels('get', '/products').observe(duration)
            return response.json()['products']
        except requests.RequestException as e:
            duration = time.time() - start_time
            REQUEST_COUNT.labels('get', '/products', 500).inc()
            REQUEST_LATENCY.labels('get', '/products').observe(duration)
            logger.error(f"Error getting products: {str(e)}")
            return []

def get_product(product_id, currency="USD"):
    """Get a specific product by ID, with optional currency conversion"""
    with tracer.start_as_current_span("load_gen_get_product") as span:
        span.set_attribute("product_id", product_id)
        span.set_attribute("currency", currency)
        
        start_time = time.time()
        try:
            response = requests.get(
                f"{GATEWAY_SERVICE}/product/{product_id}",
                params={"currency": currency}
            )
            response.raise_for_status()
            duration = time.time() - start_time
            REQUEST_COUNT.labels('get', f'/product/{product_id}', response.status_code).inc()
            REQUEST_LATENCY.labels('get', f'/product/{product_id}').observe(duration)
            return response.json()
        except requests.RequestException as e:
            duration = time.time() - start_time
            REQUEST_COUNT.labels('get', f'/product/{product_id}', 500).inc()
            REQUEST_LATENCY.labels('get', f'/product/{product_id}').observe(duration)
            logger.error(f"Error getting product {product_id}: {str(e)}")
            return None

def checkout_cart(cart_items, user_id, currency):
    """Perform checkout process for a cart of items"""
    with tracer.start_as_current_span("load_gen_checkout") as span:
        if not cart_items:
            return
        
        span.set_attribute("user_id", user_id)
        span.set_attribute("currency", currency)
        span.set_attribute("items_count", len(cart_items))
        
        # Create checkout request
        checkout_data = {
            "user_id": user_id,
            "user_currency": currency,
            "address": random.choice(ADDRESSES),
            "email": random.choice(EMAILS),
            "items": cart_items
        }
        
        start_time = time.time()
        try:
            response = requests.post(
                f"{GATEWAY_SERVICE}/checkout",
                json=checkout_data
            )
            response.raise_for_status()
            duration = time.time() - start_time
            REQUEST_COUNT.labels('post', '/checkout', response.status_code).inc()
            REQUEST_LATENCY.labels('post', '/checkout').observe(duration)
            
            order = response.json()
            span.set_attribute("order_id", order.get('order_id', ''))
            span.set_attribute("order_total", order.get('total', 0))
            
            logger.info(f"Checkout successful. Order ID: {order['order_id']}, Total: {order['total']} {order['currency']}")
            return order
        except requests.RequestException as e:
            duration = time.time() - start_time
            REQUEST_COUNT.labels('post', '/checkout', 500).inc()
            REQUEST_LATENCY.labels('post', '/checkout').observe(duration)
            logger.error(f"Error during checkout: {str(e)}")
            return None

def simulate_user_browse():
    """Simulate a user browsing the shop"""
    with tracer.start_as_current_span("load_gen_user_browse") as span:
        # First get all products
        all_products = get_products()
        if not all_products:
            return
        
        # Select a random number of products to view in detail (1-3)
        num_products_to_view = random.randint(1, 3)
        products_to_view = random.sample(all_products, min(num_products_to_view, len(all_products)))
        
        # Choose a random currency for this session
        currency = random.choice(CURRENCIES)
        span.set_attribute("currency", currency)
        span.set_attribute("products_to_view", num_products_to_view)
        
        # View each product in detail
        for product in products_to_view:
            get_product(product['id'], currency)
            time.sleep(random.uniform(0.5, 2.0))
        
        # Decide whether to checkout (50% chance)
        will_checkout = random.random() < 0.5
        span.set_attribute("will_checkout", will_checkout)
        
        if will_checkout:
            # Create a cart with 1-3 items
            cart_size = random.randint(1, 3)
            cart_items = []
            
            for _ in range(cart_size):
                product = random.choice(all_products)
                cart_items.append({
                    "product_id": product['id'],
                    "quantity": random.randint(1, 3)
                })
            
            # Checkout
            user_id = random.choice(USER_IDS)
            checkout_cart(cart_items, user_id, currency)

def simulate_category_browse():
    """Simulate a user browsing by category"""
    with tracer.start_as_current_span("load_gen_category_browse") as span:
        # Choose a random category
        category = random.choice(PRODUCT_CATEGORIES)
        span.set_attribute("category", category)
        
        # Get products in that category
        start_time = time.time()
        try:
            response = requests.get(f"{GATEWAY_SERVICE}/products", params={"category": category})
            response.raise_for_status()
            duration = time.time() - start_time
            REQUEST_COUNT.labels('get', '/products?category', response.status_code).inc()
            REQUEST_LATENCY.labels('get', '/products?category').observe(duration)
            
            products = response.json()['products']
            span.set_attribute("products_found", len(products))
            
            # If products found, maybe look at one in detail
            if products and random.random() < 0.7:
                product = random.choice(products)
                get_product(product['id'])
                
        except requests.RequestException as e:
            duration = time.time() - start_time
            REQUEST_COUNT.labels('get', '/products?category', 500).inc()
            REQUEST_LATENCY.labels('get', '/products?category').observe(duration)
            logger.error(f"Error browsing category {category}: {str(e)}")

def generate_load(delay_between_users=2.0):
    """Main load generation loop"""
    while True:
        try:
            with tracer.start_as_current_span("load_gen_user_session") as span:
                # Choose a random user behavior
                behavior = random.choices(
                    ['browse', 'category', 'direct_checkout'],
                    weights=[0.6, 0.3, 0.1],
                    k=1
                )[0]
                
                span.set_attribute("behavior", behavior)
                
                if behavior == 'browse':
                    simulate_user_browse()
                elif behavior == 'category':
                    simulate_category_browse()
                elif behavior == 'direct_checkout':
                    # Get all products first
                    all_products = get_products()
                    if all_products:
                        # Create a cart with 1-2 items
                        cart_items = []
                        for _ in range(random.randint(1, 2)):
                            product = random.choice(all_products)
                            cart_items.append({
                                "product_id": product['id'],
                                "quantity": random.randint(1, 2)
                            })
                        
                        # Checkout
                        user_id = random.choice(USER_IDS)
                        currency = random.choice(CURRENCIES)
                        checkout_cart(cart_items, user_id, currency)
                
                # Wait between simulated users
                time.sleep(random.uniform(delay_between_users * 0.8, delay_between_users * 1.2))
            
        except Exception as e:
            logger.error(f"Unexpected error in load generation: {str(e)}")
            time.sleep(5)  # Wait a bit longer if there's an error

if __name__ == "__main__":
    # Configuration from environment variables
    delay_between_users = float(os.getenv('DELAY_BETWEEN_USERS', '2.0'))
    metrics_port = int(os.getenv('METRICS_PORT', '8099'))
    
    # Start Prometheus metrics server
    start_http_server(metrics_port)
    logger.info(f"Metrics server started on port {metrics_port}")
    
    # Wait for other services to start
    logger.info("Waiting for services to start...")
    time.sleep(10)
    
    # Start load generation
    logger.info(f"Starting load generator with {delay_between_users}s average delay between users")
    generate_load(delay_between_users) 