// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/<deviceid>" HTTP topic
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// Handle inbound HTTP requests to fetch log files
func inboundWebDeviceStatusHandler(rw http.ResponseWriter, req *http.Request) {
	stats.Count.HTTP++

	// Set response mime type
	rw.Header().Set("Content-Type", "application/json")

	// Log it
	deviceUID := req.RequestURI[len(TTServerTopicDeviceStatus):]
	fmt.Printf("%s Device information request for %s\n", LogTime(), deviceUID)

	// Open the file
	file := GetDeviceStatusFilePath(deviceUID)
	fd, err := os.Open(file)
	if err != nil {
		io.WriteString(rw, ErrorString(err))
		return
	}
	defer fd.Close()

	// Copy the file to output
	io.Copy(rw, fd)

}

// GenerateDeviceSummaryWebPage generates the web page version of a device summary
func GenerateDeviceSummaryWebPage(rw http.ResponseWriter, contents []byte) {

	// Read the web page template
	page, err := os.ReadFile("./device.html")
	if err != nil {
		io.WriteString(rw, "error reading page\n")
		return
	}

	// Replace the placeholder in the HTML file
	output := strings.Replace(string(page), "{\"abc\":\"def\"}", string(contents), 1)

	// Return the output
	io.WriteString(rw, output)

}
