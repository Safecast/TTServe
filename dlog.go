// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Log file handling
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	ttdata "github.com/Safecast/safecast-go"
)

// DeviceLogSep is the date/time separator
func DeviceLogSep() string {
	return "$"
}

// DeviceLogFilename constructs path of a log file
func DeviceLogFilename(DeviceUID string, Extension string) string {
	fn := time.Now().UTC().Format("2006-01" + DeviceLogSep())
	fn += DeviceUIDFilename(DeviceUID)
	return SafecastDirectory() + TTDeviceLogPath + "/" + fn + Extension
}

// WriteToLogs writes logs.
// Note that we don't do this with a goroutine because the serialization is helpful
// in log-ordering for buffered I/O messages where there are a huge batch of readings
// that are updated in sequence very quickly.
func WriteToLogs(sd ttdata.SafecastData) {
	go trackDevice(sd.DeviceUID, sd.DeviceID, time.Now())
	go WriteDeviceStatus(sd)
	go JSONDeviceLog(sd)
}

// JSONDeviceLog writes the value to the log
func JSONDeviceLog(sd ttdata.SafecastData) {

	file := DeviceLogFilename(sd.DeviceUID, ".json")

	// Open it
	fd, err := os.OpenFile(file, os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {

		// Don't attempt to create it if it already exists
		_, err2 := os.Stat(file)
		if err2 == nil {
			fmt.Printf("Logging: Can't log to %s: %s\n", file, err)
			return
		}
		if err2 == nil {
			if !os.IsNotExist(err2) {
				fmt.Printf("Logging: Ignoring attempt to create %s: %s\n", file, err2)
				return
			}
		}

		// Attempt to create the file because it doesn't already exist
		fd, err = os.OpenFile(file, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			fmt.Printf("Logging: error creating %s: %s\n", file, err)
			return
		}

	}

	// Turn stats into a safe string writing
	if sd.Service == nil {
		var svc ttdata.Service
		sd.Service = &svc
	}
	scJSON, _ := json.Marshal(sd)
	fd.WriteString(string(scJSON))
	fd.WriteString("\r\n,\r\n")

	// Close and exit
	fd.Close()

}

// DeleteLogs clears the logs
func DeleteLogs(DeviceUID string) string {

	jsonFilename := DeviceLogFilename(DeviceUID, ".json")

	deleted := false
	err := os.Remove(SafecastDirectory() + jsonFilename)
	if err == nil {
		deleted = true
	}

	if !deleted {
		return fmt.Sprintf("Nothing for %d to be cleared.", DeviceUID)
	}

	return fmt.Sprintf("Device logs for %d have been deleted.", DeviceUID)

}
