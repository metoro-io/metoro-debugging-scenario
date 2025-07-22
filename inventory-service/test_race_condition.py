#!/usr/bin/env python3
"""
Test script to trigger race condition in inventory service.
This will create concurrent requests to reserve inventory, 
causing data corruption and 500 errors.
"""

import requests
import concurrent.futures
import time
import random

INVENTORY_SERVICE = "http://localhost:8085"
PRODUCT_ID = "GGOEAFKA087499"  # Product with initial quantity of 100

def reserve_inventory(request_num):
    """Make a reservation request"""
    try:
        response = requests.post(
            f"{INVENTORY_SERVICE}/inventory/reserve",
            json={
                "product_id": PRODUCT_ID,
                "quantity": random.randint(5, 15)
            }
        )
        return {
            "request": request_num,
            "status": response.status_code,
            "response": response.text if response.status_code != 200 else response.json()
        }
    except Exception as e:
        return {
            "request": request_num,
            "status": "error",
            "error": str(e)
        }

def check_inventory():
    """Check current inventory status"""
    try:
        response = requests.get(f"{INVENTORY_SERVICE}/inventory/{PRODUCT_ID}")
        if response.status_code == 200:
            data = response.json()
            print(f"\nInventory Status:")
            print(f"  Total: {data.get('quantity')}")
            print(f"  Reserved: {data.get('reserved')}")
            print(f"  Available: {data.get('available')}")
        else:
            print(f"Failed to get inventory: {response.status_code}")
    except Exception as e:
        print(f"Error checking inventory: {e}")

def main():
    print("Starting race condition test...")
    print(f"Target product: {PRODUCT_ID}")
    
    # Check initial inventory
    check_inventory()
    
    # Create concurrent requests
    num_workers = 20
    num_requests = 100
    
    print(f"\nSending {num_requests} concurrent requests with {num_workers} workers...")
    
    errors = []
    successes = []
    
    with concurrent.futures.ThreadPoolExecutor(max_workers=num_workers) as executor:
        futures = [executor.submit(reserve_inventory, i) for i in range(num_requests)]
        
        for future in concurrent.futures.as_completed(futures):
            result = future.result()
            if result['status'] == 500 or result['status'] == 'error':
                errors.append(result)
                print(f"ERROR: Request {result['request']} - {result.get('response', result.get('error'))}")
            elif result['status'] == 200:
                successes.append(result)
            
    print(f"\n\nResults:")
    print(f"  Successful reservations: {len(successes)}")
    print(f"  Errors (5XX or exceptions): {len(errors)}")
    print(f"  Error rate: {len(errors)/num_requests*100:.1f}%")
    
    # Check final inventory
    time.sleep(1)
    check_inventory()
    
    if errors:
        print(f"\n✓ Successfully triggered {len(errors)} errors!")
        print("\nSample errors:")
        for error in errors[:5]:
            print(f"  - {error}")
    else:
        print("\n✗ No errors triggered. Try running the test again.")

if __name__ == "__main__":
    main()