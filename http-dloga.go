// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/log" HTTP topic
package main

import (
	"os"
    "net/http"
	"io/ioutil"
	"strings"
    "fmt"
	"time"
    "io"
    "encoding/json"
)

// Handle inbound HTTP requests to do a quick analysis of a device's log file
func inboundWebDeviceAnalyzeHandler(rw http.ResponseWriter, req *http.Request) {
    stats.Count.HTTP++

    // Set response mime type
    rw.Header().Set("Content-Type", "application/json")

    // Log it
    filename := req.RequestURI[len(TTServerTopicDeviceLog):]
    fmt.Printf("%s LOG ANALYSIS request for %s\n", time.Now().Format(logDateFormat), filename)

    // Open the file
    file := SafecastDirectory() + TTDeviceLogPath + "/" + filename
    fd, err := os.Open(file)
    if err != nil {
        io.WriteString(rw, errorString(err))
        return
    }
    defer fd.Close()

	// Read the log
    contents, err := ioutil.ReadFile(filename)
    if err != nil {
        io.WriteString(rw, errorString(err))
        return
    }

	// Split the contents into a number of slices based on the commas
	splitContents := strings.Split(string(contents), "\n,")
	for _, c := range splitContents {

		// Generate a clean json entry
		clean := strings.Replace(c, "\n", "", -1)
		if (len(clean) == 0) {
			continue
		}

		// Unmarshal it
        value := SafecastData{}
        err = json.Unmarshal([]byte(clean), &value)
		if err != nil {
			fmt.Printf("Unable to unmarshal:\n%s\n", clean)
			continue
		}

		// Write part of it
		if value.Service.UploadedAt != nil {
			io.WriteString(rw, fmt.Sprintf("Uploaded: %s\n", value.Service.UploadedAt))
//			- does a summary of total errors encountered
//			- makes sure it got at least some data from each kind of sensor
//			- makes sure it heard from both lora and fona
//			- does some simple range check on each data value
		}

	}

	// Done

}
