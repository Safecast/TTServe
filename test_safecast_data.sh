#!/bin/bash
# Test script for TTServe with DuckDB using SafecastData format

echo "Testing TTServe with DuckDB implementation - SafecastData format"
echo "==============================================================="
echo "Sending test data to TTServe..."

# Send test data to TTServe using the format expected by the Safecast API
RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -d '{
    "device_urn": "test:device:123",
    "device_class": "test-device",
    "device_sn": "#123",
    "device_id": 12345,
    "captured_at": "2023-04-01T12:00:00Z",
    "loc": {
      "lat": 35.6895,
      "lon": 139.6917,
      "alt": 40.5
    },
    "env": {
      "temp": 22.5,
      "humid": 60.0
    },
    "dev": {
      "test": true
    },
    "bat": {
      "voltage": 3.8,
      "soc": 85.0
    }
  }' \
  http://localhost:8080/send)

echo "Response from TTServe:"
echo "$RESPONSE"

echo -e "\nTest completed!"
