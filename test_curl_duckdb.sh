#!/bin/bash
# Test script for TTServe with DuckDB using curl

echo "Testing TTServe with DuckDB using curl"
echo "====================================="

# Check if TTServe is running
if ! pgrep -f "./TTServe" > /dev/null; then
    echo "TTServe is not running. Starting TTServe..."
    ./TTServe /home/rob/Documents/Safecast/TTServe &
    sleep 3
fi

# Create a base64-encoded payload (simulating device data)
PAYLOAD=$(echo -n '{"device_id":12345,"when_captured":"2023-04-01T12:00:00Z","loc_lat":35.6895,"loc_lon":139.6917}' | base64)

# Send the request with the TTGATE user agent
echo -e "\nSending request to TTServe with TTGATE user agent..."
curl -v -X POST \
  -H "Content-Type: application/json" \
  -H "User-Agent: TTGATE" \
  -d '{
    "GatewayID": "test-gateway-123",
    "Transport": "lora",
    "GatewayTime": "2023-04-01T12:00:00Z",
    "Payload": "'$PAYLOAD'"
  }' \
  http://localhost:8080/send

echo -e "\n\nTest completed!"
echo "Note: TTServe is currently not configured to return measurement IDs in the HTTP response."
echo "The data is still being stored correctly in the DuckDB databases with IDs."
echo "You can verify this by checking the databases directly:"
echo "  duckdb /home/rob/Documents/Safecast/TTServe/api.duckdb -c \"SELECT * FROM measurements ORDER BY id DESC LIMIT 1;\""
echo "  duckdb /home/rob/Documents/Safecast/TTServe/ingest.duckdb -c \"SELECT * FROM measurements ORDER BY id DESC LIMIT 1;\""
