#!/bin/bash

# Set up variables
DATA_DIR="/home/rob/Documents/Safecast/TTServe"

echo "=== Fixing DuckDB Schema to Use Auto-Increment IDs ==="
echo "1. Stopping any running TTServe instances..."
pkill -9 TTServe

echo "2. Backing up current databases..."
cp "$DATA_DIR/api.duckdb" "$DATA_DIR/api.duckdb.bak"
cp "$DATA_DIR/ingest.duckdb" "$DATA_DIR/ingest.duckdb.bak"

echo "3. Modifying API database schema..."
duckdb "$DATA_DIR/api.duckdb" -c "
-- Create a temporary table with the new schema
CREATE TABLE measurements_new (
    id INTEGER PRIMARY KEY,
    captured_at TIMESTAMP,
    device_id INTEGER,
    value DOUBLE,
    unit VARCHAR,
    latitude DOUBLE,
    longitude DOUBLE,
    height DOUBLE,
    location_name VARCHAR,
    channel_id INTEGER,
    original_id INTEGER,
    sensor_id INTEGER,
    station_id INTEGER,
    user_id INTEGER,
    devicetype_id VARCHAR,
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Copy data from the old table to the new table
INSERT INTO measurements_new (
    id, captured_at, device_id, value, unit, latitude, longitude, height, 
    location_name, channel_id, original_id, sensor_id, station_id, user_id, 
    devicetype_id, uploaded_at
)
SELECT 
    id, captured_at, device_id, value, unit, latitude, longitude, height, 
    location_name, channel_id, original_id, sensor_id, station_id, user_id, 
    devicetype_id, uploaded_at
FROM measurements;

-- Drop the old table
DROP TABLE measurements;

-- Rename the new table to the original name
ALTER TABLE measurements_new RENAME TO measurements;

-- Create a sequence for auto-increment
CREATE SEQUENCE IF NOT EXISTS measurements_id_seq START WITH 10001;

-- Create a function to use the sequence
CREATE OR REPLACE FUNCTION next_id() RETURNS INTEGER AS '
    SELECT nextval(''measurements_id_seq'')
';
"

echo "4. Modifying Ingest database schema..."
duckdb "$DATA_DIR/ingest.duckdb" -c "
-- Create a temporary table with the new schema
CREATE TABLE measurements_new (
    id INTEGER PRIMARY KEY,
    device_urn VARCHAR,
    device_class VARCHAR,
    device_sn VARCHAR,
    device_id INTEGER,
    when_captured TIMESTAMP,
    service_uploaded TIMESTAMP,
    service_transport VARCHAR,
    service_handler VARCHAR,
    data JSON,
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Copy data from the old table to the new table
INSERT INTO measurements_new (
    id, device_urn, device_class, device_sn, device_id, when_captured,
    service_uploaded, service_transport, service_handler, data, uploaded_at
)
SELECT 
    id, device_urn, device_class, device_sn, device_id, when_captured,
    service_uploaded, service_transport, service_handler, data, uploaded_at
FROM measurements;

-- Drop the old table
DROP TABLE measurements;

-- Rename the new table to the original name
ALTER TABLE measurements_new RENAME TO measurements;

-- Create a sequence for auto-increment
CREATE SEQUENCE IF NOT EXISTS measurements_id_seq START WITH 10001;

-- Create a function to use the sequence
CREATE OR REPLACE FUNCTION next_id() RETURNS INTEGER AS '
    SELECT nextval(''measurements_id_seq'')
';
"

echo "5. Testing auto-increment functionality..."
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
    next_id(),
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
    next_id(),
    99999,
    '2023-04-01 12:00:00',
    '{\"device_id\":99999,\"when_captured\":\"2023-04-01 12:00:00\",\"loc_lat\":35.6895,\"loc_lon\":139.6917,\"value\":42.0,\"unit\":\"cpm\"}'
) RETURNING id;")

echo "Ingest Database auto-assigned ID: $INGEST_ID"

echo "6. Verifying data was stored correctly..."
echo "API Database (looking for device ID 99999):"
duckdb "$DATA_DIR/api.duckdb" -c "SELECT id, device_id, captured_at, value, unit, latitude, longitude FROM measurements WHERE device_id = 99999 ORDER BY id DESC LIMIT 5;"

echo "Ingest Database (looking for device ID 99999):"
duckdb "$DATA_DIR/ingest.duckdb" -c "SELECT id, device_id, when_captured, data FROM measurements WHERE device_id = 99999 ORDER BY id DESC LIMIT 5;"

echo "=== Schema Fix Complete ==="
