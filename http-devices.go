// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/log" HTTP topic
package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
)

// Handle inbound HTTP requests to fetch the entire list of devices
func inboundWebDevicesHandler(rw http.ResponseWriter, req *http.Request) {
	stats.Count.HTTP++

	// Loop over the file system, tracking all devices
	files, err := ioutil.ReadDir(SafecastDirectory() + TTDeviceLogPath)
	if err != nil {
		return
	}

	// Generate this array
	var allInfo []sheetInfo

	// Iterate over each of the values
	for _, file := range files {

		// Skip directories
		if file.IsDir() {
			continue
		}

		// Read the file
		contents, err := ioutil.ReadFile(SafecastDirectory() + TTDeviceLogPath + file.Name())
		if err != nil {
			continue
		}
		dstatus := DeviceStatus{}
		err = json.Unmarshal(contents, &dstatus)
		if err != nil {
			continue
		}

		// Generate results
		var si sheetInfo
		si.DeviceID = dstatus.DeviceID
		si.DeviceURN = dstatus.DeviceUID
		si.SN = dstatus.DeviceSN
		if dstatus.DeviceContact != nil {
			si.Custodian = dstatus.DeviceContact.Name
			si.CustodianContact = dstatus.DeviceContact.Email
		}
		if dstatus.CapturedAt != nil {
			si.LastSeen = *dstatus.CapturedAt
		}
		if dstatus.Loc != nil {
			if dstatus.Loc.Lat != nil {
				si.LastSeenLat = *dstatus.Loc.Lat
			}
			if dstatus.Loc.Lon != nil {
				si.LastSeenLon = *dstatus.Loc.Lon
			}
		}

		allInfo = append(allInfo, si)

	}

	// Marshal it
	allInfoJSON, _ := json.Marshal(allInfo)

	// Tell the caller that it's JSON
	rw.Header().Set("Content-Type", "application/json")

	// Output it
	io.WriteString(rw, string(allInfoJSON))

}
