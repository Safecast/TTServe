// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Retrieve the manufacturing ID for a given device ID
package main

import (
	"fmt"
    "time"
	"strings"
    "net/http"
    "io/ioutil"
)

// Statics
var everRetrieved bool = false
var lastRetrieved time.Time
var failedRecently bool = false
var lastError string
var parsedData string

func SafecastDeviceIDToSN(DeviceId uint32) (uint32, string) {
    var fRetrieve bool = false
	var sheetData string = ""

    if parsedData == "" {
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
		parsedData = ""

		splitContents := strings.Split(string(sheetData), "\n")
		for _, c := range splitContents {
			splitLine := strings.Split(c, ",")
			if len(splitLine) < 2 {
				fmt.Printf("?: '%s'\n", c)
			} else {
				fmt.Printf("'%s' '%s'\n", c[0], c[1])
			}
		}

		// Cache the data for future iterations
        everRetrieved = true
        lastRetrieved = time.Now()
		failedRecently = false;

    }

if (false) {
	if parsedData == "" {
		lastError = "No data found"
		return 0, lastError
	}
}
	fmt.Printf("\n\n%s\n\n", sheetData)
	return 123, ""	
}
