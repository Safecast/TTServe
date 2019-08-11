// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/log" HTTP topic
package main

import (
    "net/http"
    "encoding/json"
	"io"
)

// Handle inbound HTTP requests to fetch the entire list of devices
func inboundWebDevicesHandler(rw http.ResponseWriter, req *http.Request) {
    stats.Count.HTTP++

	// Get the device info array
	allInfo := devicesSeenInfo()

	// Marshal it
    allInfoJSON, _ := json.Marshal(allInfo)

	// Tell the caller that it's JSON
    rw.Header().Set("Content-Type", "application/json")

	// Output it
	io.WriteString(rw, string(allInfoJSON))

}
