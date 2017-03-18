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
//          - makes sure it got at least some data from each kind of sensor
//          - does some simple range check on each data value

// Stats about a single measurement
type MeasurementStat struct {
    Valid               bool
    Uploaded            time.Time
    LoraModule          string
    FonaModule          string
    Transport           string
    LoraTransport       bool
    FonaTransport       bool
    TestMeasurement     bool
    ErrorsOpc           uint32
    ErrorsPms           uint32
    ErrorsBme0          uint32
    ErrorsBme1          uint32
    ErrorsLora          uint32
    ErrorsFona          uint32
    ErrorsGeiger        uint32
    ErrorsMax01         uint32
    ErrorsUgps          uint32
    ErrorsLis           uint32
    ErrorsSpi           uint32
    ErrorsTwi           uint32
    ErrorsTwiInfo       string
    UptimeMinutes       uint32
}

// Stats about all measurements
type MeasurementDataset struct {
    DeviceId            uint32
    OldestUpload        time.Time
    NewestUpload        time.Time
    MinUploadGapSecs    uint32
    MaxUploadGapSecs    uint32
    GapsGt1week         uint32
    GapsGt1day          uint32
    GapsGt12hr          uint32
    GapsGt6hr           uint32
    GapsGt2hr           uint32
    GapsGt1hr           uint32
    GapsGt30m           uint32
    GapsGt15m           uint32
    GapsGt10m           uint32
    GapsGt5m            uint32
    GapsGt0m            uint32
    Measurements        uint32
    TestMeasurements    bool
    AnyTransport        bool
    Transports          string
    LoraTransports      uint32
    FonaTransports      uint32
    LoraModule          string
    FonaModule          string
    AnyErrors           bool
    PrevErrorsOpc       uint32
    ThisErrorsOpc       uint32
    PrevErrorsPms       uint32
    ThisErrorsPms       uint32
    PrevErrorsBme0      uint32
    ThisErrorsBme0      uint32
    PrevErrorsBme1      uint32
    ThisErrorsBme1      uint32
    PrevErrorsLora      uint32
    ThisErrorsLora      uint32
    PrevErrorsFona      uint32
    ThisErrorsFona      uint32
    PrevErrorsGeiger    uint32
    ThisErrorsGeiger    uint32
    PrevErrorsMax01     uint32
    ThisErrorsMax01     uint32
    PrevErrorsUgps      uint32
    ThisErrorsUgps      uint32
    PrevErrorsLis       uint32
    ThisErrorsLis       uint32
    PrevErrorsSpi       uint32
    ThisErrorsSpi       uint32
    PrevErrorsTwi       uint32
    ThisErrorsTwi       uint32
    ErrorsTwiInfo       string
    PrevUptimeMinutes   uint32
    MaxUptimeMinutes    uint32
    Boots               uint32
}

