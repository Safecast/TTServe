// Safecast v1-to-current data structure reformatting
package main

import (
    "fmt"
    "strings"
    "strconv"
)

// Reformat a special V1 payload to Current
func SafecastReformat(v1 *SafecastDataV1) (deviceid uint32, devtype string, data SafecastData) {
    var sd SafecastData
    var devicetype = ""

    // Required field
    if v1.DeviceID == nil {
        fmt.Printf("*** Reformat: Missing Device ID\n");
        return 0, "", sd
    }

    // Detect what range it is within, and process the conversion differently
    isPointcast := false
    if (*v1.DeviceID >= 100000 && *v1.DeviceID < 199999) {
        isPointcast = true
        devicetype = "pointcast"
        sd.DeviceID = uint64(*v1.DeviceID / 10)
    }
    isSafecastAir := false
    if (*v1.DeviceID >= 50000 && *v1.DeviceID < 59999) {
        isSafecastAir = true
        devicetype = "safecast-air"
        sd.DeviceID = uint64(*v1.DeviceID)
    }
    if !isPointcast && !isSafecastAir {
        fmt.Printf("*** Reformat: unsuccessful attempt to reformat Device ID %d\n", *v1.DeviceID);
        return 0, "", sd
    }

    // Captured
    if v1.CapturedAt != nil {
        sd.CapturedAt = v1.CapturedAt
    }

    // Loc
    if v1.Latitude != nil && v1.Longitude != nil {
        var loc Loc
        loc.Lat = *v1.Latitude
        loc.Lon = *v1.Longitude
        if v1.Height != nil {
            alt := float32(*v1.Height)
            loc.Alt = &alt
        }
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
                fmt.Printf("*** Reformat: Received CPM for non-Pointcast\n", sd.DeviceID)
            } else {
                if (*v1.DeviceID % 10) == 1 {
                    var lnd Lnd
                    cpm := *v1.Value
                    lnd.U7318 = &cpm
                    sd.Lnd = &lnd

                } else if (*v1.DeviceID % 10) == 2 {
                    var lnd Lnd
                    cpm := *v1.Value
                    lnd.EC7128 = &cpm
                    sd.Lnd = &lnd
                } else {
                    fmt.Printf("*** Reformat: %d cpm not understood for this subtype\n", sd.DeviceID);
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
                    if (unrecognized == "") {
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
            if (unrecognized != "") {
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
            fmt.Printf("*** Reformat Warning ***\n*** %s id=%d Unit %s = Value %f UNRECOGNIZED\n", devicetype, *v1.DeviceID, *v1.Unit, *v1.Value)

        }
    }

    return uint32(sd.DeviceID), devicetype, sd

}
