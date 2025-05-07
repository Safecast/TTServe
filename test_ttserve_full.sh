#!/bin/bash
# Comprehensive test for TTServe with DuckDB

echo "TTServe DuckDB Integration Test"
echo "=============================="

# Set your data directory here
DATA_DIR="/home/rob/Documents/Safecast/TTServe"

# Check if TTServe is running
if ! pgrep -f "./TTServe" > /dev/null; then
    echo "TTServe is not running. Starting TTServe..."
    ./TTServe "$DATA_DIR" &
    sleep 5
fi

# Test 1: Send data with TTGATE user agent
echo -e "\nTest 1: Sending data with TTGATE user agent..."
RESPONSE1=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "User-Agent: TTGATE" \
  -d '{
    "GatewayID": "test-gateway-123",
    "Transport": "lora",
    "GatewayTime": "2023-04-01T12:00:00Z",
    "Payload": "eyJkZXZpY2VfdXJuIjoidGVzdDpkZXZpY2U6MTIzIiwiZGV2aWNlX2lkIjoxMjM0NSwid2hlbl9jYXB0dXJlZCI6IjIwMjMtMDQtMDFUMTI6MDA6MDBaIiwibG9jX2xhdCI6MzUuNjg5NSwibG9jX2xvbiI6MTM5LjY5MTd9"
  }' \
  http://localhost:8080/send)

echo "Response from TTServe (TTGATE):"
echo "$RESPONSE1"

# Wait for data to be processed
sleep 2

# Test 2: Send data with no specific user agent (should be treated as web crawler)
echo -e "\nTest 2: Sending data with no specific user agent..."
RESPONSE2=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -d '{
    "device_urn": "test:device:456",
    "device_id": 45678,
    "when_captured": "2023-04-01T12:00:00Z",
    "loc_lat": 35.6895,
    "loc_lon": 139.6917
  }' \
  http://localhost:8080/send)

echo "Response from TTServe (no user agent):"
echo "$RESPONSE2"

# Wait for data to be processed
sleep 2

# Check the DuckDB databases
echo -e "\nChecking API database:"
duckdb "$DATA_DIR/api.duckdb" -c "SELECT * FROM measurements ORDER BY id DESC LIMIT 5;"

echo -e "\nChecking Ingest database:"
duckdb "$DATA_DIR/ingest.duckdb" -c "SELECT * FROM measurements ORDER BY id DESC LIMIT 5;"

echo -e "\nTest completed!"
