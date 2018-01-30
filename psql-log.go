// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Log to PostreSQL, and process queries
package main

import (
    "fmt"
)

// Database info
const dbTable = "data"

// LogToDb logs the specific safecast data point to the database
func LogToDb(sd SafecastData) bool {

	// Bail if the data table isn't provisioned
	err := dbValidateTable(dbTable, true)
	if err != nil {
		fmt.Printf("error opening table '%s': %s\n", dbTable, err)
		return false
	}

	// Add the object with a unique ID
	err = dbAddObject(dbTable, "", sd)
	if err != nil {
		fmt.Printf("db table error adding record to '%s': %s\n", dbTable, err)
		return false
	}

    // Done
    return true

}
