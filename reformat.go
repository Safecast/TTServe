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

// Get the type of a device
func SafecastV1DeviceType(deviceid uint32) string {
	// For true V2 numbering space
    if deviceid >= 10000 && deviceid < 19999 {
		return "pointcast"
	}
    if deviceid >= 50000 && deviceid < 59999 {
        return "safecast-air"
	}
	// For V1 numbering space
    if deviceid >= 100000 && deviceid < 199999 {
		return "pointcast"
	}
	return ""	
}

// Reformat a special V1 payload to Current
func SafecastReformat(v1 *SafecastDataV1, isTestMeasurement bool) (deviceid uint32, devtype string, data SafecastData) {
    var sd SafecastData

    // Required field
    if v1.DeviceId == nil {
        fmt.Printf("*** Reformat: Missing Device ID\n")
        return 0, "", sd
    }

    // Detect what range it is within, and process the conversion differently
    devicetype := SafecastV1DeviceType(*v1.DeviceId)
    isPointcast := false
    if devicetype == "pointcast" {
        isPointcast = true
		did := uint64(*v1.DeviceId / 10)
        sd.DeviceId = &did
    }
    isSafecastAir := false
    if devicetype == "safecast-air" {
        isSafecastAir = true
		did := uint64(*v1.DeviceId)
        sd.DeviceId = &did
    }
    if !isPointcast && !isSafecastAir {
        fmt.Printf("*** Reformat: unsuccessful attempt to reformat Device ID %d\n", *v1.DeviceId)
        return 0, "", sd
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

        case "tempc":
            var env Env
            temp := *v1.Value
            env.Temp = &temp
            sd.Env = &env

        case "cpm":
            if !isPointcast {
                fmt.Printf("*** Reformat: Received CPM for non-Pointcast %d\n", *sd.DeviceId)
            } else {
                if 1 == (*v1.DeviceId % 10) {
                    var lnd Lnd
                    cpm := *v1.Value
                    lnd.U7318 = &cpm
                    sd.Lnd = &lnd

                } else if 2 == (*v1.DeviceId % 10) {
                    var lnd Lnd
                    cpm := *v1.Value
                    lnd.EC7128 = &cpm
                    sd.Lnd = &lnd
                } else {
                    fmt.Printf("*** Reformat: %d cpm not understood for this subtype\n", *sd.DeviceId)
                }
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
                    bat.Voltage = &f32
                    dobat = true
                case "Fails":
                    u64, _ := strconv.ParseUint(field[1], 10, 32)
                    u32 := uint32(u64)
                    dev.CommsFails = &u32
                    dodev = true
                case "Restarts":
                    u64, _ := strconv.ParseUint(field[1], 10, 32)
                    u32 := uint32(u64)
                    dev.DeviceRestarts = &u32
                    dodev = true
                case "FreeRam":
                    u64, _ := strconv.ParseUint(field[1], 10, 32)
                    u32 := uint32(u64)
                    dev.FreeMem = &u32
                    dodev = true
                case "NTP count":
                    u64, _ := strconv.ParseUint(field[1], 10, 32)
                    u32 := uint32(u64)
                    dev.NTPCount = &u32
                    dodev = true
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
            fmt.Printf("*** Reformat Warning ***\n*** %s id=%d Unit %s = Value %f UNRECOGNIZED\n", devicetype, *v1.DeviceId, *v1.Unit, *v1.Value)

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
