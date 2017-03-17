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
    Valid               bool
    Uploaded            time.Time
    LoraTransport       bool
    FonaTransport       bool
    TestMeasurement     bool
}

// Stats about all measurements
type MeasurementDataset struct {
    DeviceId            uint32
    LogRange            string
    OldestUpload        time.Time
    NewestUpload        time.Time
    MinUploadGapSecs    uint32
    MaxUploadGapSecs    uint32
    Measurements        uint32
    TestMeasurements    bool
    LoraTransports      uint32
    FonaTransports      uint32
}

// Check an individual measurement
func CheckMeasurement(sd SafecastData) MeasurementStat {
    stat := MeasurementStat{}

    // Ignore old-format data that didn't have service_uploaded
    if sd.Service == nil || sd.Service.UploadedAt == nil {
        return stat
    }
    stat.Valid = true

    // Process service-related stats
    if sd.Service != nil {

        stat.Uploaded, _ = time.Parse("2006-01-02T15:04:05Z", *sd.Service.UploadedAt)

    }

    // Done
    return stat

}

func NewMeasurementDataset(deviceidstr string, logRange string) MeasurementDataset {
    ds := MeasurementDataset{}

    ds.LogRange = logRange
    u64, _ := strconv.ParseUint(deviceidstr, 10, 32)
    ds.DeviceId = uint32(u64)

    return ds

}

// Check an individual measurement
func AggregateMeasurementIntoDataset(ds *MeasurementDataset, stat MeasurementStat) {

    // Only record valid stats
    if !stat.Valid {
        return
    }
    ds.Measurements++

    // If the stat is out of date order, ignore it
    if ds.Measurements == 1 {
        ds.OldestUpload = stat.Uploaded
        ds.NewestUpload = stat.Uploaded
    }
    if stat.Uploaded.Sub(ds.NewestUpload) < 0 {
        ds.Measurements--
        fmt.Printf("** Out-of-order %d stat %v < %v\n", ds.DeviceId, stat.Uploaded, ds.NewestUpload)
        return
    }
    SecondsGap := uint32(stat.Uploaded.Sub(ds.NewestUpload) / time.Second)
    if SecondsGap != 0 {
        if ds.MinUploadGapSecs == 0 || SecondsGap < ds.MinUploadGapSecs {
            ds.MinUploadGapSecs = SecondsGap
        }
        if SecondsGap > ds.MaxUploadGapSecs {
            ds.MaxUploadGapSecs = SecondsGap
        }
    }
    ds.NewestUpload = stat.Uploaded

    // Done

}

// Check an individual measurement
func GenerateDatasetSummary(ds MeasurementDataset) string {
    s := ""

    // High-level stats
    s += fmt.Sprintf("** Health Check for Device %d\n", ds.DeviceId)
    s += fmt.Sprintf("** %s UTC\n", time.Now().Format(logDateFormat))
    s += fmt.Sprintf("\n")

    s += fmt.Sprintf("Period: %s\n", ds.LogRange)
    s += fmt.Sprintf("Measurements: %d\n", ds.Measurements)

    s += fmt.Sprintf("Oldest: %s\n", ds.OldestUpload.Format("2006-01-02 15:04 UTC"))
    s += fmt.Sprintf("Newest: %s\n", ds.NewestUpload.Format("2006-01-02 15:04 UTC"))
    s += fmt.Sprintf("Min gap: %d seconds\n", ds.MinUploadGapSecs)
    s += fmt.Sprintf("Max gap: %d seconds\n", ds.MaxUploadGapSecs)

    // Done
    return s
}
