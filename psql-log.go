// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Log to PostreSQL, and process queries
package main

import (
    "fmt"
    "time"
)

// Database info
const dbTable = "data"

// LogToDb logs the specific safecast data point to the database
func LogToDb(sd SafecastData) bool {

	// Bail if the data table isn't provisioned
	exists, err := dbTableExists(dbTable)
	if (!exists) {
		fmt.Printf("db table '%s' not provisioned: %s\n", dbTable, err)
		return false
	}

	// Create a unique ID for the record
	randstr := fmt.Sprintf("%d", time.Now().UnixNano())
	randstr += fmt.Sprintf(".%d", Random(0, 1000000000))
	randstr += fmt.Sprintf(".%d", Random(0, 1000000000))
	randstr += fmt.Sprintf(".%d", Random(0, 1000000000))
	recordID := dbHashKey(randstr)

	// Add the object
	err = dbAddObject(dbTable, recordID, sd)
	if err != nil {
		fmt.Printf("db table error adding record to '%s': %s\n", dbTable, err)
		return false
	}

    // Done
    return true

}
