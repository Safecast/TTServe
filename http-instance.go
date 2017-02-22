// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/<instanceid>" HTTP topic
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
func inboundWebInstanceHandler(rw http.ResponseWriter, req *http.Request) {

    // Set response mime type
    rw.Header().Set("Content-Type", "text/plain")

    // Log it
    filename := req.RequestURI[len(TTServerTopicInstance):]
    fmt.Printf("%s Device information request for %s\n", time.Now().Format(logDateFormat), filename)

    // Open the file
    file := SafecastDirectory() + TTServerInstancePath + "/" + filename
    fd, err := os.Open(file)
    if err != nil {
        io.WriteString(rw, errorString(err))
        return
    }
    defer fd.Close()

    // Copy the file to output
    io.Copy(rw, fd)

}

// Get the current log file name for the current instance
func InstanceLogFilename(extension string) string {
    directory := SafecastDirectory()
    prefix := time.Now().UTC().Format("2006-01-")
    file := directory + TTServerInstancePath + "/" + prefix + TTServeInstanceID + extension
    return file
}

// Log a string to the instance's log file
func ILog(sWithoutDate string) {

	// Add a standard header unless it begins with a newline
	s := sWithoutDate
	if !strings.HasPrefix(sWithoutDate, "\n") {
		s = fmt.Sprintf("%s %s", time.Now().Format(logDateFormat), sWithoutDate)
	}
	
	// Print it to the console
	fmt.Printf("%s", s)
	
    // Open it
	file := InstanceLogFilename(".log")
    fd, err := os.OpenFile(file, os.O_WRONLY|os.O_APPEND, 0666)
    if (err != nil) {

	// Don't attempt to create it if it already exists
	    _, err2 := os.Stat(file)
		if err2 == nil {
            fmt.Printf("ILogging: Can't log to %s: %s\n", file, err);
			return
	    }
        if err2 == nil {
			if !os.IsNotExist(err2) {
	            fmt.Printf("ILogging: Ignoring attempt to create %s: %s\n", file, err2);
				return
			}
	    }

        // Attempt to create the file because it doesn't already exist
        fd, err = os.OpenFile(file, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
        if (err != nil) {
            fmt.Printf("ILogging: error creating file %s: %s\n", file, err);
            return;
        }
	}

	// Append it
    fd.WriteString(s);

    // Close and exit
    fd.Close();
	
}