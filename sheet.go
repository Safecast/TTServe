// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Retrieve the manufacturing ID for a given device ID
package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type sheetInfo struct {
	DeviceID         uint32 `json:"device,omitempty"`
	SN               string `json:"sn,omitempty"`
	Custodian        string `json:"custodian_name,omitempty"`
	CustodianContact string `json:"custodian_contact,omitempty"`
	Location         string `json:"location,omitempty"`
	Dashboard        string `json:"dashboard,omitempty"`
}

var sheet []sheetInfo

// Statics
var fRetrieve bool
var lastRetrieved time.Time

// sheetInvalidateCache forces a reload
func sheetInvalidateCache() {
	fRetrieve = true
}

// sheetDeviceIDToSN converts a Safecast device ID to its manufacturing serial number
func sheetDeviceIDToSN(DeviceID uint32) (sn string, infoStr string) {
	info, err := sheetDeviceInfo(DeviceID, "")
	if err != nil {
		return "", ""
	}
	sn = info.SN
	if info.Custodian == "" && info.Location != "" {
		infoStr = info.Location
	} else if info.Custodian != "" && info.Location == "" {
		infoStr = info.Custodian
	} else {
		infoStr = fmt.Sprintf("%s, %s", info.Custodian, info.Location)
	}
	return
}

// sheetDeviceInfo retrieves sheetInfo for a given device
func sheetDeviceInfo(DeviceID uint32, DeviceSN string) (info sheetInfo, err error) {

	// Cache for some time, for performance
	if (time.Since(lastRetrieved) / time.Minute) > 15 {
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
		colDashboard := -1

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
		for row := 0; ; row++ {
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
			for col := 0; col < len(record); col++ {
				val := record[col]
				// Header row with field names
				if row == 0 {
					switch val {
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
					case "Dashboard":
						colDashboard = col
					}
				} else {
					if colSerialNumber == -1 {
						err = fmt.Errorf("no 'Serial Number' column")
						return
					}
					if colDeviceID == -1 {
						err = fmt.Errorf("no 'Device ID' column")
						return
					}
					if colCustodian == -1 {
						err = fmt.Errorf("no 'Custodian' column")
						return
					}
					if colCustodianContact == -1 {
						err = fmt.Errorf("no 'Custodian Contact' column")
						return
					}
					if colLocation == -1 {
						err = fmt.Errorf("no 'Location' column")
						return
					}
					if col == colSerialNumber {
						rec.SN = val
					} else if col == colDeviceID {
						u64, err2 := strconv.ParseUint(val, 10, 32)
						if err2 == nil {
							rec.DeviceID = uint32(u64)
						}
					} else if col == colCustodian {
						rec.Custodian = val
					} else if col == colDashboard {
						rec.Dashboard = val
					} else if col == colCustodianContact {
						rec.CustodianContact = val
					} else if col == colLocation {
						rec.Location = val
					}
				}
			}

			if rec.DeviceID != 0 || rec.SN != "" {
				sheet = append(sheet, rec)
				sheetRowsRecognized++
			}

		}

		// Summary
		fmt.Printf("\n%s *** Parsed Device Tracker CSV: recognized %d rows of %d total\n\n", LogTime(), sheetRowsRecognized, sheetRowsTotal)

	}

	// Iterate over the rows to find the device
	deviceIDFound := false
	for _, r := range sheet {
		if r.DeviceID == DeviceID || (DeviceSN != "" && r.SN == DeviceSN) {
			deviceIDFound = true
			info = r
			break
		}
	}

	// Device not found
	if !deviceIDFound {

		// It was agreed with Rob t(see ttnode/src/io.c) that we would reserve the low 2^20 addresses
		// for fixed allocation.  If we didn't find the device ID here and if it was in that range,
		// use THAT as the serial number.
		if DeviceID < 1048576 {
			info.DeviceID = DeviceID
		} else {
			err = fmt.Errorf("not found in Tracker Sheet")
			return
		}

	}

	// return the info
	return
}
