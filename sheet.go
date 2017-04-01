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
var everRetrieved bool = false
var lastRetrieved time.Time
var failedRecently bool = false
var lastError string

func SafecastDeviceIDToSN(DeviceId uint32) (uint32, string) {
    var fRetrieve bool = false
    var sheetData string = ""

    if sheet == nil {
        fRetrieve = true
    }

    if everRetrieved && (time.Now().Sub(lastRetrieved) / time.Minute) > 5 {
        fRetrieve = true
        failedRecently = false
    }

    if fRetrieve && failedRecently {
        return 0, lastError
    }

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
        fmt.Printf("\n%s *** Refreshed %d entries from Google Sheets\n", logTime(), len(splitContents))
        everRetrieved = true
        lastRetrieved = time.Now()
        failedRecently = false;

    }

    // Iterate over the rows
    deviceIdFound := false;
    snFound := uint32(0)
    for _, r := range sheet {
        if r.deviceid == DeviceId {
            deviceIdFound = true
            snFound = r.sn
            break
        }
    }

    if !deviceIdFound {
        lastError = "Device ID not found"
        return 0, lastError
    }
    if snFound == 0 {
        lastError = "S/N not found for device"
        return 0, lastError
    }

    return snFound, ""

}
