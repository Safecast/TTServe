package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Safecast/safecast-go/ttdata"
)

// Test function to verify that measurement IDs are returned by the DuckDB implementation
func main() {
	// Initialize the DuckDB databases
	err := InitializeDuckDB("/home/rob/Documents/Safecast/TTServe")
	if err != nil {
		fmt.Printf("Error initializing DuckDB databases: %v\n", err)
		return
	}

	// Create a test SafecastData struct
	sd := ttdata.SafecastData{
		DeviceUID:   "test:device:123",
		DeviceClass: "test-device",
		DeviceSN:    "#123",
		DeviceID:    12345,
	}

	// Set the capture time
	captureTime := time.Now().UTC().Format(time.RFC3339)
	sd.CapturedAt = &captureTime

	// Set location data
	lat := 35.6895
	lon := 139.6917
	alt := 40.5
	sd.Loc = &ttdata.Loc{
		Lat: &lat,
		Lon: &lon,
		Alt: &alt,
	}

	// Upload the data and get the measurement IDs
	fmt.Println("Uploading test data to DuckDB...")
	ids, err := Upload(sd)
	if err != nil {
		fmt.Printf("Error uploading data: %v\n", err)
		return
	}

	// Print the measurement IDs
	fmt.Println("Measurement IDs returned by Upload function:")
	idsJSON, _ := json.MarshalIndent(ids, "", "  ")
	fmt.Println(string(idsJSON))

	fmt.Println("Test completed successfully!")
}
