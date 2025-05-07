#!/bin/bash
# Simple curl test for TTServe with DuckDB

echo "Simple curl test for TTServe with DuckDB"
echo "======================================"

# Create a base64-encoded payload (simulating device data)
PAYLOAD=$(echo -n '{"device_id":12345,"when_captured":"2023-04-01T12:00:00Z","loc_lat":35.6895,"loc_lon":139.6917}' | base64)

# Send the request with the TTGATE user agent
echo -e "\nSending request to TTServe with TTGATE user agent..."
curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "User-Agent: TTGATE" \
  -d "{\"GatewayID\":\"test-gateway-123\",\"Transport\":\"lora\",\"GatewayTime\":\"2023-04-01T12:00:00Z\",\"Payload\":\"$PAYLOAD\"}" \
  http://localhost:8080/send

echo -e "\n\nVerifying data was stored in the databases:"
echo -e "\nAPI database (latest record):"
duckdb /home/rob/Documents/Safecast/TTServe/api.duckdb -c "SELECT id, device_id, captured_at FROM measurements ORDER BY id DESC LIMIT 1;"

echo -e "\nIngest database (latest record):"
duckdb /home/rob/Documents/Safecast/TTServe/ingest.duckdb -c "SELECT id, device_id, when_captured FROM measurements ORDER BY id DESC LIMIT 1;"

echo -e "\nTest completed!"
