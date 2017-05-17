// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/log" HTTP topic
package main

import (
	"os"
	"strings"
    "net/http"
    "fmt"
    "io"
)

// Handle inbound HTTP requests to fetch log files
func inboundWebDeviceLogHandler(rw http.ResponseWriter, req *http.Request) {
    stats.Count.HTTP++

    // Log it
    filename := req.RequestURI[len(TTServerTopicDeviceLog):]
    fmt.Printf("%s LOG request for %s\n", LogTime(), filename)

	// Break down the components of what's requested
	components := strings.Split(filename, ".")
	if (len(components) != 2) {
		io.WriteString(rw, "Unrecognized device ID\n");
		return
	}
	extension := components[1]
	
    // Open the file
    file := SafecastDirectory() + TTDeviceLogPath + "/" + filename
    fd, err := os.Open(file)
    if err != nil {
        io.WriteString(rw, ErrorString(err))
        return
    }
    defer fd.Close()

	// If this is CSV, force a download
	if extension == "csv" {
		rw.Header().Set("Content-Disposition", "attachment; filename=" + filename)
		rw.Header().Set("Content-Type", "application/octet-stream")
	} else {
	    rw.Header().Set("Content-Type", "application/json")
	}
	
    // Copy the file to output
    io.Copy(rw, fd)

}
