#!/bin/bash
# Debug script for TTServe with DuckDB

# Set your data directory here
DATA_DIR="/home/rob/Documents/Safecast/TTServe"

echo "TTServe Debugging Tool"
echo "======================"

# Check which ports are being used by TTServe
echo "Checking for TTServe process:"
pid=$(pgrep -f "./TTServe")
if [ -n "$pid" ]; then
    echo "TTServe is running with PID: $pid"
    echo "Command: $(ps -p $pid -o args=)"
    echo -e "\nOpen ports:"
    netstat -tuln | grep LISTEN
else
    echo "TTServe does not appear to be running"
fi

# Try multiple ports for TTServe
echo -e "\nTesting TTServe endpoints:"
echo "-------------------------"

# Try port 80
echo "Trying port 80..."
curl -s -o /dev/null -w "%{http_code}" -X GET http://localhost:80/ || echo "Failed to connect"

# Try port 8080
echo -e "\nTrying port 8080..."
curl -s -o /dev/null -w "%{http_code}" -X GET http://localhost:8080/ || echo "Failed to connect"

# Check if the DuckDB files exist
echo -e "\nChecking DuckDB database files:"
echo "-----------------------------"
ls -la "$DATA_DIR/api.duckdb" 2>/dev/null || echo "API database file not found"
ls -la "$DATA_DIR/ingest.duckdb" 2>/dev/null || echo "Ingest database file not found"

echo -e "\nTo send test data to TTServe (after finding the correct port):"
echo "curl -X POST \\"
echo "  -H \"Content-Type: application/json\" \\"
echo "  -d '{\"device_urn\":\"test:device:123\",\"device\":12345,\"when_captured\":\"$(date -u +"%Y-%m-%dT%H:%M:%SZ")\",\"loc_lat\":35.6895,\"loc_lon\":139.6917}' \\"
echo "  http://localhost:PORT/send"

echo -e "\nTo check the DuckDB databases after stopping TTServe:"
echo "1. Stop TTServe (Ctrl+C in the TTServe terminal)"
echo "2. Run: duckdb $DATA_DIR/api.duckdb -c \"SELECT * FROM measurements;\""
echo "3. Run: duckdb $DATA_DIR/ingest.duckdb -c \"SELECT * FROM measurements;\""
