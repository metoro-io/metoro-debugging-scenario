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

app = Flask(__name__)
app.register_blueprint(healthz, url_prefix="/healthz")

# Prometheus metrics
REQUEST_COUNT = Counter('checkout_request_count', 'Checkout Service Request Count', ['method', 'endpoint', 'http_status'])
REQUEST_LATENCY = Histogram('checkout_request_latency_seconds', 'Checkout Service Request Latency', ['method', 'endpoint'])

# Service URLs from environment variables with defaults for local development
PRODUCT_CATALOG_SERVICE = os.getenv('PRODUCT_CATALOG_SERVICE', 'http://localhost:8081')
CURRENCY_SERVICE = os.getenv('CURRENCY_SERVICE', 'http://localhost:8082')

# In-memory order storage (in a real app, this would be a database)
orders = {}

# Fault configuration
fault_config = {
    'enabled': False,
    'latency_ms': 0,
    'error_rate': 0.0,
    'expiration_time': time.time()
}

def healthz_status():
    return True

app.config["HEALTHZ"] = {
    "live": healthz_status,
    "ready": healthz_status,
}

# Middleware for fault injection
@app.before_request
def before_request():
    # Skip fault injection for metrics and health endpoints
    if request.path.startswith('/metrics') or request.path.startswith('/healthz'):
        return
    
    # Check if fault injection is enabled and not expired
    if fault_config['enabled'] and time.time() < fault_config['expiration_time']:
        # Latency injection
        if fault_config['latency_ms'] > 0:
            time.sleep(fault_config['latency_ms'] / 1000.0)
        
        # Error injection
        if fault_config['error_rate'] > 0 and random.random() < fault_config['error_rate']:
            return jsonify({"error": "Injected fault: checkout service error"}), 500

@app.route('/')
def home():
    return jsonify({
        "service": "Checkout Service",
        "status": "UP"
    })

@app.route('/process', methods=['POST'])
def process_checkout():
    start_time = time.time()
    
    try:
        checkout_data = request.get_json()
        if not checkout_data:
            REQUEST_COUNT.labels('post', '/process', 400).inc()
            return jsonify({"error": "No checkout data provided"}), 400
        
        # Check if required fields are present
        required_fields = ['user_id', 'user_currency', 'address', 'email', 'items']
        for field in required_fields:
            if field not in checkout_data:
                REQUEST_COUNT.labels('post', '/process', 400).inc()
                return jsonify({"error": f"Missing required field: {field}"}), 400
        
        # Get product information for each item in the cart
        items = checkout_data['items']
        products = []
        
        for item in items:
            if 'product_id' not in item or 'quantity' not in item:
                REQUEST_COUNT.labels('post', '/process', 400).inc()
                return jsonify({"error": "Invalid item format"}), 400
            
            # Get product details from product catalog service
            try:
                product_response = requests.get(f"{PRODUCT_CATALOG_SERVICE}/product/{item['product_id']}")
                product_response.raise_for_status()
                product = product_response.json()
                
                # Apply currency conversion if needed
                if checkout_data['user_currency'] != 'USD':
                    try:
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
                        app.logger.error(f"Currency conversion error: {str(e)}")
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
                app.logger.error(f"Product catalog service error: {str(e)}")
                REQUEST_COUNT.labels('post', '/process', 500).inc()
                return jsonify({"error": f"Failed to retrieve product information: {str(e)}"}), 500
        
        # Calculate order total
        order_total = sum(p['item_total'] for p in products)
        
        # Generate order ID
        order_id = str(uuid.uuid4())
        
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
        app.logger.error(f"Checkout processing error: {str(e)}")
        REQUEST_COUNT.labels('post', '/process', 500).inc()
        return jsonify({"error": f"Checkout processing failed: {str(e)}"}), 500

@app.route('/order/<order_id>', methods=['GET'])
def get_order(order_id):
    start_time = time.time()
    
    if order_id not in orders:
        REQUEST_COUNT.labels('get', '/order/<order_id>', 404).inc()
        return jsonify({"error": "Order not found"}), 404
    
    duration = time.time() - start_time
    REQUEST_COUNT.labels('get', '/order/<order_id>', 200).inc()
    REQUEST_LATENCY.labels('get', '/order/<order_id>').observe(duration)
    
    return jsonify(orders[order_id])

@app.route('/metrics')
def metrics():
    return prometheus_client.generate_latest()

@app.route('/fault', methods=['POST'])
def inject_fault():
    global fault_config
    
    data = request.get_json()
    if not data:
        return jsonify({"error": "No data provided"}), 400
    
    enabled = data.get('enabled', False)
    latency_ms = data.get('latency_ms', 0)
    error_rate = data.get('error_rate', 0.0)
    duration_sec = data.get('duration_sec', 60)
    
    fault_config = {
        'enabled': enabled,
        'latency_ms': latency_ms,
        'error_rate': error_rate,
        'expiration_time': time.time() + duration_sec
    }
    
    return jsonify({
        "status": "Fault injection configuration updated",
        "config": fault_config
    })

if __name__ == '__main__':
    port = os.getenv('PORT', '8084')
    app.run(host='0.0.0.0', port=int(port)) 