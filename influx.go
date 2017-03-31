// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Influx-related
package main

import (
    "fmt"
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
	bpcfg.Precision = "s"
	bp, bperr := influx.NewBatchPoints(bpcfg)
	if bperr != nil {
		fmt.Printf("Influx batch points creation error: %v\n", bperr)
		return false
	}

    // Marshal the safecast data to json text
    sdJSON, _ := json.Marshal(sd)
	var fields map[string]interface{}
	jmerr := json.Unmarshal(sdJSON, &fields)
	if jmerr != nil {
		fmt.Printf("JSON unmarshaling error: %v\n", jmerr)
		return false
	}

	// Make a new point
	pt, mperr := influx.NewPoint(SafecastDataPoint, nil, fields)
	if mperr != nil {
		fmt.Printf("Influx point creation error: %v\n", mperr)
		return false
	}

	fmt.Printf("Influx point:\n%v\n", pt)
	
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
