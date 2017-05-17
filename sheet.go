// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Retrieve the manufacturing ID for a given device ID
package main

import (
    "fmt"
    "time"
    "strings"
    "strconv"
    "net/http"
    "io/ioutil"
)

type sheetRow struct {
    sn                  uint32
    deviceid            uint32
}
var sheet []sheetRow

// Statics
var everRetrieved bool
var lastRetrieved time.Time
var failedRecently bool
var lastError string

// DeviceIDToSN converts a Safecast device ID to its manufacturing serial number
func DeviceIDToSN(DeviceID uint32) (uint32, string) {
    var fRetrieve bool
    var sheetData string

	// Retrieve if never yet retrieved
    if sheet == nil {
        fRetrieve = true
    }

	// Cache for some time, for performance
    if everRetrieved && (time.Now().Sub(lastRetrieved) / time.Minute) > 15 {
        fRetrieve = true
        failedRecently = false
    }

	// If we've got an error, make sure we don't thrash every time we come in here
    if fRetrieve && failedRecently {
        return 0, lastError
    }

	// Fetch and parse the sheet
    if fRetrieve {
        rsp, err := http.Get(sheetsSolarcastTracker)
        if err != nil {
            lastError = fmt.Sprintf("%v", err)
            failedRecently = true;
            return 0, lastError
        }
        defer rsp.Body.Close()
        buf, err := ioutil.ReadAll(rsp.Body)
        if err != nil {
            lastError = fmt.Sprintf("%v", err)
            failedRecently = true;
            return 0, lastError
        }

        // Parse the sheet
        sheetData = string(buf)
        sheet = nil

        splitContents := strings.Split(string(sheetData), "\n")
        for _, c := range splitContents {
            splitLine := strings.Split(c, ",")
            if len(splitLine) >= 2 {
                u64, err := strconv.ParseUint(splitLine[0], 10, 32)
                if err == nil {
                    var row sheetRow
                    row.sn = uint32(u64)
                    row.deviceid = 0
                    u64, err := strconv.ParseUint(splitLine[1], 10, 32)
                    if err == nil {
                        row.deviceid = uint32(u64)
                    }
                    sheet = append(sheet, row)
                }
            }
        }

        // Cache the data for future iterations
        fmt.Printf("\n%s *** Refreshed %d entries from Google Sheets\n", LogTime(), len(splitContents))
        everRetrieved = true
        lastRetrieved = time.Now()
        failedRecently = false;

    }

    // Iterate over the rows to find the device
    deviceIDFound := false;
    snFound := uint32(0)
    for _, r := range sheet {
        if r.deviceid == DeviceID {
            deviceIDFound = true
            snFound = r.sn
            break
        }
    }

	// Done
    if !deviceIDFound {
        lastError = "Device ID not found"
        return 0, lastError
    }
    if snFound == 0 {
        lastError = "S/N not found for device"
        return 0, lastError
    }

    return snFound, ""

}
