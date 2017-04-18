// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/check" HTTP topic
package main

import (
    "net/http"
	"io/ioutil"
	"strings"
    "fmt"
	"time"
    "io"
    "encoding/json"
)

// Handle inbound HTTP requests to do a quick analysis of a device's log file
func inboundWebDeviceCheckHandler(rw http.ResponseWriter, req *http.Request) {
    stats.Count.HTTP++

    // Set response mime type
    rw.Header().Set("Content-Type", "application/json")

    // Log it
    deviceidstr := req.RequestURI[len(TTServerTopicDeviceCheck):]
	timeRange := time.Now().UTC().Format("2006-01")
    filename := fmt.Sprintf("%s/%s-%s.json", TTDeviceLogPath, timeRange, deviceidstr)

    fmt.Printf("%s LOG ANALYSIS request for %s\n", logTime(), filename)

	// Check it
	success, s := CheckJSON(SafecastDirectory() + filename, deviceidstr)
	if !success {
		io.WriteString(rw, s)
	}
	
	// Done
	io.WriteString(rw, s)

}

// Check a JSON file
func CheckJSON(infile string, outfile string) (success bool, result string) {
	
	// Read the log
    contents, err := ioutil.ReadFile(infile)
    if err != nil {
        return false, errorString(err)
    }

	// Begin taking stats
	stats := NewMeasurementDataset(outfile)

	// Split the contents into a number of slices based on the commas
	splitContents := strings.Split(string(contents), "\n,")
	for _, c := range splitContents {

		// Generate a clean json entry
		clean := strings.Replace(c, "\n", "", -1)
		if len(clean) == 0 {
			continue
		}

		// Unmarshal it.  Badly-formatted json occasionally occurs because of
		// concurrent file writes to the log from different process instances,
		// but this is rare - so no worry.
        value := SafecastData{}
        err = json.Unmarshal([]byte(clean), &value)
		if err != nil {
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
