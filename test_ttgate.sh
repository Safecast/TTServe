#!/bin/bash
# Test script for TTServe with DuckDB using the TTGateReq format

echo "Testing TTServe with DuckDB implementation - TTGateReq format"
echo "============================================================"
echo "Sending test data to TTServe..."

# Create a payload in the TTGateReq format expected by the TTSERVE user agent
# The payload should be base64 encoded binary data
PAYLOAD=$(echo -n '{"device_urn":"test:device:123","device":12345,"when_captured":"2023-04-01T12:00:00Z","loc_lat":35.6895,"loc_lon":139.6917}' | base64)

RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "User-Agent: TTSERVE" \
  -d '{
    "payload": "'$PAYLOAD'",
    "service_transport": "test-transport",
    "gateway_received": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'"
  }' \
  http://localhost:8080/send)

echo "Response from TTServe:"
echo "$RESPONSE"

echo -e "\nTest completed!"
