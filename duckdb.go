// Copyright 2025 Safecast.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// DuckDB database implementation for Safecast TTServe
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/marcboeker/go-duckdb"
	ttdata "github.com/Safecast/safecast-go"
)

var (
	// Database connections
	apiDB    *sql.DB
	ingestDB *sql.DB

	// Mutex for thread safety
	apiMutex    sync.Mutex
	ingestMutex sync.Mutex

	// Database paths
	apiDBPath    string
	ingestDBPath string

	// Database initialized flags
	apiDBInitialized    bool
	ingestDBInitialized bool
)

// InitDuckDB initializes the DuckDB databases
func InitDuckDB(dataDir string) error {
	// Set database paths
	apiDBPath = filepath.Join(dataDir, "api.duckdb")
	ingestDBPath = filepath.Join(dataDir, "ingest.duckdb")

	// Initialize API database
	if err := initAPIDatabase(); err != nil {
		return fmt.Errorf("failed to initialize API database: %v", err)
	}

	// Initialize Ingest database
	if err := initIngestDatabase(); err != nil {
		return fmt.Errorf("failed to initialize Ingest database: %v", err)
	}

	return nil
}

// initAPIDatabase initializes the API database for radiation measurements
func initAPIDatabase() error {
	apiMutex.Lock()
	defer apiMutex.Unlock()

	// Open database connection
	connector, err := duckdb.NewConnector(apiDBPath, nil)
	if err != nil {
		return err
	}

	apiDB = sql.OpenDB(connector)

	// Create table for SafecastDataV1 if it doesn't exist
	_, err = apiDB.Exec(`
		CREATE TABLE IF NOT EXISTS measurements (
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
		)
	`)
	if err != nil {
		return err
	}

	// Create sequence for auto-increment if it doesn't exist
	_, err = apiDB.Exec(`
		CREATE SEQUENCE IF NOT EXISTS api_id_seq START WITH 10001
	`)
	if err != nil {
		return err
	}

	apiDBInitialized = true
	return nil
}

// initIngestDatabase initializes the Ingest database for all data
func initIngestDatabase() error {
	ingestMutex.Lock()
	defer ingestMutex.Unlock()

	// Open database connection
	connector, err := duckdb.NewConnector(ingestDBPath, nil)
	if err != nil {
		return err
	}

	ingestDB = sql.OpenDB(connector)

	// Create table for ttdata.SafecastData
	_, err = ingestDB.Exec(`
		CREATE TABLE IF NOT EXISTS measurements (
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
		)
	`)
	if err != nil {
		return err
	}

	// Create sequence for auto-increment if it doesn't exist
	_, err = ingestDB.Exec(`
		CREATE SEQUENCE IF NOT EXISTS ingest_id_seq START WITH 10001
	`)
	if err != nil {
		return err
	}

	ingestDBInitialized = true
	return nil
}

// CloseDuckDB closes the DuckDB database connections
func CloseDuckDB() {
	if apiDB != nil {
		apiDB.Close()
	}
	if ingestDB != nil {
		ingestDB.Close()
	}
}

