// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Retrieve the manufacturing ID for a given device ID
package main

import (
	"fmt"
	"time"
	"io"
	"strings"
	"strconv"
	"net/http"
	"encoding/csv"
)

type sheetInfo struct {
	DeviceID			uint32			`json:"device,omitempty"`
	SerialNumber		string			`json:"sn,omitempty"`
	Custodian			string			`json:"custodian_name,omitempty"`
	CustodianContact	string			`json:"custodian_contact,omitempty"`
	Location			string			`json:"location,omitempty"`
	// Info that is not contained in spreadsheet, but which is needed externally
	LastSeen			string			`json:"last_seen,omitempty"`
}
var sheet []sheetInfo

// Statics
var fRetrieve bool
var lastRetrieved time.Time

// sheetInvalidateCache forces a reload
func sheetInvalidateCache() {
	fRetrieve = true
}

// normalize a serial number to remove characters not valid in a filename
func normalizeSN(sn string) (result string) {
	sn = strings.ToLower(sn)
	for _, char := range sn {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			result += fmt.Sprintf("%c", char)
			continue
		}
		if (char == ' ') {
			result += "_"
			continue
		}
	}
	return
}

// sheetDeviceIDToSN converts a Safecast device ID to its manufacturing serial number
func sheetDeviceIDToSN(DeviceID uint32) (sn string, infoStr string) {
	info, err := sheetDeviceInfo(DeviceID, "")
	if err != nil {
		return "", fmt.Sprintf("%s", err)
	}
	sn = info.SerialNumber
	if (info.Custodian == "" && info.Location != "") {
		infoStr = fmt.Sprintf("%s", info.Location)
	} else if (info.Custodian != "" && info.Location == "") {
		infoStr = fmt.Sprintf("%s", info.Custodian)
	} else {
		infoStr = fmt.Sprintf("%s, %s", info.Custodian, info.Location)
	}
	return
}

// sheetDeviceInfo retrieves sheetInfo for a given device
func sheetDeviceInfo(DeviceID uint32, normalizedSN string) (info sheetInfo, err error) {

	// Cache for some time, for performance
	if (time.Now().Sub(lastRetrieved) / time.Minute) > 15 {
		fRetrieve = true
	}

	// Fetch and parse the sheet
	if fRetrieve {

		// Set retrieved date regardless of error, so we don't thrash trying to reload
		fRetrieve = false
		lastRetrieved = time.Now()

		// Preset for parsing
		sheet = nil
		colSerialNumber := -1
		colDeviceID := -1
		colCustodian := -1
		colCustodianContact := -1
		colLocation := -1

		// Reload
		rsp, err2 := http.Get(sheetsSolarcastTracker)
		if err2 != nil {
			err = fmt.Errorf("sheet: open: %s", err2)
			return
		}
		defer rsp.Body.Close()
		r := csv.NewReader(rsp.Body)
		sheetRowsTotal := 0
		sheetRowsRecognized := 0
		for row:=0;;row++ {
			record, err2 := r.Read()
			if err2 == io.EOF {
				break
			}
			if err2 != nil {
				err = fmt.Errorf("sheet: read: %s", err2)
				return
			}
			sheetRowsTotal++
			rec := sheetInfo{}
			for col:=0; col<len(record); col++ {
				val := record[col]
				// Header row with field names
				if row == 0 {
					switch (val) {
					case "Serial Number":
						colSerialNumber = col
					case "Device ID":
						colDeviceID = col
					case "Custodian":
						colCustodian = col
					case "Custodian Contact":
						colCustodianContact = col
					case "Location":
						colLocation = col
					}
				} else {
					if (colSerialNumber == -1) {
						err = fmt.Errorf("no 'Serial Number' column")
						return
					}
					if (colDeviceID == -1) {
						err = fmt.Errorf("no 'Device ID' column")
						return
					}
					if (colCustodian == -1) {
						err = fmt.Errorf("no 'Custodian' column")
						return
					}
					if (colCustodianContact == -1) {
						err = fmt.Errorf("no 'Custodian Contact' column")
						return
					}
					if (colLocation == -1) {
						err = fmt.Errorf("no 'Location' column")
						return
					}
					if col == colSerialNumber {
						rec.SerialNumber = normalizeSN(val)
					} else if col == colDeviceID {
						u64, err2 := strconv.ParseUint(val, 10, 32)
						if err2 == nil {
							rec.DeviceID = uint32(u64)
						}
					} else if col == colCustodian {
						rec.Custodian = val
					} else if col == colCustodianContact {
						rec.CustodianContact = val
					} else if col == colLocation {
						rec.Location = val
					}
				}
			}

			if rec.DeviceID != 0 || rec.SerialNumber != "" {
				sheet = append(sheet, rec)
				sheetRowsRecognized++
				fmt.Printf("OZZIE: %v\n", rec);
			}

		}

		// Summary
		fmt.Printf("\n%s *** Parsed Device Tracker CSV: recognized %d rows of %d total\n\n", LogTime(), sheetRowsRecognized, sheetRowsTotal)


	}

	// Iterate over the rows to find the device
	deviceIDFound := false;
	for _, r := range sheet {
		if r.DeviceID == DeviceID || r.SerialNumber == normalizedSN {
			deviceIDFound = true
			info = r
			break
		}
	}
	fmt.Printf("OZZIE: looking for %d '%s' found? %T\n", DeviceID, normalizedSN, deviceIDFound);

	// Device not found
	if !deviceIDFound {

		// It was agreed with Rob t(see ttnode/src/io.c) that we would reserve the low 2^20 addresses
		// for fixed allocation.  If we didn't find the device ID here and if it was in that range,
		// use THAT as the serial number.
		if (DeviceID < 1048576) {
			info.DeviceID = DeviceID
		} else {
			err = fmt.Errorf("not found in Tracker Sheet")
			return
		}

	}

	// return the info
	return
}
