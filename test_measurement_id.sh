#!/bin/bash
# Test script to verify if measurement IDs are returned by TTServe

echo "Testing Measurement ID Return"
echo "============================"

# Set your data directory here
DATA_DIR="/home/rob/Documents/Safecast/TTServe"

# Check if TTServe is running
if ! pgrep -f "./TTServe" > /dev/null; then
    echo "TTServe is not running. Starting TTServe..."
    ./TTServe "$DATA_DIR" &
    sleep 5
fi

# Send test data to TTServe using the format expected by the Safecast API
echo -e "\nSending test data to TTServe..."
RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "User-Agent: TTGATE" \
  -d '{
    "GatewayID": "test-gateway-123",
    "Transport": "lora",
    "GatewayTime": "2023-04-01T12:00:00Z",
    "Payload": "eyJkZXZpY2VfdXJuIjoidGVzdDpkZXZpY2U6MTIzIiwiZGV2aWNlX2lkIjoxMjM0NSwid2hlbl9jYXB0dXJlZCI6IjIwMjMtMDQtMDFUMTI6MDA6MDBaIiwibG9jX2xhdCI6MzUuNjg5NSwibG9jX2xvbiI6MTM5LjY5MTd9"
  }' \
  http://localhost:8080/send)

echo "Response from TTServe:"
echo "$RESPONSE"

# Check if the response contains measurement IDs
if [[ "$RESPONSE" == *"measurement_id"* ]]; then
    echo -e "\n✅ Success: Response contains measurement ID(s)"
    
    # Extract and display the measurement IDs
    echo -e "\nExtracted Measurement IDs:"
    echo "$RESPONSE" | grep -o '"measurement_id[^"]*":"[^"]*"' | sed 's/"measurement_id[^"]*":"//g' | sed 's/"//g'
else
    echo -e "\n❌ Failure: Response does not contain measurement ID(s)"
fi

# Check the most recent entries in the databases to verify the data was stored
echo -e "\nMost recent entry in API database:"
duckdb "$DATA_DIR/api.duckdb" -c "SELECT id, device_id, captured_at FROM measurements ORDER BY id DESC LIMIT 1;"

echo -e "\nMost recent entry in Ingest database:"
duckdb "$DATA_DIR/ingest.duckdb" -c "SELECT id, device_id, when_captured FROM measurements ORDER BY id DESC LIMIT 1;"

echo -e "\nTest completed!"
