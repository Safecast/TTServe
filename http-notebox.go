// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the routing from a notebox
package main

import (
    "io/ioutil"
    "net/http"
    "fmt"
)

// Handle inbound HTTP requests from Notebox's via the Notehub reporter task
func inboundWebNoteboxHandler(rw http.ResponseWriter, req *http.Request) {
    var body []byte
    var err error

    // Remember when it was uploaded to us
    UploadedAt := NowInUTC()

    // Get the remote address, and only add this to the count if it's likely from
    // the internal HTTP load balancer.
    remoteAddr, isReal := getRequestorIPv4(req)
    if !isReal {
        remoteAddr = "internal address"
    }

    // Read the body as a byte array
    body, err = ioutil.ReadAll(req.Body)
    if err != nil {
        stats.Count.HTTP++
        fmt.Printf("Error reading HTTP request body: \n%v\n", req)
        return

    }

	// Display it
	fmt.Printf("*** %s received this from %s\n%s\n***\n", UploadedAt, remoteAddr, string(body))

    // A real request
    stats.Count.HTTP++

}
