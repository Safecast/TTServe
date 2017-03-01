// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/server-log/<instanceid>" HTTP topic
package main

import (
	"os"
    "net/http"
    "fmt"
	"time"
	"strings"
    "io"
)

// Handle inbound HTTP requests to fetch log files
func inboundWebServerLogHandler(rw http.ResponseWriter, req *http.Request) {
    stats.CountHTTP++

    // Set response mime type
    rw.Header().Set("Content-Type", "text/plain")

    // Log it
    fn := req.RequestURI[len(TTServerTopicServerLog):]
    fmt.Printf("%s instance information request for %s\n", time.Now().Format(logDateFormat), fn)

	// Crack the secret
    Str := strings.Split(fn, "$")
	if len(Str) != 2 {
	    fmt.Printf("%s Badly formatted instance request\n", time.Now().Format(logDateFormat))
		return
	}		
    secret := Str[0]
    filename := Str[1]
	if secret != ServerLogSecret() {
	    fmt.Printf("%s Ssecret %s != %s\n", time.Now().Format(logDateFormat), secret, ServerLogSecret())
		return
	}		

    // Open the file
    file := SafecastDirectory() + TTServerLogPath + "/" + filename
    fd, err := os.Open(file)
    if err != nil {
        io.WriteString(rw, errorString(err))
        return
    }
    defer fd.Close()

    // Copy the file to output
    io.Copy(rw, fd)

}
