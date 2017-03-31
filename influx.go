// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Influx-related
package main

import (
    "fmt"
	"time"
    "encoding/json"
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

	// Add "idb" values of our date data structures, for influx queries
	s64 := fmt.Sprintf("%d", *sd.DeviceId)
	sd.DeviceIdIdb = &s64
	if sd.CapturedAt != nil {
		t, e := time.Parse("2006-01-02T15:04:05Z", *sd.CapturedAt)
		if e == nil {
			i64 := t.UnixNano()
			s64 := fmt.Sprintf("%19d", i64)
			sd.CapturedAtIdb = &s64
		}
	}
	if sd.Service != nil && sd.Service.UploadedAt != nil {
		t, e := time.Parse("2006-01-02T15:04:05Z", *sd.Service.UploadedAt)
		if e == nil {
			i64 := t.UnixNano()
			s64 := fmt.Sprintf("%19d", i64)
			sd.Service.UploadedAtIdb = &s64
		}
	}
	if sd.Gateway != nil && sd.Gateway.ReceivedAt != nil {
		t, e := time.Parse("2006-01-02T15:04:05Z", *sd.Gateway.ReceivedAt)
		if e == nil {
			i64 := t.UnixNano()
			s64 := fmt.Sprintf("%19d", i64)
			sd.Gateway.ReceivedAtIdb = &s64
		}
	}
	if sd.Loc != nil && sd.Loc.MotionBegan != nil {
		t, e := time.Parse("2006-01-02T15:04:05Z", *sd.Loc.MotionBegan)
		if e == nil {
			i64 := t.UnixNano()
			s64 := fmt.Sprintf("%19d", i64)
			sd.Loc.MotionBeganIdb = &s64
		}
	}

	// Split the safecast data into "tags" and "fields", where
	// Tags must be strings and are indexed, so queries are very fast
	// Fields have arbitrary values that are not indexed, so queries are slower
	sdFields := sd
	sdTags := SafecastData{}
	if sdFields.Service != nil && sdFields.Service.UploadedAtIdb != nil {
		if sdTags.Service == nil {
			var svc Service
			sdTags.Service = &svc
		}
		sdTags.Service.UploadedAtIdb = sdFields.Service.UploadedAtIdb
		sdFields.Service.UploadedAtIdb = nil
	}
	if sdFields.Service != nil && sdFields.Service.Handler != nil {
		if sdTags.Service == nil {
			var svc Service
			sdTags.Service = &svc
		}
		sdTags.Service.Handler = sdFields.Service.Handler
		sdFields.Service.Handler = nil
	}
	if sdFields.Service != nil && sdFields.Service.Transport != nil {
		if sdTags.Service == nil {
			var svc Service
			sdTags.Service = &svc
		}
		sdTags.Service.Transport = sdFields.Service.Transport
		sdFields.Service.Transport = nil
	}
	if sdFields.Gateway != nil && sdFields.Gateway.ReceivedAtIdb != nil {
		if sdTags.Gateway == nil {
			var gw Gateway
			sdTags.Gateway = &gw
		}
		sdTags.Gateway.ReceivedAtIdb = sdFields.Gateway.ReceivedAtIdb
		sdFields.Gateway.ReceivedAtIdb = nil
	}
	if sdFields.Loc != nil && sdFields.Loc.MotionBeganIdb != nil {
		if sdTags.Loc == nil {
			var loc Loc
			sdTags.Loc = &loc
		}
		sdTags.Loc.MotionBeganIdb = sdFields.Loc.MotionBeganIdb
		sdFields.Loc.MotionBeganIdb = nil
	}
	if sdFields.Loc != nil && sdFields.Loc.Olc != nil {
		if sdTags.Loc == nil {
			var loc Loc
			sdTags.Loc = &loc
		}
		sdTags.Loc.Olc = sdFields.Loc.Olc
		sdFields.Loc.Olc = nil
	}
	if sdFields.Dev != nil && sdFields.Dev.DeviceLabel != nil {
		if sdTags.Dev == nil {
			var dev Dev
			sdTags.Dev = &dev
		}
		sdTags.Dev.DeviceLabel = sdFields.Dev.DeviceLabel
		sdFields.Dev.DeviceLabel = nil
	}
	if sdFields.Dev != nil && sdFields.Dev.AppVersion != nil {
		if sdTags.Dev == nil {
			var dev Dev
			sdTags.Dev = &dev
		}
		sdTags.Dev.AppVersion = sdFields.Dev.AppVersion
		sdFields.Dev.AppVersion = nil
	}
	if sdFields.Dev != nil && sdFields.Dev.ModuleLora != nil {
		if sdTags.Dev == nil {
			var dev Dev
			sdTags.Dev = &dev
		}
		sdTags.Dev.ModuleLora = sdFields.Dev.ModuleLora
		sdFields.Dev.ModuleLora = nil
	}
	if sdFields.Dev != nil && sdFields.Dev.ModuleFona != nil {
		if sdTags.Dev == nil {
			var dev Dev
			sdTags.Dev = &dev
		}
		sdTags.Dev.ModuleFona = sdFields.Dev.ModuleFona
		sdFields.Dev.ModuleFona = nil
	}
	if sdFields.DeviceIdIdb != nil {
		sdTags.DeviceIdIdb = sdFields.DeviceIdIdb
		sdFields.DeviceIdIdb = nil
	}
	if sdFields.CapturedAtIdb != nil {
		sdTags.CapturedAtIdb = sdFields.CapturedAtIdb
		sdFields.CapturedAtIdb = nil
	}
	
    // Marshal the safecast data to json text
    sdTagsJson, _ := json.Marshal(sdTags)
	var tags map[string]string
	jterr := json.Unmarshal(sdTagsJson, &tags)
	if jterr != nil {
		fmt.Printf("JSON tags unmarshaling error: %v\n", jterr)
		return false
	}
    sdFieldsJson, _ := json.Marshal(sdFields)
	var fields map[string]interface{}
	jferr := json.Unmarshal(sdFieldsJson, &fields)
	if jferr != nil {
		fmt.Printf("JSON fields unmarshaling error: %v\n", jferr)
		return false
	}

	// Make a new point
	pt, mperr := influx.NewPoint(SafecastDataPoint, tags, fields)
	if mperr != nil {
		fmt.Printf("Influx point creation error: %v\n", mperr)
		return false
	}

	// Debug
	if (false) {
		fmt.Printf("***   Tags:\n%s\n", string(sdTagsJson));
		fmt.Printf("*** Fields:\n%s\n", string(sdFieldsJson));
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
