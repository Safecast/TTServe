#!/bin/bash

# Set up variables
DATA_DIR="/home/rob/Documents/Safecast/TTServe"
DEVICE_ID=76543210  # Unique device ID to easily identify our test data
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo "=== Testing TTServe Direct JSON Send ==="
echo "1. Stopping any running TTServe instances..."
pkill -9 TTServe

echo "2. Starting TTServe..."
"$DATA_DIR/TTServe" "$DATA_DIR" > ttserve_log.txt 2>&1 &
TTSERVE_PID=$!
echo "TTServe started with PID: $TTSERVE_PID"

# Wait for TTServe to initialize
sleep 5

echo "3. Sending direct JSON data to TTServe..."
RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "User-Agent: TTGATE" \
  -d '{
    "device_id": '"$DEVICE_ID"',
    "when_captured": "'"$TIMESTAMP"'",
    "loc_lat": 35.6895,
    "loc_lon": 139.6917,
    "value": 42.0,
    "unit": "cpm"
  }' \
  http://localhost:8080/send)

echo "Response: $RESPONSE"

# Wait for data to be processed
sleep 5

echo "4. Stopping TTServe..."
kill -9 $TTSERVE_PID

echo "5. Checking if data was stored correctly..."
echo "API Database (looking for device ID $DEVICE_ID):"
duckdb "$DATA_DIR/api.duckdb" -c "SELECT id, device_id, captured_at, value, unit, latitude, longitude FROM measurements WHERE device_id = $DEVICE_ID ORDER BY id DESC LIMIT 5;"

echo "Ingest Database (looking for device ID $DEVICE_ID):"
duckdb "$DATA_DIR/ingest.duckdb" -c "SELECT id, device_id, when_captured, data FROM measurements WHERE device_id = $DEVICE_ID ORDER BY id DESC LIMIT 5;"

echo "=== Test Complete ==="
