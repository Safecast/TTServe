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
func logQuery(qstr string, isCSV bool, user string) (numResults int, url string, err error) {

    // Bail if the data table isn't provisioned
    err = dbValidateTable(dbTable, true)
    if err != nil {
        return
    }

    // Unmarshal the text into a JSON query
    q := DbQuery{}
    err = json.Unmarshal([]byte(qstr), &q)
    if err != nil {
        err = fmt.Errorf("query format not recognized: %s: %s\n%v\n", qstr, err, []byte(qstr))
		return
    }

	// If format not specified, take the default from method param
    if (q.Format == "") {
        if (isCSV) {
            q.Format = "csv"
        } else {
            q.Format = "json"
        }
    }

	// If no columns specified, allow it in JSON (which dumps the whole thing), but not in CSV
	if (q.Columns == "") {
		if (q.Format == "json") {
			q.Columns = ".value"
		} else {
	        err = fmt.Errorf("columns to return must be specified using \"columns\" field")
			return
		}
	}

    // Build a PSQL query
    sqlQuery, qerr := dbBuildQuery(dbTable, &q)
    if qerr != nil {
		err = fmt.Errorf("cannot build query: %s\n", qerr)
        return 
    }

    // Generate the filename
    file := time.Now().UTC().Format("2006-01-02T15:04:05Z") + "-" + user
    if isCSV {
        file = file + "." + q.Format
    } else {
        file = file + "." + q.Format
    }
    url = fmt.Sprintf("http://%s%s%s", TTServerHTTPAddress, TTServerTopicQueryResults, file)
    filename := SafecastDirectory() + TTQueryPath + "/"  + file

    // Create the output file
    var fd *os.File
    if isCSV {
        fd, err = csvNew(filename)
    } else {
        fd, err = jsonNew(filename)
    }
    if err != nil {
        err = fmt.Errorf("cannot create file %s: %s", file, err)
		return
    }

    var response string
    numResults, _, response, err = dbQueryToWriter(fd, sqlQuery, false, &q)
    if err != nil {
        fmt.Printf("QueryWriter error: %s\n", err)
    }
    fmt.Printf("QueryWriter response: %s file %s\n", response, file)

    // Done, after writing the footer
    if isCSV {
        csvClose(fd)
    } else {
        jsonClose(fd)
    }

    return 
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
