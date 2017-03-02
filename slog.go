// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Server log support
package main

import (
    "os"
    "fmt"
    "time"
    "hash/crc32"
    "strings"
)

// Get the current log file name for the current instance
func ServerLogFilename(extension string) string {
    prefix := time.Now().UTC().Format("2006-01-")
    filename := prefix + TTServeInstanceID + extension
    return filename
}

// A secret that only allows the URLs from the Slack command to function
func ServerLogSecret() string {
    timestr := ControlFileTime(TTServerSlackCommandControlFile,"").Format(logDateFormat)
    checksum := crc32.ChecksumIEEE([]byte(timestr))
    checkstr := fmt.Sprintf("%d", checksum)
    return checkstr
}

// Log a string to the instance's log file
func ServerLog(sWithoutDate string) {

    // Add a standard header unless it begins with a newline
    s := sWithoutDate
    if !strings.HasPrefix(sWithoutDate, "\n") {
        s = fmt.Sprintf("%s %s", time.Now().Format(logDateFormat), sWithoutDate)
    }

    // Print it to the console
    fmt.Printf("%s", s)

    // Open it
    file := SafecastDirectory() + TTServerLogPath + "/" + ServerLogFilename(".log")
    fd, err := os.OpenFile(file, os.O_WRONLY|os.O_APPEND, 0666)
    if (err != nil) {

        // Don't attempt to create it if it already exists
        _, err2 := os.Stat(file)
        if err2 == nil {
            fmt.Printf("ServerLogging: Can't log to %s: %s\n", file, err);
            return
        }
        if err2 == nil {
            if !os.IsNotExist(err2) {
                fmt.Printf("ServerLogging: Ignoring attempt to create %s: %s\n", file, err2);
                return
            }
        }

        // Attempt to create the file because it doesn't already exist
        fd, err = os.OpenFile(file, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
        if (err != nil) {
            fmt.Printf("ServerLogging: error creating file %s: %s\n", file, err);
            return;
        }
    }

    // Append it
    fd.WriteString(s);

    // Close and exit
    fd.Close();

}
