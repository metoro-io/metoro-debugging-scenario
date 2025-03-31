import os
import json
import requests
from flask import Flask, request, jsonify
import prometheus_client
from prometheus_client import Counter
from flask_healthz import healthz

app = Flask(__name__)
app.register_blueprint(healthz, url_prefix="/healthz")

# Service URLs from environment variables with defaults for local development
PRODUCT_CATALOG_SERVICE = os.getenv('PRODUCT_CATALOG_SERVICE', 'http://localhost:8081')
CURRENCY_SERVICE = os.getenv('CURRENCY_SERVICE', 'http://localhost:8082')
AD_SERVICE = os.getenv('AD_SERVICE', 'http://localhost:8083')
CHECKOUT_SERVICE = os.getenv('CHECKOUT_SERVICE', 'http://localhost:8084')

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
    return jsonify({
        "status": "success",
        "message": "Gateway Service is running"
    })

@app.route('/products', methods=['GET'])
def get_products():
    try:
        response = requests.get(f"{PRODUCT_CATALOG_SERVICE}/products")
        response.raise_for_status()
        products = response.json()
        
        # Get currency info
        currency = request.args.get('currency', 'USD')
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
            # Add ads to the response
            response_data = {
                "products": products,
                "ads": ads
            }
        else:
            response_data = {
                "products": products,
                "ads": []
            }
            
        REQUEST_COUNT.labels('get', '/products', 200).inc()
        return jsonify(response_data)
        
    except requests.RequestException as e:
        REQUEST_COUNT.labels('get', '/products', 500).inc()
        return jsonify({"error": str(e)}), 500

@app.route('/product/<product_id>', methods=['GET'])
def get_product(product_id):
    try:
        response = requests.get(f"{PRODUCT_CATALOG_SERVICE}/product/{product_id}")
        response.raise_for_status()
        product = response.json()
        
        # Get currency info
        currency = request.args.get('currency', 'USD')
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
            response_data = {
                "product": product,
                "related_ads": ads
            }
        else:
            response_data = {
                "product": product,
                "related_ads": []
            }
            
        REQUEST_COUNT.labels('get', f'/product/{product_id}', 200).inc()
        return jsonify(response_data)
        
    except requests.RequestException as e:
        REQUEST_COUNT.labels('get', f'/product/{product_id}', 500).inc()
        return jsonify({"error": str(e)}), 500

@app.route('/checkout', methods=['POST'])
def checkout():
    try:
        # Forward the request to the checkout service
        checkout_data = request.get_json()
        response = requests.post(f"{CHECKOUT_SERVICE}/process", json=checkout_data)
        response.raise_for_status()
        
        REQUEST_COUNT.labels('post', '/checkout', 200).inc()
        return jsonify(response.json())
        
    except requests.RequestException as e:
        REQUEST_COUNT.labels('post', '/checkout', 500).inc()
        return jsonify({"error": str(e)}), 500

@app.route('/metrics')
def metrics():
    return prometheus_client.generate_latest()

# Fault injection endpoint - for debugging scenarios
@app.route('/fault', methods=['POST'])
def inject_fault():
    fault_config = request.get_json()
    service = fault_config.get('service')
    fault_type = fault_config.get('type')
    duration = fault_config.get('duration', 60)  # seconds
    
    # For demonstration, we just log the fault injection
    app.logger.info(f"Fault injection requested: {service}, {fault_type}, {duration}s")
    
    # In a real implementation, we would communicate with the target service
    # to inject the requested fault
    return jsonify({"status": "fault injection requested"})

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8080) 