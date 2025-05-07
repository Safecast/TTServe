#!/bin/bash
# Simple test script to verify if measurement IDs are returned by the DuckDB implementation

echo "Simple DuckDB Measurement ID Test"
echo "================================"

# Set your data directory here
DATA_DIR="/home/rob/Documents/Safecast/TTServe"

# Check if TTServe is running and stop it to release database locks
if pgrep -f "./TTServe" > /dev/null; then
    echo "Stopping TTServe to release database locks..."
    pkill -9 -f "./TTServe"
    sleep 2
fi

# Verify the database files exist
echo -e "\nVerifying database files..."
ls -la "$DATA_DIR/api.duckdb" "$DATA_DIR/ingest.duckdb"

# Insert test data directly into the API database with a simple query
echo -e "\nInserting test data into API database..."
API_ID=$(duckdb "$DATA_DIR/api.duckdb" -c "
INSERT INTO measurements (
  captured_at, device_id, value, unit, latitude, longitude
) VALUES (
  '2023-04-01T12:00:00Z', 12345, 22.5, 'cpm', 35.6895, 139.6917
) RETURNING id;")

echo "API database measurement ID: $API_ID"

# Insert test data directly into the Ingest database with a simple query
echo -e "\nInserting test data into Ingest database..."
INGEST_ID=$(duckdb "$DATA_DIR/ingest.duckdb" -c "
INSERT INTO measurements (
  device_id, when_captured, data
) VALUES (
  12345, '2023-04-01T12:00:00Z', '{\"device_id\":12345}'
) RETURNING id;")

echo "Ingest database measurement ID: $INGEST_ID"

# Create a JSON response with the measurement IDs
echo -e "\nJSON response with measurement IDs:"
echo "{
  \"api_id\": $API_ID,
  \"ingest_id\": $INGEST_ID
}"

echo -e "\nTest completed!"
