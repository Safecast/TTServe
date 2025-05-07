#!/bin/bash

# Set up variables
DATA_DIR="/home/rob/Documents/Safecast/TTServe"

echo "=== Fixing DuckDB Schema to Use Auto-Increment IDs ==="
echo "1. Stopping any running TTServe instances..."
pkill -9 TTServe

echo "2. Backing up current databases..."
cp "$DATA_DIR/api.duckdb" "$DATA_DIR/api.duckdb.bak"
cp "$DATA_DIR/ingest.duckdb" "$DATA_DIR/ingest.duckdb.bak"

echo "3. Creating sequences for auto-increment..."
echo "API Database:"
duckdb "$DATA_DIR/api.duckdb" -c "
-- Create a sequence for auto-increment
CREATE SEQUENCE IF NOT EXISTS api_id_seq START WITH 10001;
"

echo "Ingest Database:"
duckdb "$DATA_DIR/ingest.duckdb" -c "
-- Create a sequence for auto-increment
CREATE SEQUENCE IF NOT EXISTS ingest_id_seq START WITH 10001;
"

echo "4. Testing auto-increment functionality..."
echo "API Database:"
API_ID=$(duckdb "$DATA_DIR/api.duckdb" -c "
INSERT INTO measurements (
    id,
    device_id,
    captured_at,
    value,
    unit,
    latitude,
    longitude
) VALUES (
    nextval('api_id_seq'),
    99999,
    '2023-04-01 12:00:00',
    42.0,
    'cpm',
    35.6895,
    139.6917
) RETURNING id;")

echo "API Database auto-assigned ID: $API_ID"

echo "Ingest Database:"
INGEST_ID=$(duckdb "$DATA_DIR/ingest.duckdb" -c "
INSERT INTO measurements (
    id,
    device_id,
    when_captured,
    data
) VALUES (
    nextval('ingest_id_seq'),
    99999,
    '2023-04-01 12:00:00',
    '{\"device_id\":99999,\"when_captured\":\"2023-04-01 12:00:00\",\"loc_lat\":35.6895,\"loc_lon\":139.6917,\"value\":42.0,\"unit\":\"cpm\"}'
) RETURNING id;")

echo "Ingest Database auto-assigned ID: $INGEST_ID"

echo "5. Verifying data was stored correctly..."
echo "API Database (looking for device ID 99999):"
duckdb "$DATA_DIR/api.duckdb" -c "SELECT id, device_id, captured_at, value, unit, latitude, longitude FROM measurements WHERE device_id = 99999 ORDER BY id DESC LIMIT 5;"

echo "Ingest Database (looking for device ID 99999):"
duckdb "$DATA_DIR/ingest.duckdb" -c "SELECT id, device_id, when_captured, data FROM measurements WHERE device_id = 99999 ORDER BY id DESC LIMIT 5;"

echo "=== Schema Fix Complete ==="
