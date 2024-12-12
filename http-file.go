// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/file" HTTP topic
package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Handle inbound HTTP requests to fetch log files
func inboundWebFileHandler(rw http.ResponseWriter, req *http.Request) {
	stats.Count.HTTP++

	// Log it
	filename := req.RequestURI[len(TTServerTopicFile):]
	fmt.Printf("%s FILE request for %s\n", LogTime(), filename)

	// Open the file
	file := SafecastDirectory() + TTFilePath + "/" + filename
	contents, err := os.ReadFile(file)
	if err != nil {
		http.Error(rw, fmt.Sprintf("%s", err), http.StatusNotFound)
		return
	}

	// Write the file to the HTTPS client as binary, with its original filename
	rw.Header().Set("Content-Disposition", "attachment; filename="+filename)
	rw.Header().Set("Content-Type", "application/octet-stream")
	rw.Header().Set("Content-Length", fmt.Sprintf("%d", len(contents)))

	// Copy the file to output
	io.Copy(rw, bytes.NewReader(contents))

}
