// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/<deviceid>" HTTP topic
package main

import (
	"os"
	"strings"
    "io/ioutil"
    "net/http"
    "fmt"
    "io"
)

// Handle inbound HTTP requests to fetch log files
func inboundWebDeviceStatusHandler(rw http.ResponseWriter, req *http.Request) {
    stats.Count.HTTP++

    // Set response mime type
    rw.Header().Set("Content-Type", "application/json")

    // Log it
    device := req.RequestURI[len(TTServerTopicDeviceStatus):]
	valid, deviceid := WordsToNumber(device)
	if !valid {
		return;
	}
	filename := fmt.Sprintf("%d.json", deviceid)
    fmt.Printf("%s Device information request for %s\n", logTime(), filename)

    // Open the file
    file := SafecastDirectory() + TTDeviceStatusPath + "/" + filename
    fd, err := os.Open(file)
    if err != nil {
        io.WriteString(rw, errorString(err))
        return
    }
    defer fd.Close()

    // Copy the file to output
    io.Copy(rw, fd)

}

// Method to generate the web page version of a device summary
func GenerateDeviceSummaryWebPage(rw http.ResponseWriter, contents []byte) {

	// Read the web page template
    page, err := ioutil.ReadFile("./device.html")
	if err != nil {
		io.WriteString(rw, "error reading page\n")
		return
	}

	// Replace the placeholder in the HTML file
	output := strings.Replace(string(page), "{\"abc\":\"def\"}", string(contents), 1)

	// Return the output
	io.WriteString(rw, output)

}
