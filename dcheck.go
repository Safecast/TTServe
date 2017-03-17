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
	GapsGt1week			uint32
	GapsGt1day			uint32
	GapsGt12hr			uint32
	GapsGt6hr			uint32
	GapsGt2hr			uint32
	GapsGt1hr			uint32
	GapsGt30m			uint32
	GapsGt15m			uint32
	GapsGt10m			uint32
	GapsGt5m			uint32
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
		if SecondsGap > 60 * 5 {
			ds.GapsGt5m++
		}
		if SecondsGap > 60 * 10 {
			ds.GapsGt10m++
		}
		if SecondsGap > 60 * 15 {
			ds.GapsGt15m++
		}
		if SecondsGap > 60 * 30 {
			ds.GapsGt30m++
		}
		if SecondsGap > 60 * 60 * 1 {
			ds.GapsGt1hr++
		}
		if SecondsGap > 60 * 60 * 2 {
			ds.GapsGt2hr++
		}
		if SecondsGap > 60 * 60 * 6 {
			ds.GapsGt6hr++
		}
		if SecondsGap > 60 * 60 * 12 {
			ds.GapsGt12hr++
		}
		if SecondsGap > 60 * 60 * 24 * 1 {
			ds.GapsGt1day++
		}
		if SecondsGap > 60 * 60 * 24 * 7 {
			ds.GapsGt1week++
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
	if ds.Measurements == 0 {
		return s
	}
	
    s += fmt.Sprintf("Oldest: %s\n", ds.OldestUpload.Format("2006-01-02 15:04 UTC"))
    s += fmt.Sprintf("Newest: %s\n", ds.NewestUpload.Format("2006-01-02 15:04 UTC"))
    s += fmt.Sprintf("Min gap: %d seconds\n", AgoMinutes(ds.MinUploadGapSecs/60))
    s += fmt.Sprintf("Max gap: %d seconds\n", AgoMinutes(ds.MaxUploadGapSecs))
    s += fmt.Sprintf("Gaps >1w: %d%% (%d)\n", ds.GapsGt1week/ds.Measurements, ds.GapsGt1week)
    s += fmt.Sprintf("Gaps >1d: %d%% (%d)\n", ds.GapsGt1day/ds.Measurements, ds.GapsGt1day)
    s += fmt.Sprintf("Gaps >12hr: %d%% (%d)\n", ds.GapsGt12hr/ds.Measurements, ds.GapsGt12hr)
    s += fmt.Sprintf("Gaps >6hr: %d%% (%d)\n", ds.GapsGt6hr/ds.Measurements, ds.GapsGt6hr)
    s += fmt.Sprintf("Gaps >2hr: %d%% (%d)\n", ds.GapsGt2hr/ds.Measurements, ds.GapsGt2hr)
    s += fmt.Sprintf("Gaps >1hr: %d%% (%d)\n", ds.GapsGt1hr/ds.Measurements, ds.GapsGt1hr)
    s += fmt.Sprintf("Gaps >30m: %d%% (%d)\n", ds.GapsGt30m/ds.Measurements, ds.GapsGt30m)
    s += fmt.Sprintf("Gaps >15m: %d%% (%d)\n", ds.GapsGt15m/ds.Measurements, ds.GapsGt15m)
    s += fmt.Sprintf("Gaps >10m: %d%% (%d)\n", ds.GapsGt10m/ds.Measurements, ds.GapsGt10m)
    s += fmt.Sprintf("Gaps >5m: %d%% (%d)\n", ds.GapsGt5m/ds.Measurements, ds.GapsGt5m)

    // Done
    return s
}
