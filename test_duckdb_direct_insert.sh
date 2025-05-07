#!/bin/bash

# Set up variables
DATA_DIR="/home/rob/Documents/Safecast/TTServe"
DEVICE_ID=87654321  # Unique device ID to easily identify our test data
TIMESTAMP="2023-04-01 12:00:00"

echo "=== Testing DuckDB Direct Insert ==="
echo "1. Stopping any running TTServe instances..."
pkill -9 TTServe

echo "2. Checking current state of databases..."
echo "API Database:"
duckdb "$DATA_DIR/api.duckdb" -c "SELECT COUNT(*) FROM measurements;"
echo "Ingest Database:"
duckdb "$DATA_DIR/ingest.duckdb" -c "SELECT COUNT(*) FROM measurements;"

echo "3. Directly inserting test data into API database..."
duckdb "$DATA_DIR/api.duckdb" -c "
INSERT INTO measurements (
    id,
    device_id,
    captured_at,
    value,
    unit,
    latitude,
    longitude
) VALUES (
    10000,
    $DEVICE_ID,
    '$TIMESTAMP',
    42.0,
    'cpm',
    35.6895,
    139.6917
);"

echo "4. Directly inserting test data into Ingest database..."
duckdb "$DATA_DIR/ingest.duckdb" -c "
INSERT INTO measurements (
    id,
    device_id,
    when_captured,
    data
) VALUES (
    10000,
    $DEVICE_ID,
    '$TIMESTAMP',
    '{\"device_id\":$DEVICE_ID,\"when_captured\":\"$TIMESTAMP\",\"loc_lat\":35.6895,\"loc_lon\":139.6917,\"value\":42.0,\"unit\":\"cpm\"}'
);"

echo "5. Verifying data was stored correctly..."
echo "API Database (looking for device ID $DEVICE_ID):"
duckdb "$DATA_DIR/api.duckdb" -c "SELECT id, device_id, captured_at, value, unit, latitude, longitude FROM measurements WHERE device_id = $DEVICE_ID ORDER BY id DESC LIMIT 5;"

echo "Ingest Database (looking for device ID $DEVICE_ID):"
duckdb "$DATA_DIR/ingest.duckdb" -c "SELECT id, device_id, when_captured, data FROM measurements WHERE device_id = $DEVICE_ID ORDER BY id DESC LIMIT 5;"

echo "=== Test Complete ==="
