// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Safecast v1-to-current data structure reformatting
package main

import (
    "fmt"
	"time"
    "strings"
    "strconv"
    "github.com/google/open-location-code/go"
)

// Determine if this is a Solarcast Nano
func SafecastDeviceIsSolarcastNano(deviceid uint32) bool {
	sn, _ := sheetDeviceIDToSN(deviceid)
	if sn >= 33000 && sn < 34000 {
		return true
	}
	return false
}

// SafecastDeviceType returns the type of a Safecast device AS NUMBERED in our
// V2 address space
func SafecastDeviceType(deviceid uint32) (programmatic string, display string) {

    // Pointcast
    if deviceid >= 10000 && deviceid <= 29999 {
        return "pointcast", "Pointcast"
    }

    // Exception for pointcast device 100
    if deviceid == 100 {
        return "pointcast", "Pointcast"
    }

    // Air
    if deviceid >= 50000 && deviceid <= 59999 {
        return "safecast-air", "Safecast Air"
    }

    // GeigieCast
    if deviceid >= 60000 && deviceid <= 69999 {
        return "geigiecast", "bGeigiecast"
    }

    // nGeigie
    if deviceid > 0 && deviceid <= 999 {
        return "ngeigie", "nGeigie"
    }

	// Unknown device type - must be a solarcast
	return "", "Solarcast"
	
}

// SafecastV1DeviceType returns the type of a device AS NATIVELY NUMBERED
// by pointcast, safecast-air, or ngeigie devices
func SafecastV1DeviceType(deviceid uint32) (devicetype string, devicename string, v2DeviceID uint32) {

    // For standard V1 pointcast numbering space
    if deviceid >= 100000 && deviceid <= 299999 {
        return "pointcast", "Pointcast", deviceid/10
    }

    // Exception for pointcast device 100x
    if deviceid >= 1000 && deviceid <= 1999 {
        return "pointcast", "Pointcast", deviceid/10
    }

    // Air
    if deviceid >= 50000 && deviceid <= 59999 {
        return "safecast-air", "Safecast Air", deviceid
    }

    // GeigieCast
    if deviceid >= 60000 && deviceid <= 69999 {
        return "geigiecast", "bGeigiecast", deviceid
    }

    // nGeigie
    if deviceid > 0 && deviceid <= 999 {
        return "ngeigie", "nGeigie", deviceid
    }
	
    return "", "Solarcast", deviceid

}

