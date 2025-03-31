import os
import time
import threading
import random
from flask import Flask, request, jsonify
import prometheus_client
from prometheus_client import Counter, Histogram
from flask_healthz import healthz

app = Flask(__name__)
app.register_blueprint(healthz, url_prefix="/healthz")

# Prometheus metrics
REQUEST_COUNT = Counter('currency_request_count', 'Currency Service Request Count', ['method', 'endpoint', 'http_status'])
REQUEST_LATENCY = Histogram('currency_request_latency_seconds', 'Currency Service Request Latency', ['method', 'endpoint'])

# Exchange rates (relative to USD)
EXCHANGE_RATES = {
    'USD': 1.0,
    'EUR': 0.85,
    'GBP': 0.75,
    'JPY': 110.0,
    'CAD': 1.25,
    'AUD': 1.35,
    'CNY': 6.45,
}

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
            return jsonify({"error": "Injected fault: currency service error"}), 500

@app.route('/')
def home():
    return jsonify({
        "service": "Currency Service",
        "status": "UP"
    })

@app.route('/convert', methods=['GET'])
def convert_currency():
    start_time = time.time()
    
    from_currency = request.args.get('from', 'USD')
    to_currency = request.args.get('to', 'USD')
    amount = request.args.get('amount')
    
    # Validate inputs
    if from_currency not in EXCHANGE_RATES:
        REQUEST_COUNT.labels('get', '/convert', 400).inc()
        return jsonify({"error": f"Currency {from_currency} not supported"}), 400
    
    if to_currency not in EXCHANGE_RATES:
        REQUEST_COUNT.labels('get', '/convert', 400).inc()
        return jsonify({"error": f"Currency {to_currency} not supported"}), 400
    
    # Calculate conversion rate
    rate = EXCHANGE_RATES[to_currency] / EXCHANGE_RATES[from_currency]
    
    # If amount is provided, calculate converted amount
    result = {
        "from": from_currency,
        "to": to_currency,
        "rate": rate
    }
    
    if amount:
        try:
            amount_float = float(amount)
            result["amount"] = amount_float
            result["converted"] = round(amount_float * rate, 2)
        except ValueError:
            REQUEST_COUNT.labels('get', '/convert', 400).inc()
            return jsonify({"error": "Invalid amount provided"}), 400
    
    duration = time.time() - start_time
    REQUEST_COUNT.labels('get', '/convert', 200).inc()
    REQUEST_LATENCY.labels('get', '/convert').observe(duration)
    
    return jsonify(result)

@app.route('/rates', methods=['GET'])
def get_rates():
    start_time = time.time()
    
    base = request.args.get('base', 'USD')
    if base not in EXCHANGE_RATES:
        REQUEST_COUNT.labels('get', '/rates', 400).inc()
        return jsonify({"error": f"Currency {base} not supported"}), 400
    
    rates = {}
    for currency, rate in EXCHANGE_RATES.items():
        rates[currency] = round(rate / EXCHANGE_RATES[base], 4)
    
    result = {
        "base": base,
        "rates": rates
    }
    
    duration = time.time() - start_time
    REQUEST_COUNT.labels('get', '/rates', 200).inc()
    REQUEST_LATENCY.labels('get', '/rates').observe(duration)
    
    return jsonify(result)

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
    # Start with a separate thread to serve Prometheus metrics
    port = os.getenv('PORT', '8082')
    app.run(host='0.0.0.0', port=int(port)) 