// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/server/<instanceid>" HTTP topic
package main

import (
    "os"
    "net/http"
    "fmt"
    "io"
)

// Handle inbound HTTP requests to fetch log files
func inboundWebServerStatusHandler(rw http.ResponseWriter, req *http.Request) {
    stats.Count.HTTP++

    // Set response mime type
    rw.Header().Set("Content-Type", "application/json")

    // Log it
    if req.RequestURI != TTServerStatusPath && len(req.RequestURI) > len(TTServerTopicServerStatus) {
        filename := req.RequestURI[len(TTServerTopicServerStatus):]
        if filename != "" {

            fmt.Printf("%s Server information request for %s\n", LogTime(), filename)

            // Open the file
            file := SafecastDirectory() + TTServerStatusPath + "/" + filename + ".json"
            fd, err := os.Open(file)
            if err != nil {
                io.WriteString(rw, ErrorString(err))
                return
            }
            defer fd.Close()

            // Copy the file to output
            io.Copy(rw, fd)
            return

        }
    }

}
