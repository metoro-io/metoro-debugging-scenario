import os
import time
import random
import uuid
import requests
from prometheus_client import start_http_server, Counter, Histogram

from structured_logger import StructuredLogger

# Initialize structured logger
logger = StructuredLogger("load-generator-instabook")

# Instabook service URL from environment variables
INSTABOOK_SERVICE = os.getenv('INSTABOOK_SERVICE', 'http://localhost:8087')

# Prometheus metrics
REQUEST_COUNT = Counter(
    'load_generator_instabook_request_count',
    'Load Generator Instabook Request Count',
    ['method', 'endpoint', 'status']
)
REQUEST_LATENCY = Histogram(
    'load_generator_instabook_request_latency_seconds',
    'Load Generator Instabook Request Latency',
    ['method', 'endpoint']
)

# Session storage for read operations
created_sessions = []


def generate_session_data():
    """Generate random session data for booking."""
    user_ids = [f"user-{i}" for i in range(1, 21)]
    statuses = ["pending", "confirmed", "processing"]
    booking_types = ["flight", "hotel", "car", "package"]

    return {
        "id": str(uuid.uuid4()),
        "user_id": random.choice(user_ids),
        "booking_id": f"BK-{uuid.uuid4().hex[:8].upper()}",
        "status": random.choice(statuses),
        "data": f"Booking for {random.choice(booking_types)} - {random.randint(1, 5)} items"
    }


def create_booking_session():
    """Create a new booking session via instabook service."""
    session_data = generate_session_data()

    start_time = time.time()
    try:
        response = requests.post(
            f"{INSTABOOK_SERVICE}/booking/session",
            json=session_data,
            timeout=10
        )
        duration = time.time() - start_time

        REQUEST_COUNT.labels('post', '/booking/session', response.status_code).inc()
        REQUEST_LATENCY.labels('post', '/booking/session').observe(duration)

        if response.status_code == 201:
            logger.info("Successfully created booking session",
                       session_id=session_data["id"],
                       user_id=session_data["user_id"])
            # Store for later retrieval
            created_sessions.append(session_data["id"])
            # Keep only last 50 sessions
            if len(created_sessions) > 50:
                created_sessions.pop(0)
            return session_data["id"]
        elif response.status_code == 500:
            error_msg = response.json().get("error", "Unknown error")
            logger.error("Failed to create booking session - internal error",
                        session_id=session_data["id"],
                        status_code=response.status_code,
                        error=error_msg)
            return None
        else:
            logger.error("Failed to create booking session",
                        session_id=session_data["id"],
                        status_code=response.status_code)
            return None

    except requests.RequestException as e:
        duration = time.time() - start_time
        REQUEST_COUNT.labels('post', '/booking/session', 500).inc()
        REQUEST_LATENCY.labels('post', '/booking/session').observe(duration)
        logger.error("Request error creating booking session",
                    session_id=session_data["id"],
                    error=str(e))
        return None


def get_booking_session(session_id):
    """Get an existing booking session via instabook service."""
    start_time = time.time()
    try:
        response = requests.get(
            f"{INSTABOOK_SERVICE}/booking/session/{session_id}",
            timeout=10
        )
        duration = time.time() - start_time

        REQUEST_COUNT.labels('get', '/booking/session/:id', response.status_code).inc()
        REQUEST_LATENCY.labels('get', '/booking/session/:id').observe(duration)

        if response.status_code == 200:
            session = response.json()
            logger.info("Successfully retrieved booking session",
                       session_id=session_id,
                       user_id=session.get("user_id"))
            return session
        elif response.status_code == 500:
            error_msg = response.json().get("error", "Unknown error")
            logger.error("Failed to get booking session - internal error",
                        session_id=session_id,
                        status_code=response.status_code,
                        error=error_msg)
            return None
        elif response.status_code == 404:
            logger.warning("Booking session not found",
                          session_id=session_id)
            return None
        else:
            logger.error("Failed to get booking session",
                        session_id=session_id,
                        status_code=response.status_code)
            return None

    except requests.RequestException as e:
        duration = time.time() - start_time
        REQUEST_COUNT.labels('get', '/booking/session/:id', 500).inc()
        REQUEST_LATENCY.labels('get', '/booking/session/:id').observe(duration)
        logger.error("Request error getting booking session",
                    session_id=session_id,
                    error=str(e))
        return None


def simulate_booking_flow():
    """Simulate a typical booking flow."""
    # Create a new session
    session_id = create_booking_session()

    if session_id:
        # Small delay then read it back
        time.sleep(random.uniform(0.5, 1.5))
        get_booking_session(session_id)


def simulate_read_existing():
    """Read an existing session (if any exist)."""
    if created_sessions:
        session_id = random.choice(created_sessions)
        get_booking_session(session_id)


def generate_load(delay_between_requests=3.0):
    """Main load generation loop."""
    logger.info("Starting load generation",
               delay=delay_between_requests,
               instabook_service=INSTABOOK_SERVICE)

    while True:
        try:
            # Choose a random action
            action = random.choices(
                ['create_and_read', 'read_existing', 'create_only'],
                weights=[0.5, 0.3, 0.2],
                k=1
            )[0]

            if action == 'create_and_read':
                simulate_booking_flow()
            elif action == 'read_existing':
                simulate_read_existing()
            elif action == 'create_only':
                create_booking_session()

            # Wait between requests with some jitter
            time.sleep(random.uniform(
                delay_between_requests * 0.8,
                delay_between_requests * 1.2
            ))

        except Exception as e:
            logger.error("Unexpected error in load generation", error=str(e))
            time.sleep(5)


if __name__ == "__main__":
    # Configuration from environment variables
    delay_between_requests = float(os.getenv('DELAY_BETWEEN_REQUESTS', '3.0'))
    metrics_port = int(os.getenv('METRICS_PORT', '8098'))

    # Start Prometheus metrics server
    start_http_server(metrics_port)
    logger.info("Metrics server started", port=metrics_port)

    # Wait for other services to start
    logger.info("Waiting for services to start...")
    time.sleep(10)

    # Start load generation
    generate_load(delay_between_requests)
