#!/bin/bash
# Test script for sending data to TTServe using curl

# Set the TTServe URL (adjust if running on a different port)
TTSERVE_URL="http://localhost:80/send"

# Create a sample JSON payload for a Safecast measurement
cat > test_payload.json << EOL
{
  "device_urn": "test:device:123",
  "device_class": "test-device",
  "device_sn": "TEST123",
  "device": 12345,
  "when_captured": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "loc_lat": 35.6895,
  "loc_lon": 139.6917,
  "loc_alt": 40.5,
  "env_temp": 25.5,
  "env_humid": 60.0,
  "lnd_cpm": 42,
  "lnd_usvh": 0.12,
  "dev_test": true
}
EOL

echo "Sending test data to TTServe..."
curl -X POST \
  -H "Content-Type: application/json" \
  -d @test_payload.json \
  $TTSERVE_URL

echo -e "\n\nData sent. To check the DuckDB databases:"
echo "1. Use the DuckDB CLI:"
echo "   duckdb [data-directory]/api.duckdb"
echo "   duckdb [data-directory]/ingest.duckdb"
echo "2. Run SQL queries like: SELECT * FROM measurements;"