// SafecastReformatFromV1 reformats a special V1 payload to Current
func SafecastReformatFromV1(v1 *SafecastDataV1, isTestMeasurement bool) (deviceid uint32, devtype string, data SafecastData) {
    var sd SafecastData
	var v1DeviceID uint32

    // Required field
    if v1.DeviceID == nil {
        fmt.Printf("*** Reformat: Missing Device ID\n")
        return 0, "", sd
    }

	// Fetch the V1 device ID and place it into a var so we can manipulate it
	v1DeviceID = *v1.DeviceID

	// Catch attempts to use DeviceID == 0 by placing it into somewhere we can watch.
	// We're putting this here 2017-04-12 because we're observing that nGeigie Device #40
	// is occasionally sending:
	// {"longitude":"140.9917","latitude":"37.5635","device_id":"0","value":"0",
	//  "unit":"cpm","height":"5","devicetype_id":"Pointcast V1"}
	if v1DeviceID == 0 {
        return 0, "", sd
	}

	// Special-case a single nGeigie that had been partially converted to Pointcast firmware,
	// a bug fix we put in on 2017-04-13 at Rob's guidance.
	if v1DeviceID == 48 {
		v1DeviceID = 40
	}
	
    // Detect what range it is within, and process the conversion differently,
	// rejecting non-reformattable devices
    devicetype, devicename, v2DeviceID := SafecastV1DeviceType(v1DeviceID)
    if devicetype == "" {
        fmt.Printf("*** Reformat: unsuccessful attempt to reformat Device ID %d\n", v1DeviceID)
        return 0, "", sd
    }

	// THIS is where we determine sensor types based on device ID
	tubeType := "unknown"
	if v2DeviceID == 100 || v2DeviceID == 63 || v2DeviceID == 54 {
		tubeType = "U712"
	} else if v2DeviceID == 78 || v2DeviceID == 20105 {
		tubeType = "W78017"
	} else if devicetype == "ngeigie" {
		tubeType = "U7318"
	} else if devicetype == "geigiecast" {
		tubeType = "U7318"
	} else if devicetype == "pointcast" && v1DeviceID % 10 == 1 {
		tubeType = "U7318"
	} else if devicetype == "pointcast" && v1DeviceID % 10 == 2 {
		tubeType = "EC7128"
	}

	// Device ID
    sd.DeviceID = &v2DeviceID
	urn := fmt.Sprintf("%s:%d", devicetype, v2DeviceID)
	sd.DeviceUID = &urn

	// Device Serial Number
	sn, _ := sheetDeviceIDToSN(v2DeviceID)
	if sn != 0 {
		snstr := fmt.Sprintf("%s #%d", devicename, sn)
		sd.DeviceSN = &snstr
	}

    // Captured
    if v1.CapturedAt != nil {

		// Correct for badly formatted safecast-air data of the form 2017-9-7T2:3:4Z
		t, err := time.Parse("2006-1-2T15:4:5Z", *v1.CapturedAt)
		if err != nil {
	        sd.CapturedAt = v1.CapturedAt
		} else {
			s := t.UTC().Format("2006-01-02T15:04:05Z")
			sd.CapturedAt = &s
		}

    }

    // Loc
    if v1.Latitude != nil && v1.Longitude != nil {
        var loc Loc
        loc.Lat = v1.Latitude
        loc.Lon = v1.Longitude
        if v1.Height != nil {
            alt := float32(*v1.Height)
            loc.Alt = &alt
        }
        // 11 digits is 3m accuracy
        Olc := olc.Encode(float64(*loc.Lat), float64(*loc.Lon), 11)
        loc.Olc = &Olc
        sd.Loc = &loc
    }

    // Reverse-engineer Unit/Value to yield the good stuff
    if v1.Unit != nil && v1.Value != nil {

        switch (strings.ToLower(*v1.Unit)) {

        case "pm1":
            var opc Opc
            pm := *v1.Value
            opc.Pm01_0 = &pm
            sd.Opc = &opc

        case "pm2.5":
            var opc Opc
            pm := *v1.Value
            opc.Pm02_5 = &pm
            sd.Opc = &opc

        case "pm10":
            var opc Opc
            pm := *v1.Value
            opc.Pm10_0 = &pm
            sd.Opc = &opc

        case "humd%":
            var env Env
            humid := *v1.Value
            env.Humid = &humid
            sd.Env = &env

        case "celcius":
            fallthrough
        case "tempc":
            var env Env
            temp := *v1.Value
            env.Temp = &temp
            sd.Env = &env

        case "cpm":
            var lnd Lnd
            cpm := *v1.Value
			// Special case for missing tube on this sensor
			if v1DeviceID == 1001 && cpm == 0 {
		        return 0, "", sd
			}
			switch tubeType {
			case "U7318":
                lnd.U7318 = &cpm
	            sd.Lnd = &lnd
			case "U712":
                lnd.U712 = &cpm
	            sd.Lnd = &lnd
			case "EC7128":
                lnd.EC7128 = &cpm
	            sd.Lnd = &lnd
			case "W78017":
                lnd.W78017 = &cpm
	            sd.Lnd = &lnd
			default:
                fmt.Printf("*** Reformat: Received CPM for unrecognized device %d\n", *sd.DeviceID)
            }

        case "status":
            // The value is the temp
            var env Env
            TempC := *v1.Value
            env.Temp = &TempC
            sd.Env = &env

            // Parse subfields
            var bat Bat
            var dobat = false
            var dev Dev
            var dodev = false

            unrecognized := ""
            status := ""
            if v1.DeviceTypeID != nil {
                status = *v1.DeviceTypeID
            }
            fields := strings.Split(status, ",")
            for v := range fields {
                field := strings.Split(fields[v], ":")
                switch (field[0]) {
                case "Battery Voltage":
                    f64, _ := strconv.ParseFloat(field[1], 32)
                    f32 := float32(f64)
                    if f32 != 0 {
                        bat.Voltage = &f32
                        dobat = true
                    }
                case "Fails":
                    u64, _ := strconv.ParseUint(field[1], 10, 32)
                    u32 := uint32(u64)
                    if u32 != 0 {
                        dev.CommsFails = &u32
                        dodev = true
                    }
                case "Restarts":
                    u64, _ := strconv.ParseUint(field[1], 10, 32)
                    u32 := uint32(u64)
                    if u32 != 0 {
                        dev.DeviceRestarts = &u32
                        dodev = true
                    }
                case "FreeRam":
                    u64, _ := strconv.ParseUint(field[1], 10, 32)
                    u32 := uint32(u64)
                    if u32 != 0 {
                        dev.FreeMem = &u32
                        dodev = true
                    }
                case "NTP count":
                    u64, _ := strconv.ParseUint(field[1], 10, 32)
                    u32 := uint32(u64)
                    if u32 != 0 {
                        dev.NTPCount = &u32
                        dodev = true
                    }
                case "Last failure":
                    var LastFailure = field[1]
                    dev.LastFailure = &LastFailure
                    dodev = true
                default:
                    if unrecognized == "" {
                        unrecognized = "{"
                    } else {
                        unrecognized = unrecognized + ","
                    }
                    unrecognized = unrecognized + "\"" + field[0] + "\":\"" + field[1] + "\""
                case "DeviceID":
                case "Temperature":
                }
            }

            // If we found unrecognized fields, emit them
            if unrecognized != "" {
                unrecognized = unrecognized + "}"
                dev.Status = &unrecognized
                dodev = true
            }

            // Include in  the uploads
            if dobat {
                sd.Bat = &bat
            }
            if dodev {
                sd.Dev = &dev
            }

        default:
            fmt.Printf("*** Reformat Warning ***\n*** %s id=%d Unit %s = Value %f UNRECOGNIZED\n", devicetype, v1DeviceID, *v1.Unit, *v1.Value)

        }
    }

    // Test
    if isTestMeasurement {
        if sd.Dev == nil {
            var dev Dev
            sd.Dev = &dev
        }
        sd.Dev.Test = &isTestMeasurement
    }

    return uint32(*sd.DeviceID), devicetype, sd

}

