#!/bin/bash
# Direct test for DuckDB databases

echo "Direct DuckDB Database Test"
echo "=========================="

# Set your data directory here
DATA_DIR="/home/rob/Documents/Safecast/TTServe"

# Make sure TTServe is not running (to avoid database locks)
echo "Stopping any running TTServe instances..."
pkill -f "./TTServe" 2>/dev/null
sleep 2

# Check if the DuckDB databases exist
echo "Checking DuckDB database files:"
ls -la "$DATA_DIR/api.duckdb" "$DATA_DIR/ingest.duckdb"

# Insert test data directly into the API database
echo -e "\nInserting test data into API database..."
duckdb "$DATA_DIR/api.duckdb" -c "
INSERT INTO measurements (
  id, captured_at, device_id, value, unit, latitude, longitude, height,
  location_name, devicetype_id
) VALUES (
  1, '2023-04-01T12:00:00Z', 12345, 22.5, 'cpm', 35.6895, 139.6917, 40.5,
  'Tokyo, Japan', 'test-device'
);"

# Insert test data directly into the Ingest database
echo -e "\nInserting test data into Ingest database..."
duckdb "$DATA_DIR/ingest.duckdb" -c "
INSERT INTO measurements (
  id, device_urn, device_class, device_sn, device_id, when_captured,
  service_transport, service_handler, data
) VALUES (
  1, 'test:device:123', 'test-device', '#123', 12345, '2023-04-01T12:00:00Z',
  'test-transport', 'test-handler', '{\"device_id\":12345,\"device_urn\":\"test:device:123\"}'
);"

# Verify the data in the API database
echo -e "\nVerifying data in API database:"
duckdb "$DATA_DIR/api.duckdb" -c "SELECT * FROM measurements;"

# Verify the data in the Ingest database
echo -e "\nVerifying data in Ingest database:"
duckdb "$DATA_DIR/ingest.duckdb" -c "SELECT * FROM measurements;"

echo -e "\nTest completed!"
