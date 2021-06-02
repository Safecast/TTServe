// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// JSON file handling
package main

import (
	"encoding/json"
	"io"
	"os"

	ttdata "github.com/Safecast/safecast-go"
)

func jsonOpen(filename string) (*os.File, error) {

	// Open it
	fd, err := os.OpenFile(filename, os.O_WRONLY|os.O_APPEND, 0666)

	// Exit if no error
	if err == nil {
		return fd, nil
	}

	// Don't attempt to create it if it already exists
	_, err2 := os.Stat(filename)
	if err2 == nil {
		return nil, err
	}
	if !os.IsNotExist(err2) {
		return nil, err2
	}

	// Create the new dataset
	return jsonNew(filename)

}

// Create a new dataset
func jsonNew(filename string) (*os.File, error) {

	fd, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}

	// Write the header
	fd.WriteString("[\r\n")

	// Done
	return fd, nil

}

// Done
func jsonClose(fd *os.File) {

	// Write the header
	fd.WriteString("\r\n]\r\n")

	fd.Close()
}

// Append a measurement to the dataset
func jsonAppend(fd *os.File, sd *ttdata.SafecastData, first bool) {

	// Marshal it
	scJSON, _ := json.Marshal(sd)

	// Append a separator if not the first
	if !first {
		io.WriteString(fd, "\r\n,\r\n")
	}

	// Write it
	io.WriteString(fd, string(scJSON))

}
