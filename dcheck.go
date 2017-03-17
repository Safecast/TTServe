// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Support for device performance verification
package main

import (
    "fmt"
	"strconv"
    "time"
)

// GOALS:
//          - does a summary of total errors encountered
//          - makes sure it got at least some data from each kind of sensor
//          - makes sure it heard from both lora and fona
//          - does some simple range check on each data value

// Stats about a single measurement
type MeasurementStat struct {
    Uploaded            time.Time
    LoraTransport       bool
    FonaTransport       bool
    TestMeasurement     bool
}

// Stats about all measurements
type MeasurementDataset struct {
    DeviceId            uint32
    FirstUpload         time.Time
    LastUpload          time.Time
    MinUploadGapMSecs   uint32
    MaxUploadGapSecs    uint32
    Measurements        uint32
    TestMeasurements    bool
    LoraTransports      uint32
    FonaTransports      uint32
}

// Check an individual measurement
func CheckMeasurement(sd SafecastData) MeasurementStat {
    stat := MeasurementStat{}

    // Done
    return stat

}

func NewMeasurementDataset(deviceidstr string) MeasurementDataset {
    ds := MeasurementDataset{}

    // Parse the expected device ID
    u64, _ := strconv.ParseUint(deviceidstr, 10, 32)
    ds.DeviceId = uint32(u64)

    return ds
}

// Check an individual measurement
func AggregateMeasurementIntoDataset(ds *MeasurementDataset, stat MeasurementStat) {

    ds.Measurements++

    // Done

}

// Check an individual measurement
func GenerateDatasetSummary(ds MeasurementDataset) string {
    s := ""

    // High-level stats
    s += fmt.Sprintf("** Health Check for Device %d\n** %s\n\n", ds.DeviceId, time.Now().Format(logDateFormat))
	s += fmt.Sprintf("Total Measurements: %d\n", ds.Measurements)
	
    // Done
	return s
}
