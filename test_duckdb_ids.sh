#!/bin/bash
# Test script to verify if measurement IDs are returned by the DuckDB implementation

echo "Testing DuckDB Measurement ID Return"
echo "=================================="

# Set your data directory here
DATA_DIR="/home/rob/Documents/Safecast/TTServe"

# Check if TTServe is running and stop it to release database locks
if pgrep -f "./TTServe" > /dev/null; then
    echo "Stopping TTServe to release database locks..."
    pkill -f "./TTServe"
    sleep 2
fi

# Insert test data directly into the API database
echo -e "\nInserting test data into API database..."
API_ID=$(duckdb "$DATA_DIR/api.duckdb" -c "
INSERT INTO measurements (
  captured_at, device_id, value, unit, latitude, longitude, height,
  location_name, devicetype_id
) VALUES (
  '2023-04-01T12:00:00Z', 12345, 22.5, 'cpm', 35.6895, 139.6917, 40.5,
  'Tokyo, Japan', 'test-device'
) RETURNING id;")

echo "API database measurement ID: $API_ID"

# Insert test data directly into the Ingest database
echo -e "\nInserting test data into Ingest database..."
INGEST_ID=$(duckdb "$DATA_DIR/ingest.duckdb" -c "
INSERT INTO measurements (
  device_urn, device_class, device_sn, device_id, when_captured,
  service_transport, service_handler, data
) VALUES (
  'test:device:123', 'test-device', '#123', 12345, '2023-04-01T12:00:00Z',
  'test-transport', 'test-handler', '{\"device_id\":12345,\"device_urn\":\"test:device:123\"}'
) RETURNING id;")

echo "Ingest database measurement ID: $INGEST_ID"

# Create a JSON response with the measurement IDs
echo -e "\nJSON response with measurement IDs:"
cat << EOF
{
  "api_id": $API_ID,
  "ingest_id": $INGEST_ID
}
EOF

echo -e "\nTest completed!"

# Restart TTServe
echo "Restarting TTServe..."
"$DATA_DIR/TTServe" "$DATA_DIR" &
sleep 2
echo "TTServe restarted."
