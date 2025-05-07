#!/bin/bash
# Test script for TTServe with DuckDB using ttdata.SafecastData format

# Set your data directory here
DATA_DIR="/home/rob/Documents/Safecast/TTServe"

echo "Testing TTServe with DuckDB implementation"
echo "----------------------------------------"
echo "Sending test data to TTServe..."

# Create a payload in the TTGateReq format expected by the TTSERVE user agent
RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "User-Agent: TTSERVE" \
  -d '{
    "Transport": "test-transport",
    "GatewayTime": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'",
    "Payload": "test-payload"
  }' \
  http://localhost:8080/send)

echo "Response from TTServe:"
echo "$RESPONSE"

# Wait for data to be processed
echo "Waiting for data to be processed..."
sleep 5

# Stop TTServe to release database locks
echo "Stopping TTServe..."
pkill -f "./TTServe"
sleep 2

echo -e "\n\nVerifying data in DuckDB databases:"
echo "----------------------------------------"

# Check the API database
echo "Checking API database:"
duckdb "$DATA_DIR/api.duckdb" -c "SELECT * FROM measurements;"

# Check the Ingest database
echo "Checking Ingest database:"
duckdb "$DATA_DIR/ingest.duckdb" -c "SELECT * FROM measurements;"