// StoreInAPIDatabase stores SafecastDataV1 in the API database and returns the generated ID
func StoreInAPIDatabase(data *SafecastDataV1ToEmit) (int64, error) {
	if !apiDBInitialized {
		return 0, fmt.Errorf("API database not initialized")
	}

	apiMutex.Lock()
	defer apiMutex.Unlock()

	// Convert string values to appropriate types
	var deviceID, channelID, originalID, sensorID, stationID, userID *int
	var value, latitude, longitude, height *float64
	var capturedAt *time.Time

	// Parse captured_at
	if data.CapturedAt != nil {
		t, err := time.Parse(time.RFC3339, *data.CapturedAt)
		if err == nil {
			capturedAt = &t
		}
	}

	// Parse numeric values
	if data.DeviceID != nil {
		if id, err := parseInt(*data.DeviceID); err == nil {
			deviceID = &id
		}
	}
	if data.Value != nil {
		if v, err := parseFloat(*data.Value); err == nil {
			value = &v
		}
	}
	if data.Latitude != nil {
		if lat, err := parseFloat(*data.Latitude); err == nil {
			latitude = &lat
		}
	}
	if data.Longitude != nil {
		if lon, err := parseFloat(*data.Longitude); err == nil {
			longitude = &lon
		}
	}
	if data.Height != nil {
		if h, err := parseFloat(*data.Height); err == nil {
			height = &h
		}
	}
	if data.ChannelID != nil {
		if id, err := parseInt(*data.ChannelID); err == nil {
			channelID = &id
		}
	}
	if data.OriginalID != nil {
		if id, err := parseInt(*data.OriginalID); err == nil {
			originalID = &id
		}
	}
	if data.SensorID != nil {
		if id, err := parseInt(*data.SensorID); err == nil {
			sensorID = &id
		}
	}
	if data.StationID != nil {
		if id, err := parseInt(*data.StationID); err == nil {
			stationID = &id
		}
	}
	if data.UserID != nil {
		if id, err := parseInt(*data.UserID); err == nil {
			userID = &id
		}
	}

	// Insert data into the database and get the generated ID
	var id int64
	err := apiDB.QueryRow(`
		INSERT INTO measurements (
			id, captured_at, device_id, value, unit, latitude, longitude, height,
			location_name, channel_id, original_id, sensor_id, station_id, user_id, devicetype_id
		) VALUES (nextval('api_id_seq'), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`,
		capturedAt,
		deviceID,
		value,
		data.Unit,
		latitude,
		longitude,
		height,
		data.LocationName,
		channelID,
		originalID,
		sensorID,
		stationID,
		userID,
		data.DeviceTypeID,
	).Scan(&id)

	return id, err
}

// StoreInIngestDatabase stores ttdata.SafecastData in the Ingest database and returns the generated ID
func StoreInIngestDatabase(data ttdata.SafecastData) (int64, error) {
	if !ingestDBInitialized {
		return 0, fmt.Errorf("Ingest database not initialized")
	}

	ingestMutex.Lock()
	defer ingestMutex.Unlock()

	// Convert data to JSON for storage
	jsonData, err := json.Marshal(data)
	if err != nil {
		return 0, fmt.Errorf("Error marshaling data to JSON: %v", err)
	}

	var capturedAt *time.Time
	if data.CapturedAt != nil {
		t, err := time.Parse(time.RFC3339, *data.CapturedAt)
		if err == nil {
			capturedAt = &t
		}
	}

	// Parse service uploaded_at
	var uploadedAt *time.Time
	if data.Service != nil && data.Service.UploadedAt != nil {
		t, err := time.Parse(time.RFC3339, *data.Service.UploadedAt)
		if err == nil {
			uploadedAt = &t
		}
	}

	// Insert data into the database and get the generated ID
	var id int64
	err = ingestDB.QueryRow(`
		INSERT INTO measurements (
			id, device_urn, device_class, device_sn, device_id, when_captured,
			service_uploaded, service_transport, service_handler, data
		) VALUES (nextval('ingest_id_seq'), ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`,
		data.DeviceUID,
		data.DeviceClass,
		data.DeviceSN,
		data.DeviceID,
		capturedAt,
		uploadedAt,
		getServiceTransport(data),
		getServiceHandler(data),
		string(jsonData),
	).Scan(&id)

	return id, err
}

// Helper functions for parsing values
func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// Helper function to get service transport
func getServiceTransport(data ttdata.SafecastData) *string {
	if data.Service != nil && data.Service.Transport != nil {
		return data.Service.Transport
	}
	return nil
}

// Helper function to get service handler
func getServiceHandler(data ttdata.SafecastData) *string {
	if data.Service != nil && data.Service.Handler != nil {
		return data.Service.Handler
	}
	return nil
}