// SafecastReformatToV1 reformats a special V1 payload to Current
func SafecastReformatToV1(sd SafecastData) (v1Data1 *SafecastDataV1ToEmit, v1Data2 *SafecastDataV1ToEmit, v1Data9 *SafecastDataV1ToEmit, err error) {

	// Start under the assumption that we will generate all three
	sd1 := &SafecastDataV1ToEmit{}
	sd2 := &SafecastDataV1ToEmit{}
	sd9 := &SafecastDataV1ToEmit{}
	
	// Process the fields common to all outputs
	if sd.CapturedAt != nil {
		sd1.CapturedAt = sd.CapturedAt
	}
	if sd.Loc != nil {
		if sd.Loc.Lat != nil {
			value := fmt.Sprintf("%f", *sd.Loc.Lat)
			sd1.Latitude = &value
		}
		if sd.Loc.Lon != nil {
			value := fmt.Sprintf("%f", *sd.Loc.Lon)
			sd1.Longitude = &value
		}
		if sd.Loc.Alt != nil {
			value := fmt.Sprintf("%d", int(*sd.Loc.Alt))
			sd1.Height = &value
		}
	}
	*sd2 = *sd1
	*sd9 = *sd1
	
	// Generate the device IDs.  If this isn't a solarcast device, don't generate anything.
	if sd.DeviceID == nil {
		return
	}
	deviceType, _ := SafecastDeviceType(*sd.DeviceID)
	if deviceType != "" {
		return
	}

	// Solarcast.  Return S/N as V1 device ID
	id, _ := sheetDeviceIDToSN(*sd.DeviceID)
	if id == 0 {
		return
	}
	id1 := fmt.Sprintf("%d", id*10 + 1)
	sd1.DeviceID = &id1
	id2 := fmt.Sprintf("%d", id*10 + 2)
	sd2.DeviceID = &id2
	id9 := fmt.Sprintf("%d", id*10 + 9)
	sd9.DeviceID = &id9

	// Generate the first geiger tube value
	if (sd.Lnd != nil) {
		if (sd.U7318 != nil) {
			value := fmt.Sprintf("%d", int(*sd.Lnd.U7318))
			sd1.Value = &value
			unit := "cpm"
			sd1.Unit = &unit
			deviceType := "lnd_7318u"
			sd1.DeviceTypeID = &deviceType
		} else if (sd.U712 != nil) {
			value := fmt.Sprintf("%d", int(*sd.Lnd.U712))
			sd1.Value = &value
			unit := "cpm"
			sd1.Unit = &unit
			deviceType := "lnd_712u"
			sd1.DeviceTypeID = &deviceType
		} else if (sd.W78017 != nil) {
			value := fmt.Sprintf("%d", int(*sd.Lnd.W78017))
			sd1.Value = &value
			unit := "cpm"
			sd1.Unit = &unit
			deviceType := "lnd_78017w"
			sd1.DeviceTypeID = &deviceType
		} else {
			sd1 = nil
		}
	} else {
		sd1 = nil
	}

	// Generate the second geiger tube value
	if (sd.Lnd != nil) {
		if (sd.C7318 != nil) {
			value := fmt.Sprintf("%d", int(*sd.Lnd.C7318))
			sd2.Value = &value
			unit := "cpm"
			sd2.Unit = &unit
			deviceType := "lnd_7318c"
			sd2.DeviceTypeID = &deviceType
		} else if (sd.EC7128 != nil) {
			value := fmt.Sprintf("%d", int(*sd.Lnd.EC7128))
			sd2.Value = &value
			unit := "cpm"
			sd2.Unit = &unit
			deviceType := "lnd_7128ec"
			sd2.DeviceTypeID = &deviceType
		} else {
			sd2 = nil
		}
	} else {
		sd2 = nil
	}

	// Generate the temp value
	if (sd.Env != nil && sd.Env.Temp != nil) {
		value := fmt.Sprintf("%.1f", *sd.Env.Temp)
		sd9.Value = &value
		unit := "celcius"
		sd9.Unit = &unit
		info := fmt.Sprintf("DeviceID:%d,Temperature:%.1f", id, *sd.Env.Temp)
		if (sd.Bat != nil && sd.Bat.Voltage != nil) {
			info += fmt.Sprintf(",Battery Voltage:%1f", *sd.Bat.Voltage)
		}
		sd9.DeviceTypeID = &info
	} else {
		sd9 = nil
	}

	// Done
	v1Data1 = sd1
	v1Data2 = sd2
	v1Data9 = sd9
	return
}
