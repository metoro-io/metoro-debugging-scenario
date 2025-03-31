import unittest
import json
import sys
import os
from unittest.mock import patch, MagicMock

# Add parent directory to path so we can import the app
sys.path.insert(0, os.path.abspath(os.path.dirname(__file__)))
from app import app

class GatewayServiceTest(unittest.TestCase):
    def setUp(self):
        self.app = app.test_client()
        self.app.testing = True

    def test_home_endpoint(self):
        response = self.app.get('/')
        data = json.loads(response.data)
        self.assertEqual(response.status_code, 200)
        self.assertEqual(data['status'], 'success')
        self.assertEqual(data['message'], 'Gateway Service is running')

    @patch('app.requests.get')
    def test_get_products(self, mock_get):
        # Mock the product catalog response
        mock_product_response = MagicMock()
        mock_product_response.status_code = 200
        mock_product_response.json.return_value = [
            {"id": 1, "name": "Test Product", "price": 10.0, "currency": "USD"}
        ]
        
        # Mock the ad service response
        mock_ad_response = MagicMock()
        mock_ad_response.status_code = 200
        mock_ad_response.json.return_value = [
            {"id": "ad1", "redirect_url": "https://example.com", "text": "Ad text"}
        ]
        
        # Configure mock to return different responses for different URLs
        def side_effect(url, **kwargs):
            if 'products' in url:
                return mock_product_response
            elif 'ads' in url:
                return mock_ad_response
            return MagicMock()
        
        mock_get.side_effect = side_effect
        
        response = self.app.get('/products')
        data = json.loads(response.data)
        
        self.assertEqual(response.status_code, 200)
        self.assertIn('products', data)
        self.assertIn('ads', data)

    @patch('app.requests.get')
    def test_get_product(self, mock_get):
        # Mock the product catalog response
        mock_product_response = MagicMock()
        mock_product_response.status_code = 200
        mock_product_response.json.return_value = {
            "id": 1, "name": "Test Product", "price": 10.0, "currency": "USD"
        }
        
        # Mock the ad service response
        mock_ad_response = MagicMock()
        mock_ad_response.status_code = 200
        mock_ad_response.json.return_value = [
            {"id": "ad1", "redirect_url": "https://example.com", "text": "Ad text"}
        ]
        
        # Configure mock to return different responses for different URLs
        def side_effect(url, **kwargs):
            if '/product/' in url:
                return mock_product_response
            elif 'ads' in url:
                return mock_ad_response
            return MagicMock()
        
        mock_get.side_effect = side_effect
        
        response = self.app.get('/product/1')
        data = json.loads(response.data)
        
        self.assertEqual(response.status_code, 200)
        self.assertIn('product', data)
        self.assertIn('related_ads', data)

    @patch('app.requests.post')
    def test_checkout(self, mock_post):
        # Mock the checkout service response
        mock_checkout_response = MagicMock()
        mock_checkout_response.status_code = 200
        mock_checkout_response.json.return_value = {
            "order_id": "test-order-id",
            "total": 20.0,
            "currency": "USD",
            "status": "PROCESSED"
        }
        mock_post.return_value = mock_checkout_response
        
        checkout_data = {
            "user_id": "user-1",
            "user_currency": "USD",
            "address": {"street": "123 Test St"},
            "email": "test@example.com",
            "items": [{"product_id": 1, "quantity": 2}]
        }
        
        response = self.app.post(
            '/checkout',
            data=json.dumps(checkout_data),
            content_type='application/json'
        )
        data = json.loads(response.data)
        
        self.assertEqual(response.status_code, 200)
        self.assertEqual(data['order_id'], 'test-order-id')
        self.assertEqual(data['total'], 20.0)

if __name__ == '__main__':
    unittest.main() 