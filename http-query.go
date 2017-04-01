// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/query-results" HTTP topic
package main

import (
	"os"
    "net/http"
    "fmt"
    "io"
)

// Handle inbound HTTP requests to fetch log files
func inboundWebQueryResultsHandler(rw http.ResponseWriter, req *http.Request) {
    stats.Count.HTTP++

    // Log it
    filename := req.RequestURI[len(TTServerTopicQueryResults):]
    fmt.Printf("%s Query results request for %s\n", logTime(), filename)

    // Open the file
    file := SafecastDirectory() + TTInfluxQueryPath + "/" + filename
    fd, err := os.Open(file)
    if err != nil {
        io.WriteString(rw, errorString(err))
        return
    }
    defer fd.Close()

	// Force a download
	rw.Header().Set("Content-Disposition", "attachment; filename=" + filename)
	rw.Header().Set("Content-Type", "application/octet-stream")
	
    // Copy the file to output
    io.Copy(rw, fd)

}