func NewMeasurementDataset(deviceidstr string) MeasurementDataset {
    ds := MeasurementDataset{}

    u64, _ := strconv.ParseUint(deviceidstr, 10, 32)
    ds.DeviceId = uint32(u64)
    ds.LoraModule = "LoRa"
    ds.FonaModule = "Fona"
    ds.Boots++

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

        if sd.Dev != nil {

            if sd.Dev.ModuleLora != nil {
                stat.LoraModule = *sd.Dev.ModuleLora
            }
            if sd.Dev.ModuleFona != nil {
                stat.FonaModule = *sd.Dev.ModuleFona
            }

            if sd.Dev.ErrorsOpc != nil {
                stat.ErrorsOpc = *sd.Dev.ErrorsOpc
            }
            if sd.Dev.ErrorsPms != nil {
                stat.ErrorsPms = *sd.Dev.ErrorsPms
            }
            if sd.Dev.ErrorsBme0 != nil {
                stat.ErrorsBme0 = *sd.Dev.ErrorsBme0
            }
            if sd.Dev.ErrorsBme1 != nil {
                stat.ErrorsBme1 = *sd.Dev.ErrorsBme1
            }
            if sd.Dev.ErrorsLora != nil {
                stat.ErrorsLora = *sd.Dev.ErrorsLora
            }
            if sd.Dev.ErrorsFona != nil {
                stat.ErrorsFona = *sd.Dev.ErrorsFona
            }
            if sd.Dev.ErrorsGeiger != nil {
                stat.ErrorsGeiger = *sd.Dev.ErrorsGeiger
            }
            if sd.Dev.ErrorsMax01 != nil {
                stat.ErrorsMax01 = *sd.Dev.ErrorsMax01
            }
            if sd.Dev.ErrorsUgps != nil {
                stat.ErrorsUgps = *sd.Dev.ErrorsUgps
            }
            if sd.Dev.ErrorsLis != nil {
                stat.ErrorsLis = *sd.Dev.ErrorsLis
            }
            if sd.Dev.ErrorsSpi != nil {
                stat.ErrorsSpi = *sd.Dev.ErrorsSpi
            }
            if sd.Dev.ErrorsTwi != nil {
                stat.ErrorsTwi = *sd.Dev.ErrorsTwi
            }
            if sd.Dev.ErrorsTwiInfo != nil {
                stat.ErrorsTwiInfo = *sd.Dev.ErrorsTwiInfo
            }

            if sd.Dev.UptimeMinutes != nil {
                stat.UptimeMinutes = *sd.Dev.UptimeMinutes
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
    if SecondsGap > 0 {
        if ds.MinUploadGapSecs == 0 || SecondsGap < ds.MinUploadGapSecs {
            ds.MinUploadGapSecs = SecondsGap
        }
        if SecondsGap > ds.MaxUploadGapSecs {
            ds.MaxUploadGapSecs = SecondsGap
        }
    }
    if SecondsGap > 0 {
        ds.GapsGt0m++
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
    ds.NewestUpload = stat.Uploaded

    // Transport
    if stat.LoraTransport {
        ds.LoraTransports++
        ds.AnyTransport = true
    }
    if stat.FonaTransport {
        ds.FonaTransports++
        ds.AnyTransport = true
    }
    foundTransport := false
    for _, c := range strings.Split(ds.Transports, ",") {
        if c == stat.Transport {
            foundTransport = true
            break
        }
    }
    if !foundTransport {
        ds.AnyTransport = true
        if ds.Transports == "" {
            ds.Transports = stat.Transport
        } else {
            ds.Transports = ds.Transports + "," + stat.Transport
        }
    }

    if stat.LoraModule != "" {
        ds.LoraModule = stat.LoraModule
    }
    if stat.FonaModule != "" {
        ds.FonaModule = stat.FonaModule
    }

    // Errors this session
    if stat.ErrorsOpc > ds.ThisErrorsOpc {
        ds.ThisErrorsOpc = stat.ErrorsOpc
        ds.AnyErrors = true
    }
    if stat.ErrorsPms > ds.ThisErrorsPms {
        ds.ThisErrorsPms = stat.ErrorsPms
        ds.AnyErrors = true
    }
    if stat.ErrorsBme0 > ds.ThisErrorsBme0 {
        ds.ThisErrorsBme0 = stat.ErrorsBme0
        ds.AnyErrors = true
    }
    if stat.ErrorsBme1 > ds.ThisErrorsBme1 {
        ds.ThisErrorsBme1 = stat.ErrorsBme1
        ds.AnyErrors = true
    }
    if stat.ErrorsLora > ds.ThisErrorsLora {
        ds.ThisErrorsLora = stat.ErrorsLora
        ds.AnyErrors = true
    }
    if stat.ErrorsFona > ds.ThisErrorsFona {
        ds.ThisErrorsFona = stat.ErrorsFona
        ds.AnyErrors = true
    }
    if stat.ErrorsGeiger > ds.ThisErrorsGeiger {
        ds.ThisErrorsGeiger = stat.ErrorsGeiger
        ds.AnyErrors = true
    }
    if stat.ErrorsMax01 > ds.ThisErrorsMax01 {
        ds.ThisErrorsMax01 = stat.ErrorsMax01
        ds.AnyErrors = true
    }
    if stat.ErrorsUgps > ds.ThisErrorsUgps {
        ds.ThisErrorsUgps = stat.ErrorsUgps
        ds.AnyErrors = true
    }
    if stat.ErrorsLis > ds.ThisErrorsLis {
        ds.ThisErrorsLis = stat.ErrorsLis
        ds.AnyErrors = true
    }
    if stat.ErrorsSpi > ds.ThisErrorsSpi {
        ds.ThisErrorsSpi = stat.ErrorsSpi
        ds.AnyErrors = true
    }
    if stat.ErrorsTwi > ds.ThisErrorsTwi {
        ds.ThisErrorsTwi = stat.ErrorsTwi
        ds.AnyErrors = true
    }

    for _, staterr := range strings.Split(stat.ErrorsTwiInfo, ",") {
        foundError := false
        if staterr != "" {
            for _, c := range strings.Split(ds.ErrorsTwiInfo, ",") {
                if c == staterr {
                    foundError = true
                    break
                }
            }
            if !foundError {
                ds.AnyErrors = true
                if ds.ErrorsTwiInfo == "" {
                    ds.ErrorsTwiInfo = staterr
                } else {
                    ds.ErrorsTwiInfo = ds.ErrorsTwiInfo + "," + staterr
                }
            }
        }
    }

    // Uptime
    if stat.UptimeMinutes != 0 {
        if stat.UptimeMinutes > ds.MaxUptimeMinutes {
            ds.MaxUptimeMinutes = stat.UptimeMinutes
        }
        if stat.UptimeMinutes < ds.PrevUptimeMinutes {
            ds.Boots++

            // Add errors to running totals from prior boots
            ds.PrevErrorsOpc += ds.ThisErrorsOpc
            ds.ThisErrorsOpc = 0
            ds.PrevErrorsPms += ds.ThisErrorsPms
            ds.ThisErrorsPms = 0
            ds.PrevErrorsBme0 += ds.ThisErrorsBme0
            ds.ThisErrorsBme0 = 0
            ds.PrevErrorsBme1 += ds.ThisErrorsBme1
            ds.ThisErrorsBme1 = 0
            ds.PrevErrorsLora += ds.ThisErrorsLora
            ds.ThisErrorsLora = 0
            ds.PrevErrorsFona += ds.ThisErrorsFona
            ds.ThisErrorsFona = 0
            ds.PrevErrorsGeiger += ds.ThisErrorsGeiger
            ds.ThisErrorsGeiger = 0
            ds.PrevErrorsMax01 += ds.ThisErrorsMax01
            ds.ThisErrorsMax01 = 0
            ds.PrevErrorsUgps += ds.ThisErrorsUgps
            ds.ThisErrorsUgps = 0
            ds.PrevErrorsLis += ds.ThisErrorsLis
            ds.ThisErrorsLis = 0
            ds.PrevErrorsSpi += ds.ThisErrorsSpi
            ds.ThisErrorsSpi = 0
            ds.PrevErrorsTwi += ds.ThisErrorsTwi
            ds.ThisErrorsTwi = 0

        }
        ds.PrevUptimeMinutes = stat.UptimeMinutes
    }

    // Done

}

// Check an individual measurement
func GenerateDatasetSummary(ds MeasurementDataset) string {
    s := ""

    // High-level stats
    s += fmt.Sprintf("** %d Health Check\n", ds.DeviceId)
    s += fmt.Sprintf("** %s UTC\n", time.Now().Format(logDateFormat))
    s += fmt.Sprintf("\n")

    if ds.Boots == 1 {
        if ds.MaxUptimeMinutes != 0 {
            s += fmt.Sprintf("Uptime: %s\n", AgoMinutes(ds.MaxUptimeMinutes))
        }
    } else {
        s += fmt.Sprintf("Max uptime of %s across %d sessions\n", AgoMinutes(ds.MaxUptimeMinutes), ds.Boots)
    }

    s += fmt.Sprintf("%d measurements in %s\n", ds.Measurements, AgoMinutes(uint32(ds.NewestUpload.Sub(ds.OldestUpload)/time.Minute)))
    s += fmt.Sprintf("Oldest: %s\n", ds.OldestUpload.Format("2006-01-02 15:04 UTC"))
    s += fmt.Sprintf("Newest: %s\n", ds.NewestUpload.Format("2006-01-02 15:04 UTC"))
    s += fmt.Sprintf("\n")
    if ds.Measurements == 0 {
        return s
    }

    // Inter-measurement timing
    s += fmt.Sprintf("Gaps %s - %s\n", AgoMinutes(ds.MinUploadGapSecs/60), AgoMinutes(ds.MaxUploadGapSecs/60))
    s += fmt.Sprintf("Gaps >1w:   %.0f%% (%d)\n", 100*float32(ds.GapsGt1week)/float32(ds.GapsGt0m), ds.GapsGt1week)
    s += fmt.Sprintf("Gaps >1d:   %.0f%% (%d)\n", 100*float32(ds.GapsGt1day)/float32(ds.GapsGt0m), ds.GapsGt1day)
    s += fmt.Sprintf("Gaps >12hr: %.0f%% (%d)\n", 100*float32(ds.GapsGt12hr)/float32(ds.GapsGt0m), ds.GapsGt12hr)
    s += fmt.Sprintf("Gaps >6hr:  %.0f%% (%d)\n", 100*float32(ds.GapsGt6hr)/float32(ds.GapsGt0m), ds.GapsGt6hr)
    s += fmt.Sprintf("Gaps >2hr:  %.0f%% (%d)\n", 100*float32(ds.GapsGt2hr)/float32(ds.GapsGt0m), ds.GapsGt2hr)
    s += fmt.Sprintf("Gaps >1hr:  %.0f%% (%d)\n", 100*float32(ds.GapsGt1hr)/float32(ds.GapsGt0m), ds.GapsGt1hr)
    s += fmt.Sprintf("Gaps >30m:  %.0f%% (%d)\n", 100*float32(ds.GapsGt30m)/float32(ds.GapsGt0m), ds.GapsGt30m)
    s += fmt.Sprintf("Gaps >15m:  %.0f%% (%d)\n", 100*float32(ds.GapsGt15m)/float32(ds.GapsGt0m), ds.GapsGt15m)
    s += fmt.Sprintf("Gaps >10m:  %.0f%% (%d)\n", 100*float32(ds.GapsGt10m)/float32(ds.GapsGt0m), ds.GapsGt10m)
    s += fmt.Sprintf("Gaps >5m:   %.0f%% (%d)\n", 100*float32(ds.GapsGt5m)/float32(ds.GapsGt0m), ds.GapsGt5m)
    s += fmt.Sprintf("Gaps <=5m:  %.0f%% (%d)\n", 100*float32(ds.GapsGt0m-ds.GapsGt5m)/float32(ds.GapsGt0m), ds.GapsGt0m-ds.GapsGt5m)
    s += fmt.Sprintf("\n")

    // Network
    if ds.AnyTransport {
        s += fmt.Sprintf("Transports: %s\n", ds.Transports)
        s += fmt.Sprintf("%s: %.0f%% (%d)\n", ds.LoraModule, 100*float32(ds.LoraTransports)/float32(ds.Measurements), ds.LoraTransports)
        s += fmt.Sprintf("%s: %.0f%% (%d)\n", ds.FonaModule, 100*float32(ds.FonaTransports)/float32(ds.Measurements), ds.FonaTransports)
        s += fmt.Sprintf("\n")
    }

    // Errors
    if ds.AnyErrors {
        if ds.Boots == 1 {
            s += fmt.Sprintf("Errors:\n")
        } else {
            s += fmt.Sprintf("Errors across %d sessions:\n", ds.Boots)
        }
        s += fmt.Sprintf("Opc:    %d\n", ds.PrevErrorsOpc+ds.ThisErrorsOpc)
        s += fmt.Sprintf("Pms:    %d\n", ds.PrevErrorsPms+ds.ThisErrorsPms)
        s += fmt.Sprintf("Bme0:   %d\n", ds.PrevErrorsBme0+ds.ThisErrorsBme0)
        s += fmt.Sprintf("Bme1:   %d\n", ds.PrevErrorsBme1+ds.ThisErrorsBme1)
        s += fmt.Sprintf("Lora:   %d\n", ds.PrevErrorsLora+ds.ThisErrorsLora)
        s += fmt.Sprintf("Fona:   %d\n", ds.PrevErrorsFona+ds.ThisErrorsFona)
        s += fmt.Sprintf("Geiger: %d\n", ds.PrevErrorsGeiger+ds.ThisErrorsGeiger)
        s += fmt.Sprintf("Max01:  %d\n", ds.PrevErrorsMax01+ds.ThisErrorsMax01)
        s += fmt.Sprintf("Ugps:   %d\n", ds.PrevErrorsUgps+ds.ThisErrorsUgps)
        s += fmt.Sprintf("Lis:    %d\n", ds.PrevErrorsLis+ds.ThisErrorsLis)
        s += fmt.Sprintf("Spi:    %d\n", ds.PrevErrorsSpi+ds.ThisErrorsSpi)
        s += fmt.Sprintf("Twi:    %d %s\n", ds.PrevErrorsTwi+ds.ThisErrorsTwi, ds.ErrorsTwiInfo)
        s += fmt.Sprintf("\n")
    }

    // Done
    return s
}
