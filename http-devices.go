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
	"strconv"
)

// Handle inbound HTTP requests to fetch the entire list of devices
func inboundWebDevicesHandler(rw http.ResponseWriter, req *http.Request) {
	stats.Count.HTTP++

	// Parse arguments to extract offset and count
	_, args, err := HTTPArgs(req, TTServerTopicDevices)
	if err != nil {
		io.WriteString(rw, fmt.Sprintf("%s", err))
		return
	}
	offset, _ := strconv.Atoi(args["offset"])
	count, _ := strconv.Atoi(args["count"])
	if count == 0 {
		offset = 0
	}

	// Loop over the file system, tracking all devices
	files, err := ioutil.ReadDir(SafecastDirectory() + TTDeviceStatusPath)
	if err != nil {
		io.WriteString(rw, fmt.Sprintf("%s", err))
		return
	}

	// Generate this array
	var allStatus []SafecastData

	// Iterate over each of the values
	for _, file := range files {

		// Skip directories
		if file.IsDir() {
			continue
		}

		// Skip if we're still processing an offset
		if offset > 0 {
			offset--
			continue
		}

		// Read the file
		contents, err := ioutil.ReadFile(SafecastDirectory() + TTDeviceStatusPath + "/" + file.Name())
		if err != nil {
			continue
		}
		dstatus := DeviceStatus{}
		err = json.Unmarshal(contents, &dstatus)
		if err != nil {
			continue
		}

		// Copy only the "current values" to the output, not the historical data
		allStatus = append(allStatus, dstatus.SafecastData)

		// Stop if we're processing count
		if count > 0 {
			count--
			if count == 0 {
				break
			}
		}

	}

	// Marshal it
	allStatusJSON, _ := json.Marshal(allStatus)

	// Tell the caller that it's JSON
	rw.Header().Set("Content-Type", "application/json")

	// Output it
	io.WriteString(rw, string(allStatusJSON))

}
