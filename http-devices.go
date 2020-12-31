// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/log" HTTP topic
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

// Handle inbound HTTP requests to fetch the entire list of devices
func inboundWebDevicesHandler(rw http.ResponseWriter, req *http.Request) {
	stats.Count.HTTP++

	// Loop over the file system, tracking all devices
	files, err := ioutil.ReadDir(SafecastDirectory() + TTDeviceStatusPath)
	if err != nil {
		fmt.Printf("/devices query error: %s", err)
		io.WriteString(rw, fmt.Sprintf("ReadDir: %s", err))
		return
	}

	// Generate this array
	var allStatus []DeviceStatus

	// Iterate over each of the values
	for _, file := range files {

		// Skip directories
		if file.IsDir() {
			continue
		}

		// Read the file
		contents, err := ioutil.ReadFile(SafecastDirectory() + TTDeviceStatusPath + file.Name())
		fmt.Printf("/devices query error: %s", err)
		if err != nil {
			continue
		}
		dstatus := DeviceStatus{}
		err = json.Unmarshal(contents, &dstatus)
		if err != nil {
			continue
		}

		// Copy only the "current values" to the output, not the historical data
		var ds DeviceStatus
		ds.SafecastData = dstatus.SafecastData
		allStatus = append(allStatus, ds)

	}

	// Marshal it
	allStatusJSON, _ := json.Marshal(allStatus)

	// Tell the caller that it's JSON
	rw.Header().Set("Content-Type", "application/json")

	// Output it
	io.WriteString(rw, string(allStatusJSON))

}
