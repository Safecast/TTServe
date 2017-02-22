// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/<gatewayid>" HTTP topic
package main

import (
    "os"
    "net/http"
    "fmt"
    "time"
    "io"
    "io/ioutil"
    "encoding/json"
)

// Handle inbound HTTP requests to fetch log files
func inboundWebGatewayHandler(rw http.ResponseWriter, req *http.Request) {

    // Set response mime type
    rw.Header().Set("Content-Type", "application/json")

    // Log it
    if req.RequestURI != TTServerGatewayPath {
        filename := req.RequestURI[len(TTServerTopicGateway2):]
        if filename != "" {

            fmt.Printf("%s Gateway information request for %s\n", time.Now().Format(logDateFormat), filename)

            // Open the file
            file := SafecastDirectory() + TTServerGatewayPath + "/" + filename + ".json"
            fd, err := os.Open(file)
            if err != nil {
                io.WriteString(rw, errorString(err))
                return
            }
            defer fd.Close()

            // Copy the file to output
            io.Copy(rw, fd)
            return

        }
    }

	// We have an update request
    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        fmt.Printf("GW: Error reading HTTP request body: \n%v\n", req)
        return
    }

	// Unmarshal it
    var ttg TTGateReq

    err = json.Unmarshal(body, &ttg)
    if err != nil {
        fmt.Printf("*** Received badly formatted Device Update request:\n%v\n", body)
        return
    }

    fmt.Printf("%s Received gateway update for %s\n", time.Now().Format(logDateFormat), ttg.GatewayId)
	go SafecastWriteGateway(ttg)

}