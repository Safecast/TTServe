// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Support for device performance verification
package main

import (
    "fmt"
    "strconv"
	"strings"
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
	Transport			string
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
	Transports			string
    LoraTransports      uint32
    FonaTransports      uint32
}

func NewMeasurementDataset(deviceidstr string, logRange string) MeasurementDataset {
    ds := MeasurementDataset{}

    ds.LogRange = logRange
    u64, _ := strconv.ParseUint(deviceidstr, 10, 32)
    ds.DeviceId = uint32(u64)

    return ds

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

		if sd.Service.Transport != nil {
	        str := strings.Split(*sd.Service.Transport, ":")
			scheme := ""
			if len(str) >= 1 {
				scheme = str[0]
			}
			stat.Transport = scheme
			switch scheme {
			case "lora":
				fallthrough
			case "ttn-http":
				fallthrough
			case "ttn-mqqt":
				stat.LoraTransport = true
			case "device-udp":
				fallthrough
			case "device-tcp":
				stat.FonaTransport = true
			}

		}

	}
	
    // Done
    return stat

}

// Check an individual measurement
func AggregateMeasurementIntoDataset(ds *MeasurementDataset, stat MeasurementStat) {

    // Only record valid stats
    if !stat.Valid {
        return
    }
    ds.Measurements++

    // Timing
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

	// Transport
	if stat.LoraTransport {
		ds.LoraTransports++
	}
	if stat.FonaTransport {
		ds.FonaTransports++
	}
	foundTransport := false
	for _, c := range strings.Split(ds.Transports, ",") {
		fmt.Printf("c(%d)='%s' transport(%d)='%s' %d\n", len(c), c, len(stat.Transport), stat.Transport, c == stat.Transport)
		if c == stat.Transport {
			foundTransport = true
			break
		}
	}
	if !foundTransport {
		if ds.Transports == "" {
			ds.Transports = stat.Transport
		} else {
			ds.Transports = ds.Transports + "," + stat.Transport
		}
	}
			
    // Done

}

// Check an individual measurement
func GenerateDatasetSummary(ds MeasurementDataset) string {
    s := ""

    // High-level stats
    s += fmt.Sprintf("** Health Check for Device %d\n", ds.DeviceId)
    s += fmt.Sprintf("** %s UTC\n", time.Now().Format(logDateFormat))
    s += fmt.Sprintf("\n")

    s += fmt.Sprintf("Measurements: %d\n", ds.Measurements)
    s += fmt.Sprintf("Period: %s\n", ds.LogRange)
    s += fmt.Sprintf("Oldest: %s\n", ds.OldestUpload.Format("2006-01-02 15:04 UTC"))
    s += fmt.Sprintf("Newest: %s\n", ds.NewestUpload.Format("2006-01-02 15:04 UTC"))
    s += fmt.Sprintf("\n")
	if ds.Measurements == 0 {
		return s
	}
	
	// Inter-measurement timing
    s += fmt.Sprintf("Gaps %s-%s\n", AgoMinutes(ds.MinUploadGapSecs/60), AgoMinutes(ds.MaxUploadGapSecs/60))
    s += fmt.Sprintf("Gaps >1w:   %.0f%% (%d)\n", 100*float32(ds.GapsGt1week)/float32(ds.Measurements), ds.GapsGt1week)
    s += fmt.Sprintf("Gaps >1d:   %.0f%% (%d)\n", 100*float32(ds.GapsGt1day)/float32(ds.Measurements), ds.GapsGt1day)
    s += fmt.Sprintf("Gaps >12hr: %.0f%% (%d)\n", 100*float32(ds.GapsGt12hr)/float32(ds.Measurements), ds.GapsGt12hr)
    s += fmt.Sprintf("Gaps >6hr:  %.0f%% (%d)\n", 100*float32(ds.GapsGt6hr)/float32(ds.Measurements), ds.GapsGt6hr)
    s += fmt.Sprintf("Gaps >2hr:  %.0f%% (%d)\n", 100*float32(ds.GapsGt2hr)/float32(ds.Measurements), ds.GapsGt2hr)
    s += fmt.Sprintf("Gaps >1hr:  %.0f%% (%d)\n", 100*float32(ds.GapsGt1hr)/float32(ds.Measurements), ds.GapsGt1hr)
    s += fmt.Sprintf("Gaps >30m:  %.0f%% (%d)\n", 100*float32(ds.GapsGt30m)/float32(ds.Measurements), ds.GapsGt30m)
    s += fmt.Sprintf("Gaps >15m:  %.0f%% (%d)\n", 100*float32(ds.GapsGt15m)/float32(ds.Measurements), ds.GapsGt15m)
    s += fmt.Sprintf("Gaps >10m:  %.0f%% (%d)\n", 100*float32(ds.GapsGt10m)/float32(ds.Measurements), ds.GapsGt10m)
    s += fmt.Sprintf("Gaps >5m:   %.0f%% (%d)\n", 100*float32(ds.GapsGt5m)/float32(ds.Measurements), ds.GapsGt5m)
    s += fmt.Sprintf("Gaps <=5m:  %.0f%% (%d)\n", 100*float32(ds.Measurements-ds.GapsGt5m)/float32(ds.Measurements), ds.Measurements-ds.GapsGt5m)
    s += fmt.Sprintf("\n")

	// Network
	s += fmt.Sprintf("Transports: %s\n", ds.Transports)
	s += fmt.Sprintf("LoRa: %.0f%% (%d)\n", 100*float32(ds.LoraTransports)/float32(ds.Measurements), ds.LoraTransports)
	s += fmt.Sprintf("Fona: %.0f%% (%d)\n", 100*float32(ds.FonaTransports)/float32(ds.Measurements), ds.FonaTransports)
	
    // Done
    return s
}
