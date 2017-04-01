// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Influx-related
package main

import (
    "fmt"
    "time"
    influx "github.com/influxdata/influxdb/client/v2"
)

const (
    SafecastDb = "safecast"
    SafecastDataPoint = "data"
)

func SafecastLogToInflux(sd SafecastData) bool {
    var clcfg influx.HTTPConfig

    // Initialize the client
    clcfg.Addr = fmt.Sprintf("https://%s:8086", ServiceConfig.InfluxHost)
    clcfg.Username = ServiceConfig.InfluxUsername
    clcfg.Password = ServiceConfig.InfluxPassword

    // Open the client
    cl, clerr := influx.NewHTTPClient(clcfg)
    if clerr == nil {
        defer cl.Close()
    } else {
        fmt.Printf("Influx connect error: %v\n", clerr)
        return false
    }

    // Create a new batch
    bpcfg := influx.BatchPointsConfig{}
    bpcfg.Database = SafecastDb
    bp, bperr := influx.NewBatchPoints(bpcfg)
    if bperr != nil {
        fmt.Printf("Influx batch points creation error: %v\n", bperr)
        return false
    }

	// Create the tags and fields structures from which a point will be made
	tags := map[string]string{}
	fields := map[string]interface{}{}

	// Extract each safecast field into its influx equivalent
	tags["device_str"] = fmt.Sprintf("%d", *sd.DeviceId)
    if sd.CapturedAt != nil {
        t, e := time.Parse("2006-01-02T15:04:05Z", *sd.CapturedAt)
        if e == nil {
            i64 := t.UnixNano()
            fields["when_captured_num"] = &i64
        }
    }
	if sd.Loc != nil {
		if sd.Loc.MotionBegan != nil {
			t, e := time.Parse("2006-01-02T15:04:05Z", *sd.Loc.MotionBegan)
			if e == nil {
				i64 := t.UnixNano()
				fields["loc_when_motion_began_num"] = &i64
	        }
	    }
		if sd.Loc.Olc != nil {
			tags["loc_olc"] = *sd.Loc.Olc
		}
	}
	if sd.Service != nil {
	    if sd.Service.UploadedAt != nil {
	        t, e := time.Parse("2006-01-02T15:04:05Z", *sd.Service.UploadedAt)
	        if e == nil {
	            i64 := t.UnixNano()
	            fields["service_uploaded_num"] = &i64
			}
		}
		if sd.Service.Handler != nil {
			tags["service_handler"] = *sd.Service.Handler
		}
		if sd.Service.Transport != nil {
			tags["service_transport"] = *sd.Service.Transport
		}
	}
	if sd.Dev != nil {
		if sd.Dev.DeviceLabel != nil {
			tags["dev_label"] = *sd.Dev.DeviceLabel
		}
		if sd.Dev.AppVersion != nil {
			tags["dev_firmware"] = *sd.Dev.AppVersion
		}
		if sd.Dev.ModuleLora != nil {
			tags["dev_module_lora"] = *sd.Dev.ModuleLora
		}
		if sd.Dev.ModuleFona != nil {
			tags["dev_module_fona"] = *sd.Dev.ModuleFona
		}
	}
	if sd.Gateway != nil {
		if sd.Gateway.ReceivedAt != nil {
	        t, e := time.Parse("2006-01-02T15:04:05Z", *sd.Gateway.ReceivedAt)
	        if e == nil {
	            i64 := t.UnixNano()
	            fields["gateway_received_num"] = &i64
	        }
		}
    }

    // Make a new point
    pt, mperr := influx.NewPoint(SafecastDataPoint, tags, fields)
    if mperr != nil {
        fmt.Printf("Influx point creation error: %v\n", mperr)
        return false
    }

    // Debug
    if (true) {
        fmt.Printf("*** Influx:\n%v\n", pt)
    }

    // Add the point to the batch
    bp.AddPoint(pt)

    // Write the batch
    wrerr := cl.Write(bp)
    if wrerr != nil {
        fmt.Printf("Influx write error: %v\n", wrerr)
        return false
    }

    // Done
    return true

}
