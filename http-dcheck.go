// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/check" HTTP topic
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	ttdata "github.com/Safecast/safecast-go"
)

// Handle inbound HTTP requests to do a quick analysis of a device's log file
func inboundWebDeviceCheckHandler(rw http.ResponseWriter, req *http.Request) {
	stats.Count.HTTP++

	// Set response mime type
	rw.Header().Set("Content-Type", "application/json")

	// Log it
	deviceidstr := req.RequestURI[len(TTServerTopicDeviceCheck):]
	timeRange := time.Now().UTC().Format("2006-01")
	filename := fmt.Sprintf("%s/%s$%s.json", TTDeviceLogPath, timeRange, DeviceUIDFilename(deviceidstr))

	fmt.Printf("%s LOG ANALYSIS request for %s\n", LogTime(), filename)

	// Check it
	success, s := CheckJSON(SafecastDirectory() + filename)
	if !success {
		io.WriteString(rw, s)
	}

	// Done
	io.WriteString(rw, s)

}

// CheckJSON performs a standard check on a JSON file
func CheckJSON(infile string) (success bool, result string) {

	// Read the log
	contents, err := os.ReadFile(infile)
	if err != nil {
		return false, ErrorString(err)
	}

	// Begin taking stats
	stats := NewMeasurementDataset()

	// Split the contents into a number of slices based on the commas
	ctmp := strings.Replace(string(contents), "\n,", ",\n", -1)
	splitContents := strings.Split(ctmp, ",\n")
	for _, c := range splitContents {

		// Generate a clean json entry
		clean := strings.Replace(c, "\n", "", -1)
		if len(clean) == 0 {
			continue
		}

		// Unmarshal it.  Badly-formatted json occasionally occurs because of
		// concurrent file writes to the log from different process instances,
		// but this is rare - so no worry.
		value := ttdata.SafecastData{}
		err = json.Unmarshal([]byte(clean), &value)
		if err != nil {
			fmt.Printf("CHECK: error unmarshaling data: %s\n", err)
			continue
		}

		// Take a measurement
		MeasurementStat := CheckMeasurement(value)

		// Aggregate statistics
		AggregateMeasurementIntoDataset(&stats, MeasurementStat)

	}

	// Measurements completed
	AggregationCompleted(&stats)

	// Generate the summary of the aggregation
	s := GenerateDatasetSummary(stats)

	// Done
	return true, s

}
