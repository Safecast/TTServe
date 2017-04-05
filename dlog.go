// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Log file handling, whether JSON or CSV
package main

import (
    "os"
    "fmt"
    "time"
    "encoding/json"
)

// Construct path of a log file
func SafecastDeviceLogFilename(DeviceId string, Extension string) string {
    directory := SafecastDirectory()
    prefix := time.Now().UTC().Format("2006-01-")
    file := directory + TTDeviceLogPath + "/" + prefix + DeviceId + Extension
    return file
}

// Write to logs.
// Note that we don't do this with a goroutine because the serialization is helpful
// in log-ordering for buffered I/O messages where there are a huge batch of readings
// that are updated in sequence very quickly.
func SafecastWriteToLogs(sd SafecastData) {
    go SafecastLogToInflux(sd)
    go SafecastWriteDeviceStatus(sd)
    go SafecastJSONDeviceLog(sd)
    go SafecastCSVDeviceLog(sd)
}

// Write the value to the log
func SafecastJSONDeviceLog(sd SafecastData) {

    file := SafecastDeviceLogFilename(fmt.Sprintf("%d", *sd.DeviceId), ".json")

    // Open it
    fd, err := os.OpenFile(file, os.O_WRONLY|os.O_APPEND, 0666)
    if err != nil {

        // Don't attempt to create it if it already exists
        _, err2 := os.Stat(file)
        if err2 == nil {
            fmt.Printf("Logging: Can't log to %s: %s\n", file, err)
            return
        }
        if err2 == nil {
            if !os.IsNotExist(err2) {
                fmt.Printf("Logging: Ignoring attempt to create %s: %s\n", file, err2)
                return
            }
        }

        // Attempt to create the file because it doesn't already exist
        fd, err = os.OpenFile(file, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
        if err != nil {
            fmt.Printf("Logging: error creating %s: %s\n", file, err)
            return
        }

    }

    // Turn stats into a safe string writing
    if sd.Service == nil {
        var svc Service
        sd.Service = &svc
    }
    scJSON, _ := json.Marshal(sd)
    fd.WriteString(string(scJSON))
    fd.WriteString("\r\n,\r\n")

    // Close and exit
    fd.Close()

}

// Write the value to the log
func SafecastCSVDeviceLog(sd SafecastData) {

	// Open the file for append
    filename := SafecastDeviceLogFilename(fmt.Sprintf("%d", *sd.DeviceId), ".csv")
	fd, err := csvOpen(filename)
    if err != nil {
        fmt.Printf("Logging: Can't open %s: %s\n", filename, err)
        return
    }

	// Append this measurement
	csvAppend(fd, &sd)

	// Done
	csvClose(fd)

}
