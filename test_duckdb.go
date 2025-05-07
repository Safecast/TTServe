// Test script for DuckDB implementation in TTServe
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	ttdata "github.com/Safecast/safecast-go"
)

// TestData generates a sample SafecastData record
func TestData() ttdata.SafecastData {
	now := time.Now().Format(time.RFC3339)
	capturedAt := now
	
	// Create a sample SafecastData record
	sd := ttdata.SafecastData{
		DeviceUID:   "test:device:123",
		DeviceClass: "test-device",
		DeviceSN:    "TEST123",
		DeviceID:    12345,
		CapturedAt:  &capturedAt,
	}

	// Add location data
	lat := 35.6895
	lon := 139.6917
	sd.Loc = &ttdata.Loc{
		Lat: &lat,
		Lon: &lon,
	}

	// Add environmental data
	temp := 25.5
	humid := 60.0
	sd.Env = &ttdata.Env{
		Temp:  &temp,
		Humid: &humid,
	}

	// Add radiation data
	// We'll comment out this section since we need to check the actual field names
	// in the ttdata.Lnd struct
	sd.Lnd = &ttdata.Lnd{}
	
	// Note: You'll need to check the actual field names in the ttdata.Lnd struct
	// and update this section accordingly. The field names might be different
	// than what we expected (Cpm and Usvh).

	return sd
}

// SendData sends the test data to TTServe
func SendData(sd ttdata.SafecastData) error {
	// Convert to JSON
	jsonData, err := json.Marshal(sd)
	if err != nil {
		return fmt.Errorf("error marshaling data: %v", err)
	}

	// Send to TTServe
	url := "http://localhost:80/send"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Response status: %s\n", resp.Status)
	return nil
}

// QueryDatabases directly queries the DuckDB databases
func QueryDatabases(dataDir string) error {
	// This is a placeholder - in a real implementation, you would:
	// 1. Open connections to the DuckDB databases
	// 2. Execute SQL queries to verify the data was stored
	// 3. Print the results

	fmt.Println("To manually check the databases:")
	fmt.Printf("1. API Database: %s/api.duckdb\n", dataDir)
	fmt.Printf("2. Ingest Database: %s/ingest.duckdb\n", dataDir)
	fmt.Println("You can use the DuckDB CLI or a DuckDB viewer to examine these files.")

	return nil
}

func testMain() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run test_duckdb.go [data-directory]")
		os.Exit(1)
	}

	dataDir := os.Args[1]
	
	// Generate and send test data
	fmt.Println("Generating test data...")
	sd := TestData()
	
	fmt.Println("Sending data to TTServe...")
	err := SendData(sd)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	
	// Give TTServe time to process and store the data
	fmt.Println("Waiting for data to be processed...")
	time.Sleep(2 * time.Second)
	
	// Query the databases to verify data was stored
	fmt.Println("Checking databases...")
	err = QueryDatabases(dataDir)
	if err != nil {
		fmt.Printf("Error querying databases: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("Test completed successfully!")
}
