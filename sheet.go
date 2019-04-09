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
    custodian           string
    location            string
}
var sheet []sheetRow

// Statics
var everRetrieved bool
var lastRetrieved time.Time
var failedRecently bool
var lastError string

// DeviceIDToSN converts a Safecast device ID to its manufacturing serial number
func DeviceIDToSN(DeviceID uint32) (sn uint32, info string) {
    var fRetrieve bool
    var sheetData string

    // Retrieve if never yet retrieved
    if !everRetrieved {
        fRetrieve = true
    }

    // Cache for some time, for performance
    if everRetrieved && (time.Now().Sub(lastRetrieved) / time.Minute) > 15 {
        fRetrieve = true
        failedRecently = false
    }

    // If we've got an error, make sure we don't thrash every time we come in here
    if fRetrieve && failedRecently {
        return 0, ""
    }

    // Fetch and parse the sheet
    if fRetrieve {
        rsp, err := http.Get(sheetsSolarcastTracker)
        if err != nil {
            lastError = fmt.Sprintf("%s", err)
            failedRecently = true;
			fmt.Printf("***** CANNOT http.Get %s\n", sheetsSolarcastTracker)
			fmt.Printf("***** %s\n", lastError)
            return 0, ""
        }
        defer rsp.Body.Close()
        buf, err := ioutil.ReadAll(rsp.Body)
        if err != nil {
            lastError = fmt.Sprintf("%s", err)
            failedRecently = true;
			fmt.Printf("***** CANNOT ioutil.ReadAll %s\n", sheetsSolarcastTracker)
			fmt.Printf("***** %s\n", lastError)
            return 0, ""
        }

        // Parse the sheet.  If the col numbers change, this must be changed
        sheetData = string(buf)
        sheet = nil
        splitContents := strings.Split(string(sheetData), "\n")
        for _, c := range splitContents {
            var row sheetRow
            splitLine := strings.Split(c, ",")
            for col, val := range splitLine {
                switch col {
                case 0: // A
                    u64, err := strconv.ParseUint(val, 10, 32)
                    if err == nil {
                        row.sn = uint32(u64)
                    }
                case 1: // B
                    u64, err := strconv.ParseUint(val, 10, 32)
                    if err == nil {
                        row.deviceid = uint32(u64)
                    }
                case 5: // F
                    row.custodian = val
                case 6: // G
                    row.location = val

                }
            }
            if row.sn != 0 && row.deviceid != 0 {
                sheet = append(sheet, row)
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

            // Craft an info string from the sheetRow
            if (r.custodian == "" && r.location != "") {
                info = fmt.Sprintf("%s", r.location)
            } else if (r.custodian != "" && r.location == "") {
                info = fmt.Sprintf("%s", r.custodian)
            } else {
                info = fmt.Sprintf("%s, %s", r.custodian, r.location)
            }

            break
        }
    }

    // Done
    if !deviceIDFound {
        return 0, ""
    }
    if snFound == 0 {
        return 0, ""
    }

    return snFound, info
}
