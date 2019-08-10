// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Retrieve the manufacturing ID for a given device ID
package main

import (
    "fmt"
    "time"
    "io"
    "strconv"
    "net/http"
	"encoding/csv"
)

type sheetRow struct {
    SerialNumber        uint32
    DeviceID            uint32
    Custodian           string
    Location            string
}
var sheet []sheetRow

// Statics
var lastRetrieved time.Time

// DeviceIDToSN converts a Safecast device ID to its manufacturing serial number
func DeviceIDToSN(DeviceID uint32) (sn uint32, info string) {
    var fRetrieve bool

    // Cache for some time, for performance
    if (time.Now().Sub(lastRetrieved) / time.Minute) > 15 {
        fRetrieve = true
    }

    // Fetch and parse the sheet
    if fRetrieve {

		// Set retrieved date regardless of error, so we don't thrash trying to reload
        lastRetrieved = time.Now()

		// Preset for parsing
        sheet = nil
		colSerialNumber := -1
		colDeviceID := -1
		colCustodian := -1
		colLocation := -1

		// Reload
        rsp, err := http.Get(sheetsSolarcastTracker)
        if err != nil {
            fmt.Printf("***** CANNOT http.Get %s: %s\n", sheetsSolarcastTracker, err)
        } else {
            defer rsp.Body.Close()
			r := csv.NewReader(rsp.Body)
			sheetRowsTotal := 0
			sheetRowsRecognized := 0
			for row:=0;;row++ {
				record, err := r.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					fmt.Printf("***** error reading CSV: %s\n", err)
					break
				}
				sheetRowsTotal++
                rec := sheetRow{}
				for col:=0; col<len(record); col++ {
					val := record[col]
					// Skip first header row
					if row == 0 {
						continue
					}
					// Header row with field names
					if row == 1 {
						switch (val) {
						case "Serial Number":
							colSerialNumber = col
						case "Device ID":
							colDeviceID = col
						case "Custodian":
							colCustodian = col
						case "Location":
							colLocation = col
						}
					} else {
						if (colSerialNumber == -1) {
							return 1, "no 'Serial Number' column"
						}
						if (colDeviceID == -1) {
							return 1, "no 'Device ID' column"
						}
						if (colCustodian == -1) {
							return 1, "no 'Custodian' column"
						}
						if (colLocation == -1) {
							return 1, "no 'Location' column"
						}
						if col == colSerialNumber {
		                    u64, err := strconv.ParseUint(val, 10, 32)
			                if err == nil {
				                rec.SerialNumber = uint32(u64)
					        }
						} else if col == colDeviceID {
			                u64, err := strconv.ParseUint(val, 10, 32)
		                    if err == nil {
				                rec.DeviceID = uint32(u64)
					        }
						} else if col == colCustodian {
		                    rec.Custodian = val
						} else if col == colLocation {
				            rec.Location = val
					    }
					}
				}
                if rec.DeviceID != 0 {
                    sheet = append(sheet, rec)
					sheetRowsRecognized++
                }
			}

            // Summary
            fmt.Printf("\n%s *** Parsed Device Tracker CSV: recognized %d rows of %d total\n\n", LogTime(), sheetRowsRecognized, sheetRowsTotal)

        }


    }

    // Iterate over the rows to find the device
    deviceIDFound := false;
    snFound := uint32(0)
    for _, r := range sheet {
        if r.DeviceID == DeviceID {

            deviceIDFound = true
            snFound = r.SerialNumber

            // Craft an info string from the sheetRow
            if (r.Custodian == "" && r.Location != "") {
                info = fmt.Sprintf("%s", r.Location)
            } else if (r.Custodian != "" && r.Location == "") {
                info = fmt.Sprintf("%s", r.Custodian)
            } else {
                info = fmt.Sprintf("%s, %s", r.Custodian, r.Location)
            }

            break
        }
    }

    // Not found
    if !deviceIDFound {

        // It was agreed with Rob t(see ttnode/src/io.c) that we would reserve the low 2^20 addresses
        // for fixed allocation.  If we didn't find the device ID here and if it was in that range,
        // use THAT as the serial number.
        if (DeviceID < 1048576) {
            return DeviceID, ""
        }

        // A new style device that was not found
        fmt.Printf("*** Please enter info for device %d in the Tracker spreadsheet\n", DeviceID)
        return 0, ""
    }

    return snFound, info
}
