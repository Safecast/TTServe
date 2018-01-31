// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Log to PostreSQL, and process queries
package main

import (
	"os"
    "fmt"
	"time"
    "encoding/json"
)

// Database info
const dbTable = "data"

// Query the log to a file
func logQuery(qstr string, isCSV bool, user string) error {

	q := DbQuery{}
	errParse := json.Unmarshal([]byte(qstr), &q)
	if errParse != nil {
		return fmt.Errorf("query format not recognized: %s: %s\n%v\n", qstr, errParse, []byte(qstr))
	}

	sqlQuery, err := dbBuildQuery(dbTable, &q)
	if err != nil {
		return fmt.Errorf("cannot build query: %s\n", err)
	}

	// Generate the filename
    file := time.Now().UTC().Format("2006-01-02") + "-" + user
	if isCSV {
		file = file + ".csv"
	} else {
		file = file + ".json"
	}
    filename := SafecastDirectory() + TTQueryPath + "/"  + file

    // Create the output file
	var fd *os.File
	if isCSV {
		fd, err = csvNew(filename)
	} else {
		fd, err = jsonNew(filename)
	}
    if err != nil {
        return fmt.Errorf("cannot create file %s: %s", file, err)
    }
    defer fd.Close()
	
	var response string
	_, response, err = dbQueryToWriter(fd, sqlQuery, false, &q)
	if err != nil {
		fmt.Printf("QueryWriter error: %s\n", err)
	}
	fmt.Printf("QueryWriter response: %s\n", response)

	return nil
}

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
