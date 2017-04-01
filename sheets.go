// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Retrieve the manufacturing ID for a given device ID
package main

import (
	"fmt"
    "time"
    "net/http"
    "io/ioutil"
)

// Statics
var everRetrieved bool = false
var lastRetrieved time.Time
var failedRecently bool = false
var lastError string
var sheetData string

func SafecastDeviceIDToSN(DeviceId uint32) (uint32, string) {
    var fRetrieve bool = false

    if sheetData == "" {
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

        // Cache the sheet for future iterations
        sheetData = string(buf)
        everRetrieved = true
        lastRetrieved = time.Now()
		failedRecently = false;

    }

	if sheetData == "" {
		lastError = "No data found"
		return 0, lastError
	}

	fmt.Printf("\n\n%s\n\n", sheetData)
	return 123, ""	
}
