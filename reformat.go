// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Safecast v1-to-current data structure reformatting
package main

import (
    "fmt"
    "strings"
    "strconv"
    "github.com/google/open-location-code/go"
)

// Get the type of a device AS NUMBERED in our
// V2 address space
func SafecastV1V2DeviceType(deviceid uint32) string {

    // nGeigie
    if deviceid > 0 && deviceid <= 999 {
        return "ngeigie"
    }

    // Pointcast
    if deviceid >= 1000 && deviceid <= 19999 {
        return "pointcast"
    }

    // Air
    if deviceid >= 50000 && deviceid <= 59999 {
        return "safecast-air"
    }

	return ""
	
}

// Get the type of a device AS NATIVELY NUMBERED
// by pointcast, safecast-air, or ngeigie devices
func SafecastV1DeviceType(deviceid uint32) string {

    // nGeigie
    if deviceid > 0 && deviceid <= 999 {
        return "ngeigie"
    }

    // For standard V1 pointcast numbering space
    if deviceid >= 100000 && deviceid < 299999 {
        return "pointcast"
    }

    // Exception for pointcast device 1002
    if deviceid >= 10000 && deviceid < 29999 {
        return "pointcast"
    }

    // Air
    if deviceid >= 50000 && deviceid < 59999 {
        return "safecast-air"
    }

    return ""
}

// Reformat a special V1 payload to Current
func SafecastReformat(v1 *SafecastDataV1, isTestMeasurement bool) (deviceid uint32, devtype string, data SafecastData) {
    var sd SafecastData
	var v1DeviceId uint32

    // Required field
    if v1.DeviceId == nil {
        fmt.Printf("*** Reformat: Missing Device ID\n")
        return 0, "", sd
    }

	// Fetch the V1 device ID and place it into a var so we can manipulate it
	v1DeviceId = *v1.DeviceId

	// Catch attempts to use DeviceID == 0 by placing it into somewhere we can watch.
	// We're putting this here 2017-04-12 because we're observing that nGeigie Device #40
	// is occasionally sending:
	// {"longitude":"140.9917","latitude":"37.5635","device_id":"0","value":"0",
	//  "unit":"cpm","height":"5","devicetype_id":"Pointcast V1"}
	if v1DeviceId == 0 {
        return 0, "", sd
	}

	// Special-case a single nGeigie that had been partially converted to Pointcast firmware,
	// a bug fix we put in on 2017-04-13 at Rob's guidance.
	if v1DeviceId == 48 {
		v1DeviceId = 40
	}
	
    // Detect what range it is within, and process the conversion differently
    devicetype := SafecastV1DeviceType(v1DeviceId)
	v2DeviceId := uint32(0)
    isPointcast := false
    if devicetype == "pointcast" {
        isPointcast = true
        v2DeviceId = uint32(v1DeviceId / 10)
        sd.DeviceId = &v2DeviceId
    }
    isSafecastAir := false
    if devicetype == "safecast-air" {
        isSafecastAir = true
        v2DeviceId = uint32(v1DeviceId)
        sd.DeviceId = &v2DeviceId
    }
    isNgeigie := false
    if devicetype == "ngeigie" {
        isNgeigie = true
        v2DeviceId = uint32(v1DeviceId)
        sd.DeviceId = &v2DeviceId
    }
	
	// Reject non-reformattable devices
    if !isPointcast && !isSafecastAir && !isNgeigie {
        fmt.Printf("*** Reformat: unsuccessful attempt to reformat Device ID %d\n", v1DeviceId)
        return 0, "", sd
    }

	// THIS is where we determine sensor types based on device ID
	tubeType := "unknown"
	if v2DeviceId == 1002 || v2DeviceId == 63 || v2DeviceId == 54 {
		tubeType = "U712"
	} else if v2DeviceId == 78 {
		tubeType = "W78017"
	} else if isNgeigie {
		tubeType = "U7318"
	} else if isPointcast && v1DeviceId % 10 == 1 {
		tubeType = "U7318"
	} else if isPointcast && v1DeviceId % 10 == 2 {
		tubeType = "EC7128"
	}

    // Captured
    if v1.CapturedAt != nil {
        sd.CapturedAt = v1.CapturedAt
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
                fmt.Printf("*** Reformat: Received CPM for unrecognized device %d\n", *sd.DeviceId)
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
            if v1.DeviceTypeId != nil {
                status = *v1.DeviceTypeId
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
                    var LastFailure string = field[1]
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
            fmt.Printf("*** Reformat Warning ***\n*** %s id=%d Unit %s = Value %f UNRECOGNIZED\n", devicetype, v1DeviceId, *v1.Unit, *v1.Value)

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

    return uint32(*sd.DeviceId), devicetype, sd

}
