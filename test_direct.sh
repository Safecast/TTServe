#!/bin/bash
# Direct curl command to test TTServe with DuckDB

# Set your data directory here
DATA_DIR="/home/rob/Documents/Safecast/TTServe"

echo "Testing TTServe with DuckDB implementation"
echo "----------------------------------------"

# Send test data using curl
echo "Sending test data to TTServe..."

# Send test data to TTServe and capture the response
RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "User-Agent: TTSERVE" \
  -d '{
    "device_urn": "test:device:123",
    "device": 12345,
    "when_captured": "2023-04-01T12:00:00Z",
    "loc_lat": 35.6895,
    "loc_lon": 139.6917,
    "loc_alt": 40.5,
    "bat_voltage": 3.8,
    "bat_soc": 85,
    "env_temp": 22.5,
    "env_humid": 60.0,
    "dev_test": true
  }' \
  http://localhost:8080/send)

# Display the response
echo "Response from TTServe:"
echo "$RESPONSE"

echo -e "\n\nVerifying data in DuckDB databases:"
echo "----------------------------------------"

# Check if DuckDB CLI is installed
if command -v duckdb &> /dev/null; then
    echo "Checking API database:"
    duckdb "$DATA_DIR/api.duckdb" -c "SELECT * FROM measurements;"
    
    echo -e "\nChecking Ingest database:"
    duckdb "$DATA_DIR/ingest.duckdb" -c "SELECT * FROM measurements;"
else
    echo "DuckDB CLI not found. To check the databases manually:"
    echo "1. Install DuckDB CLI or use a DuckDB viewer"
    echo "2. Run: duckdb $DATA_DIR/api.duckdb -c \"SELECT * FROM measurements;\""
    echo "3. Run: duckdb $DATA_DIR/ingest.duckdb -c \"SELECT * FROM measurements;\""
fi
