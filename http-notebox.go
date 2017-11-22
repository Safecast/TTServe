// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the routing from a notebox
package main

import (
    "io/ioutil"
    "net/http"
    "fmt"
    "encoding/json"
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

	// Parse it into an array of SafecastData structures
	set := []SafecastData{}
	err = json.Unmarshal(body, &set)
	if err != nil {
		fmt.Printf("*** %s cannot parse received this from %s: %s\n%s\n***\n", UploadedAt, remoteAddr, err, string(body))
		return
	}

	// Display it
	fmt.Printf("\n\n*** %s received this from %s\n%v\n%s\n***\n", UploadedAt, remoteAddr, set, body)

    // A real request
    stats.Count.HTTP++

}
