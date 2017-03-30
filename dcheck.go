// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Support for device performance verification
package main

import (
    "fmt"
    "math"
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
    Test                bool
    Firmware            string
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
    ErrorsCommsPowerFails uint32
    ErrorsGeiger        uint32
    ErrorsMax01         uint32
    ErrorsUgps          uint32
    ErrorsLis           uint32
    ErrorsSpi           uint32
    ErrorsTwi           uint32
    ErrorsTwiInfo       string
    ErrorsConnectLora   uint32
    ErrorsConnectFona   uint32
    ErrorsConnectWireless uint32
    ErrorsConnectData   uint32
    ErrorsConnectService uint32
    ErrorsCommsFailures uint32
    ErrorsDeviceRestarts uint32
    UptimeMinutes       uint32
    hasBat              bool
    BatWarning          bool
    hasEnv              bool
    EnvWarning          bool
    hasEnc              bool
    EncWarning          bool
    hasLndU7318         bool
    hasLndC7318         bool
    hasLndEC7128        bool
    GeigerWarning       bool
    hasPms              bool
    PmsWarning          bool
    hasOpc              bool
    OpcWarning          bool

}

// Stats about all measurements
type MeasurementDataset struct {
    DeviceId            uint32
    Firmware            string
    MultiFirmware       bool
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
    GapsGt55m           uint32
    GapsGt50m           uint32
    GapsGt45m           uint32
    GapsGt40m           uint32
    GapsGt35m           uint32
    GapsGt30m           uint32
    GapsGt25m           uint32
    GapsGt20m           uint32
    GapsGt15m           uint32
    GapsGt10m           uint32
    GapsGt5m            uint32
    GapsGt0m            uint32
    Measurements        uint32
    TestMeasurements    uint32
    AnyTransport        bool
    Transports          string
    LoraTransports      uint32
    FonaTransports      uint32
    LoraModule          string
    FonaModule          string
    AnyErrors           bool
    AnyConnectErrors    bool
    AnyPointcastErrors  bool
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
    PrevErrorsCommsPowerFails uint32
    ThisErrorsCommsPowerFails uint32
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
    ThisErrorsConnectLora uint32
    PrevErrorsConnectLora uint32
    ThisErrorsConnectFona uint32
    PrevErrorsConnectFona uint32
    ThisErrorsConnectWireless uint32
    PrevErrorsConnectWireless uint32
    ThisErrorsConnectData uint32
    PrevErrorsConnectData uint32
    ThisErrorsConnectService uint32
    PrevErrorsConnectService uint32
    MinErrorsCommsFailures uint32
    ThisErrorsCommsFailures uint32
    PrevErrorsCommsFailures uint32
    MinErrorsDeviceRestarts uint32
    ThisErrorsDeviceRestarts uint32
    PrevErrorsDeviceRestarts uint32
    PrevUptimeMinutes   uint32
    MaxUptimeMinutes    uint32
    Boots               uint32
    BatCount            uint32
    BatWarningCount     uint32
    BatWarningFirst     time.Time
    EnvCount            uint32
    EnvWarningCount     uint32
    EnvWarningFirst     time.Time
    EncCount            uint32
    EncWarningCount     uint32
    EncWarningFirst     time.Time
    LndU7318Count       uint32
    LndC7318Count       uint32
    LndEC7128Count      uint32
    GeigerWarningCount  int32
    GeigerWarningFirst  time.Time
    PmsCount            uint32
    PmsWarningCount     uint32
    PmsWarningFirst     time.Time
    OpcCount            uint32
    OpcWarningCount     uint32
    OpcWarningFirst     time.Time
}

func NewMeasurementDataset(deviceidstr string) MeasurementDataset {
    ds := MeasurementDataset{}

    u64, _ := strconv.ParseUint(deviceidstr, 10, 32)
    ds.DeviceId = uint32(u64)
    ds.LoraModule = "unidentified lora module"
    ds.FonaModule = "unidentified fona module"
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

    }

    if sd.Dev != nil {

        if sd.Dev.Test != nil {
            stat.Test = *sd.Dev.Test
        }

        if sd.Dev.AppVersion != nil {
            firmware := *sd.Dev.AppVersion
            pieces := strings.Split(firmware, ".")
            if len(pieces) >= 3 {
                stat.Firmware = pieces[2]
            }
        }

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
        if sd.Dev.CommsPowerFails != nil {
            stat.ErrorsCommsPowerFails = *sd.Dev.CommsPowerFails
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
        if sd.Dev.ErrorsConnectLora != nil {
            stat.ErrorsConnectLora = *sd.Dev.ErrorsConnectLora
        }
        if sd.Dev.ErrorsConnectFona != nil {
            stat.ErrorsConnectFona = *sd.Dev.ErrorsConnectFona
        }
        if sd.Dev.ErrorsConnectWireless != nil {
            stat.ErrorsConnectWireless = *sd.Dev.ErrorsConnectWireless
        }
        if sd.Dev.ErrorsConnectData != nil {
            stat.ErrorsConnectData = *sd.Dev.ErrorsConnectData
        }
        if sd.Dev.ErrorsConnectService != nil {
            stat.ErrorsConnectService = *sd.Dev.ErrorsConnectService
        }
        if sd.Dev.CommsFails != nil {
            stat.ErrorsCommsFailures = *sd.Dev.CommsFails
        }
        if sd.Dev.DeviceRestarts != nil {
            stat.ErrorsDeviceRestarts = *sd.Dev.DeviceRestarts
        }

        if sd.Dev.UptimeMinutes != nil {
            stat.UptimeMinutes = *sd.Dev.UptimeMinutes
        }

    }

    if sd.Bat != nil {
        stat.hasBat = true
        if sd.Bat.Voltage != nil {
            val := *sd.Bat.Voltage
            if stat.Transport == "pointcast" || stat.Transport == "safecast-air" {
                if val < 3.0 || val > 12.0 {
                    stat.BatWarning = true
                }
            } else {
                if val < 3.0 || val > 4.5 {
                    stat.BatWarning = true
                }
            }
        }
        if sd.Bat.Current != nil {
            val := *sd.Bat.Current
            if val < -2000.0 || val > 100 {
                stat.BatWarning = true
            }
        }
        // As of 2017-03-23 we no longer verify charge, for two reasons:
        // 1) most devices require SOC training, and thus fall out of range
        // 2) we don't actually use SOC for device performance throttling,
        //    instead using a calculation derived from the voltage.  As such,
        //    SOC is largely for informational purposes.
        if (false) {
            if sd.Bat.Charge != nil {
                val := *sd.Bat.Charge
                if val < 25.0 || val > 200 {
                    stat.BatWarning = true
                }
            }
        }
    }

    if sd.Env != nil {
        stat.hasEnv = true
        if sd.Env.Temp != nil {
            val := *sd.Env.Temp
            if val < -25.0 || val > 80.0 {
                stat.EnvWarning = true
            }
        }
        if sd.Env.Humid != nil {
            val := *sd.Env.Humid
            if val < 0 || val > 100 {
                stat.EnvWarning = true
            }
        }
    }

    if sd.Dev != nil {
        if sd.Dev.Temp != nil {
            stat.hasEnc = true
            val := *sd.Dev.Temp
            if val < -25.0 || val > 80.0 {
                stat.EncWarning = true
            }
        }
        if sd.Dev.Humid != nil {
            stat.hasEnc = true
            val := *sd.Dev.Humid
            if val < 0 || val > 100 {
                stat.EncWarning = true
            }
        }
    }

    if sd.Lnd != nil {
        if sd.Lnd.U7318 != nil {
            stat.hasLndU7318 = true
            val := *sd.Lnd.U7318
            if val < 0 || val > 500 {
                stat.GeigerWarning = true
            }
        }
        if sd.Lnd.C7318 != nil {
            stat.hasLndC7318 = true
            val := *sd.Lnd.C7318
            if val < 0 || val > 500 {
                stat.GeigerWarning = true
            }
        }
        if sd.Lnd.EC7128 != nil {
            stat.hasLndEC7128 = true
            val := *sd.Lnd.EC7128
            if val < 0 || val > 500 {
                stat.GeigerWarning = true
            }
        }
    }

    if sd.Pms != nil {
        stat.hasPms = true
        if sd.Pms.Pm01_0 != nil {
            val := *sd.Pms.Pm01_0
            if val < -0 || val > 600 {
                stat.PmsWarning = true
            }
        }
        if sd.Pms.Pm02_5 != nil {
            val := *sd.Pms.Pm02_5
            if val < -0 || val > 600 {
                stat.PmsWarning = true
            }
        }
        if sd.Pms.Pm10_0 != nil {
            val := *sd.Pms.Pm10_0
            if val < -0 || val > 600 {
                stat.PmsWarning = true
            }
        }
    }

    if sd.Opc != nil {
        stat.hasOpc = true
        if sd.Opc.Pm01_0 != nil {
            val := *sd.Opc.Pm01_0
            if val < -0 || val > 600 {
                stat.OpcWarning = true
            }
        }
        if sd.Opc.Pm02_5 != nil {
            val := *sd.Opc.Pm02_5
            if val < -0 || val > 600 {
                stat.OpcWarning = true
            }
        }
        if sd.Opc.Pm10_0 != nil {
            val := *sd.Opc.Pm10_0
            if val < -0 || val > 600 {
                stat.OpcWarning = true
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
    if stat.Test {
        ds.TestMeasurements++
    }

    // Firmware
    if stat.Firmware != "" {
        foundFirmware := false
        for _, c := range strings.Split(ds.Firmware, ",") {
            if c == stat.Firmware {
                foundFirmware = true
                break
            }
        }
        if !foundFirmware {
            if ds.Firmware == "" {
                ds.Firmware = stat.Firmware
            } else {
                ds.Firmware = ds.Firmware + "," + stat.Firmware
                ds.MultiFirmware = true
            }
        }
    }

    // Init oldest and newest
    if ds.Measurements == 1 {
        ds.OldestUpload = stat.Uploaded
        ds.NewestUpload = stat.Uploaded
    }

    // Timing.  Note that it is possible to have uploads that are out-of-order because of
    // multi-instance server concurrency.  For those measurements we will still check the
    // sensor readings, but we won't factor the measurement into our "gap" calculations.
    if stat.Uploaded.Sub(ds.NewestUpload) >= 0 {

        SecondsGap := uint32(stat.Uploaded.Sub(ds.NewestUpload) / time.Second)
		MinutesGap := SecondsGap / 60
		if SecondsGap != 0 && MinutesGap == 0 {
			SecondsGap = 1
		} else {
			SecondsGap = MinutesGap * 60
		}
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
        if SecondsGap > 60 * 20 {
            ds.GapsGt20m++
        }
        if SecondsGap > 60 * 25 {
            ds.GapsGt25m++
        }
        if SecondsGap > 60 * 30 {
            ds.GapsGt30m++
        }
        if SecondsGap > 60 * 35 {
            ds.GapsGt35m++
        }
        if SecondsGap > 60 * 40 {
            ds.GapsGt40m++
        }
        if SecondsGap > 60 * 45 {
            ds.GapsGt45m++
        }
        if SecondsGap > 60 * 50 {
            ds.GapsGt50m++
        }
        if SecondsGap > 60 * 55 {
            ds.GapsGt55m++
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
    }

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
    if stat.ErrorsCommsPowerFails > ds.ThisErrorsCommsPowerFails {
        ds.ThisErrorsCommsPowerFails = stat.ErrorsCommsPowerFails
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

    for _, staterr := range strings.Split(stat.ErrorsTwiInfo, " ") {
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

    // Connect errors
    if stat.ErrorsConnectLora > ds.ThisErrorsConnectLora {
        ds.ThisErrorsConnectLora = stat.ErrorsConnectLora
        ds.AnyConnectErrors = true
    }
    if stat.ErrorsConnectFona > ds.ThisErrorsConnectFona {
        ds.ThisErrorsConnectFona = stat.ErrorsConnectFona
        ds.AnyConnectErrors = true
    }
    if stat.ErrorsConnectWireless > ds.ThisErrorsConnectWireless {
        ds.ThisErrorsConnectWireless = stat.ErrorsConnectWireless
        ds.AnyConnectErrors = true
    }
    if stat.ErrorsConnectData > ds.ThisErrorsConnectData {
        ds.ThisErrorsConnectData = stat.ErrorsConnectData
        ds.AnyConnectErrors = true
    }
    if stat.ErrorsConnectService > ds.ThisErrorsConnectService {
        ds.ThisErrorsConnectService = stat.ErrorsConnectService
        ds.AnyConnectErrors = true
    }
    if stat.ErrorsCommsFailures != 0 {
        if ds.MinErrorsCommsFailures == 0 || (ds.MinErrorsCommsFailures != 0 && stat.ErrorsCommsFailures < ds.MinErrorsCommsFailures) {
            ds.MinErrorsCommsFailures = stat.ErrorsCommsFailures
        }
    }
    if stat.ErrorsCommsFailures > ds.ThisErrorsCommsFailures {
        ds.ThisErrorsCommsFailures = stat.ErrorsCommsFailures
        ds.AnyPointcastErrors = true
    }
    if stat.ErrorsDeviceRestarts != 0 {
        if ds.MinErrorsDeviceRestarts == 0 || (ds.MinErrorsDeviceRestarts != 0 && stat.ErrorsDeviceRestarts < ds.MinErrorsDeviceRestarts) {
            ds.MinErrorsDeviceRestarts = stat.ErrorsDeviceRestarts
        }
    }
    if stat.ErrorsDeviceRestarts > ds.ThisErrorsDeviceRestarts {
        ds.ThisErrorsDeviceRestarts = stat.ErrorsDeviceRestarts
        ds.AnyPointcastErrors = true
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
            ds.PrevErrorsCommsPowerFails += ds.ThisErrorsCommsPowerFails
            ds.ThisErrorsCommsPowerFails = 0
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
            ds.PrevErrorsConnectLora += ds.ThisErrorsConnectLora
            ds.ThisErrorsConnectLora = 0
            ds.PrevErrorsConnectFona += ds.ThisErrorsConnectFona
            ds.ThisErrorsConnectFona = 0
            ds.PrevErrorsConnectWireless += ds.ThisErrorsConnectWireless
            ds.ThisErrorsConnectWireless = 0
            ds.PrevErrorsConnectData += ds.ThisErrorsConnectData
            ds.ThisErrorsConnectData = 0
            ds.PrevErrorsConnectService += ds.ThisErrorsConnectService
            ds.ThisErrorsConnectService = 0
            ds.PrevErrorsCommsFailures += ds.ThisErrorsCommsFailures
            ds.ThisErrorsCommsFailures = 0
            ds.PrevErrorsDeviceRestarts += ds.ThisErrorsDeviceRestarts
            ds.ThisErrorsDeviceRestarts = 0

        }
        ds.PrevUptimeMinutes = stat.UptimeMinutes
    }

    // Sensors
    if stat.hasBat {
        ds.BatCount++
    }
    if stat.hasEnv {
        ds.EnvCount++
    }
    if stat.hasEnc {
        ds.EncCount++
    }
    if stat.hasLndU7318 {
        ds.LndU7318Count++
    }
    if stat.hasLndC7318 {
        ds.LndC7318Count++
    }
    if stat.hasLndEC7128 {
        ds.LndEC7128Count++
    }
    if stat.hasPms {
        ds.PmsCount++
    }
    if stat.hasOpc {
        ds.OpcCount++
    }
    if stat.BatWarning {
        if ds.BatWarningCount == 0 {
            ds.BatWarningFirst = stat.Uploaded
        }
        ds.BatWarningCount++
    }
    if stat.EnvWarning {
        if ds.EnvWarningCount == 0 {
            ds.EnvWarningFirst = stat.Uploaded
        }
        ds.EnvWarningCount++
    }
    if stat.EncWarning {
        if ds.EncWarningCount == 0 {
            ds.EncWarningFirst = stat.Uploaded
        }
        ds.EncWarningCount++
    }
    if stat.GeigerWarning {
        if ds.GeigerWarningCount == 0 {
            ds.GeigerWarningFirst = stat.Uploaded
        }
        ds.GeigerWarningCount++
    }
    if stat.PmsWarning {
        if ds.PmsWarningCount == 0 {
            ds.PmsWarningFirst = stat.Uploaded
        }
        ds.PmsWarningCount++
    }
    if stat.OpcWarning {
        if ds.OpcWarningCount == 0 {
            ds.OpcWarningFirst = stat.Uploaded
        }
        ds.OpcWarningCount++
    }

    // Done

}

// Check an individual measurement
func GenerateDatasetSummary(ds MeasurementDataset) string {
    s := ""

    // High-level stats
    s += fmt.Sprintf("Checkup:\n")
    s += fmt.Sprintf("  id %d", ds.DeviceId)
    if time.Now().Sub(ds.NewestUpload)/time.Minute > 90 {
        s += fmt.Sprintf(" (OFFLINE)")
    }
    s += fmt.Sprintf("\n")
    s += fmt.Sprintf("  at %s\n", time.Now().Format("2006-01-02 15:04 UTC"))
    if ds.Firmware != "" {
        s += fmt.Sprintf("  on %s\n", ds.Firmware)
    }
    s += fmt.Sprintf("\n")

    if ds.Boots == 1 {
        if ds.MaxUptimeMinutes != 0 {
            s += fmt.Sprintf("Uptime:\n")
            s += fmt.Sprintf("  %s\n", AgoMinutes(ds.MaxUptimeMinutes))
            s += fmt.Sprintf("\n");
        }
    } else {
        s += fmt.Sprintf("Uptime:\n")
        s += fmt.Sprintf("  %s maximum found in %d sessions\n", AgoMinutes(ds.MaxUptimeMinutes), ds.Boots)
        s += fmt.Sprintf("\n");
    }

    s += fmt.Sprintf("Uploads:\n");
    s += fmt.Sprintf("  Total  %d over the course of %s\n", ds.Measurements, AgoMinutes(uint32(ds.NewestUpload.Sub(ds.OldestUpload)/time.Minute)))
    if ds.TestMeasurements != 0 {
        if ds.Measurements == ds.TestMeasurements {
            s += fmt.Sprintf("         (All of those are TEST measurements)\n");
        } else {
            s += fmt.Sprintf("         (%d of those are TEST measurements)\n", ds.TestMeasurements)
        }
    }
    s += fmt.Sprintf("  Oldest %s\n", ds.OldestUpload.Format("2006-01-02 15:04 UTC"))
    s += fmt.Sprintf("  Newest %s\n", ds.NewestUpload.Format("2006-01-02 15:04 UTC"))
    s += fmt.Sprintf("\n")

    // Network
    s += fmt.Sprintf("Communications:\n  over  %s\n", ds.Transports)
    if ds.AnyTransport {
        if ds.LoraTransports != 0 {
            s += fmt.Sprintf("  using%4.0f%% (%d) %s\n", 100*float32(ds.LoraTransports)/float32(ds.Measurements), ds.LoraTransports, ds.LoraModule)
        }
        if ds.FonaTransports != 0 {
            s += fmt.Sprintf("  using%4.0f%% (%d) %s\n", 100*float32(ds.FonaTransports)/float32(ds.Measurements), ds.FonaTransports, ds.FonaModule)
        }
    }
    s += fmt.Sprintf("\n")

    // Timing
    if ds.Measurements == 0 {
        return s
    }
    if ds.Measurements > 1 {
        s += fmt.Sprintf("Inter-upload Gaps: (%s to %s)\n", AgoMinutes(ds.MinUploadGapSecs/60), AgoMinutes(ds.MaxUploadGapSecs/60))
        f := 100*float32(ds.GapsGt1week) / float32(ds.GapsGt0m)
        if f != 0 {
            s += fmt.Sprintf("  >1w     %3.0f%% (%d)\n", f, ds.GapsGt1week)
        }
		g := ds.GapsGt1day - ds.GapsGt1week
        f = 100*float32(g) / float32(ds.GapsGt0m) 
        if f != 0 && ds.GapsGt1day != ds.GapsGt1week {
            s += fmt.Sprintf("  1d-1w   %3.0f%% (%d)\n", f, g)
        }
		g = ds.GapsGt12hr - ds.GapsGt1day
        f = 100*float32(g) / float32(ds.GapsGt0m)
        if f != 0 && ds.GapsGt12hr != ds.GapsGt1day {
            s += fmt.Sprintf("  12-24hr %3.0f%% (%d)\n", f, g)
        }
		g = ds.GapsGt6hr - ds.GapsGt12hr
        f = 100*float32(g) / float32(ds.GapsGt0m) 
        if f != 0 && ds.GapsGt6hr != ds.GapsGt12hr {
            s += fmt.Sprintf("  6-12hr  %3.0f%% (%d)\n", f, g)
        }
		g = ds.GapsGt2hr - ds.GapsGt6hr
        f = 100*float32(g) / float32(ds.GapsGt0m) 
        if f != 0 && ds.GapsGt2hr != ds.GapsGt6hr {
            s += fmt.Sprintf("  2-6hr   %3.0f%% (%d)\n", f, g)
        }
		g = ds.GapsGt1hr - ds.GapsGt2hr
        f = 100*float32(g) / float32(ds.GapsGt0m) 
        if f != 0 && ds.GapsGt1hr != ds.GapsGt2hr {
            s += fmt.Sprintf("  1-2hr   %3.0f%% (%d)\n", f, g)
        }
		g = ds.GapsGt55m - ds.GapsGt1hr
        f = 100*float32(g) / float32(ds.GapsGt0m) 
        if f != 0 && ds.GapsGt55m != ds.GapsGt1hr {
            s += fmt.Sprintf("  56-60m  %3.0f%% (%d)\n", f, g)
        }
		g = ds.GapsGt50m - ds.GapsGt55m
        f = 100*float32(g) / float32(ds.GapsGt0m) 
        if f != 0 && ds.GapsGt50m != ds.GapsGt55m {
            s += fmt.Sprintf("  51-55m  %3.0f%% (%d)\n", f, g)
        }
		g = ds.GapsGt45m - ds.GapsGt50m
        f = 100*float32(g) / float32(ds.GapsGt0m) 
        if f != 0 && ds.GapsGt45m != ds.GapsGt50m {
            s += fmt.Sprintf("  46-50m  %3.0f%% (%d)\n", f, g)
        }
		g = ds.GapsGt40m - ds.GapsGt45m
        f = 100*float32(g) / float32(ds.GapsGt0m) 
        if f != 0 && ds.GapsGt40m != ds.GapsGt45m {
            s += fmt.Sprintf("  41-45m  %3.0f%% (%d)\n", f, g)
        }
		g = ds.GapsGt35m - ds.GapsGt40m
        f = 100*float32(g) / float32(ds.GapsGt0m) 
        if f != 0 && ds.GapsGt35m != ds.GapsGt40m {
            s += fmt.Sprintf("  36-40m  %3.0f%% (%d)\n", f, g)
        }
		g = ds.GapsGt30m - ds.GapsGt35m
        f = 100*float32(g) / float32(ds.GapsGt0m) 
        if f != 0 && ds.GapsGt30m != ds.GapsGt35m {
            s += fmt.Sprintf("  31-35m  %3.0f%% (%d)\n", f, g)
        }
		g = ds.GapsGt25m - ds.GapsGt30m
        f = 100*float32(g) / float32(ds.GapsGt0m) 
        if f != 0 && ds.GapsGt25m != ds.GapsGt30m {
            s += fmt.Sprintf("  26-30m  %3.0f%% (%d)\n", f, g)
        }
		g = ds.GapsGt20m - ds.GapsGt25m
        f = 100*float32(g) / float32(ds.GapsGt0m) 
        if f != 0 && ds.GapsGt20m != ds.GapsGt25m {
            s += fmt.Sprintf("  21-25m  %3.0f%% (%d)\n", f, g)
        }
		g = ds.GapsGt15m - ds.GapsGt20m
        f = 100*float32(g) / float32(ds.GapsGt0m) 
        if f != 0 && ds.GapsGt15m != ds.GapsGt20m {
            s += fmt.Sprintf("  16-20m  %3.0f%% (%d)\n", f, g)
        }
		g = ds.GapsGt10m - ds.GapsGt15m
        f = 100*float32(g) / float32(ds.GapsGt0m) 
        if f != 0 && ds.GapsGt10m != ds.GapsGt15m {
            s += fmt.Sprintf("  11-15m  %3.0f%% (%d)\n", f, g)
        }
		g = ds.GapsGt5m - ds.GapsGt10m
        f = 100*float32(g) / float32(ds.GapsGt0m) 
        if f != 0 && ds.GapsGt5m != ds.GapsGt10m {
            s += fmt.Sprintf("   5-10m  %3.0f%% (%d)\n", f, g)
        }
		g = ds.GapsGt0m - ds.GapsGt5m
        f = 100*float32(g) / float32(ds.GapsGt0m)
        if f != 0 {
            s += fmt.Sprintf("    1-4m  %3.0f%% (%d)\n", f, g)
        }
        s += fmt.Sprintf("\n")
    }

    // Connect errors
    s += fmt.Sprintf("Connection errors:\n")
    if !ds.AnyConnectErrors {
        s += fmt.Sprintf("  None\n")
    } else {
        i := ds.PrevErrorsConnectLora + ds.ThisErrorsConnectLora
        if i > 0 {
            s += fmt.Sprintf("  Lora     %d\n", i)
        }
        i = ds.PrevErrorsConnectFona + ds.ThisErrorsConnectFona
        if i > 0 {
            s += fmt.Sprintf("  Fona     %d\n", i)
        }
        i = ds.PrevErrorsConnectWireless + ds.ThisErrorsConnectWireless
        if i > 0 {
            s += fmt.Sprintf("  Wireless %d\n", i)
        }
        i = ds.PrevErrorsConnectData + ds.ThisErrorsConnectData
        if i > 0 {
            s += fmt.Sprintf("  Data     %d\n", i)
        }
        i = ds.PrevErrorsConnectService + ds.ThisErrorsConnectService
        if i > 0 {
            s += fmt.Sprintf("  Service  %d\n", i)
        }
    }
    s += fmt.Sprintf("\n")

    // Sensors
    s += fmt.Sprintf("Measurement Counts:\n")
    if ds.BatWarningCount == 0 {
        s += fmt.Sprintf("  Bat %d\n", ds.BatCount)
    } else {
        s += fmt.Sprintf("  Bat %d (%d out of range %s)\n", ds.BatCount, ds.BatWarningCount, ds.BatWarningFirst.UTC().Format("2006-01-02T15:04:05Z"))
    }
    if ds.EnvWarningCount == 0 {
        s += fmt.Sprintf("  Env %d\n", ds.EnvCount)
    } else {
        s += fmt.Sprintf("  Env %d (%d out of range %s)\n", ds.EnvCount, ds.EnvWarningCount, ds.EnvWarningFirst.UTC().Format("2006-01-02T15:04:05Z"))
    }
    if ds.EncWarningCount == 0 {
        s += fmt.Sprintf("  Enc %d\n", ds.EncCount)
    } else {
        s += fmt.Sprintf("  Enc %d (%d out of range %s)\n", ds.EncCount, ds.EncWarningCount, ds.EncWarningFirst.UTC().Format("2006-01-02T15:04:05Z"))
    }
    if ds.PmsWarningCount == 0 {
        s += fmt.Sprintf("  Pms %d\n", ds.PmsCount)
    } else {
        s += fmt.Sprintf("  Pms %d (%d out of range %s)\n", ds.PmsCount, ds.PmsWarningCount, ds.PmsWarningFirst.UTC().Format("2006-01-02T15:04:05Z"))
    }
    if ds.OpcWarningCount == 0 {
        s += fmt.Sprintf("  Opc %d\n", ds.OpcCount)
    } else {
        s += fmt.Sprintf("  Opc %d (%d out of range %s)\n", ds.OpcCount, ds.OpcWarningCount, ds.OpcWarningFirst.UTC().Format("2006-01-02T15:04:05Z"))
    }
    geigerConfig := ""
    if ds.LndU7318Count == 0 && ds.LndC7318Count == 0 && ds.LndEC7128Count == 0 {
        geigerConfig = "(no tubes configured)"
        s += fmt.Sprintf("  Lnd 0")
    } else if ds.LndU7318Count != 0 && ds.LndC7318Count == 0 && ds.LndEC7128Count == 0 {
        geigerConfig = "(SINGLE pancake configuration)"
        s += fmt.Sprintf("  Lnd %d %s", ds.LndU7318Count, geigerConfig)
    } else if ds.LndU7318Count != 0 && ds.LndC7318Count != 0 && ds.LndEC7128Count == 0 {
        s += fmt.Sprintf("  Lnd %d|%d", ds.LndU7318Count, ds.LndC7318Count)
    } else if ds.LndU7318Count != 0 && ds.LndC7318Count == 0 && ds.LndEC7128Count != 0 {
        geigerConfig = "(dual-tube EC configuration)"
        s += fmt.Sprintf("  Lnd %d|%d %s", ds.LndU7318Count, ds.LndEC7128Count, geigerConfig)
    } else {
        geigerConfig = "(UNRECOGNIZED configuration)"
        s += fmt.Sprintf("  Lnd %du|%dc|%dec %s", ds.LndU7318Count, ds.LndC7318Count, ds.LndEC7128Count, geigerConfig)
    }
    if ds.GeigerWarningCount != 0 {
        s += fmt.Sprintf(" (%d out of range %s)", ds.GeigerWarningCount, ds.GeigerWarningFirst.UTC().Format("2006-01-02T15:04:05Z"))
    }
    s += fmt.Sprintf("\n")
    s += fmt.Sprintf("\n")

    // Errors
    if ds.Boots == 1 {
        s += fmt.Sprintf("Device errors:\n")
    } else {
        s += fmt.Sprintf("Device errors across %d sessions:\n", ds.Boots)
    }
    if !ds.AnyErrors {
        s += fmt.Sprintf("  None\n")
    } else {
        i := ds.PrevErrorsOpc + ds.ThisErrorsOpc
        if i > 0 {
            s += fmt.Sprintf("  Opc        %d\n", i)
        }
        i = ds.PrevErrorsPms + ds.ThisErrorsPms
        if i > 0 {
            s += fmt.Sprintf("  Pms        %d\n", i)
        }
        i = ds.PrevErrorsBme0 + ds.ThisErrorsBme0
        if i > 0 {
            s += fmt.Sprintf("  Bme0       %d\n", i)
        }
        i = ds.PrevErrorsBme1 + ds.ThisErrorsBme1
        if i > 0 {
            s += fmt.Sprintf("  Bme1       %d\n", i)
        }
        i = ds.PrevErrorsLora + ds.ThisErrorsLora
        if i > 0 {
            s += fmt.Sprintf("  Lora       %d\n", i)
        }
        i = ds.PrevErrorsFona + ds.ThisErrorsFona
        if i > 0 {
            s += fmt.Sprintf("  Fona       %d\n", i)
        }
        i = ds.PrevErrorsCommsPowerFails + ds.ThisErrorsCommsPowerFails
        if i > 0 {
            s += fmt.Sprintf("  Fona Power %d\n", i)
        }
        i = ds.PrevErrorsGeiger + ds.ThisErrorsGeiger
        if i > 0 {
            s += fmt.Sprintf("  Geiger     %d\n", i)
        }
        i = ds.PrevErrorsMax01 + ds.ThisErrorsMax01
        if i > 0 {
            s += fmt.Sprintf("  Max01      %d\n", i)
        }
        i = ds.PrevErrorsUgps + ds.ThisErrorsUgps
        if i > 0 {
            s += fmt.Sprintf("  Ugps       %d\n", i)
        }
        i = ds.PrevErrorsLis + ds.ThisErrorsLis
        if i > 0 {
            s += fmt.Sprintf("  Lis        %d\n", i)
        }
        i = ds.PrevErrorsSpi + ds.ThisErrorsSpi
        if i > 0 {
            s += fmt.Sprintf("  Spi        %d\n", i)
        }
        i = ds.PrevErrorsTwi + ds.ThisErrorsTwi
        if i > 0 || ds.ErrorsTwiInfo != "" {
            s += fmt.Sprintf("  Twi        %d %s\n", i, ds.ErrorsTwiInfo)
        }
    }
    s += fmt.Sprintf("\n")

    // Pointcast
    if ds.AnyPointcastErrors {
        s += fmt.Sprintf("Pointcast errors new since %s:\n", ds.OldestUpload.Format("2006-01-02 15:04 UTC"))
        i := ds.PrevErrorsCommsFailures + ds.ThisErrorsCommsFailures
        j := i - ds.MinErrorsCommsFailures
        s += fmt.Sprintf("  CommsFailures   %d new / %d total\n", j, i)
        i = ds.PrevErrorsDeviceRestarts + ds.ThisErrorsDeviceRestarts
        j = i - ds.MinErrorsDeviceRestarts;
        s += fmt.Sprintf("  DeviceRestarts  %d new / %d total\n", j, i)
        s += fmt.Sprintf("\n")
    }

    // That's all if we're not solarcast
    if ds.Transports == "pointcast" || ds.Transports == "safecast-air" {
        return s
    }

    // Solarcast summary
    s += fmt.Sprintf("Solarcast Checklist:\n")

    goalHours := 72
    if ds.MaxUptimeMinutes > uint32(goalHours) * 60 {
        s += fmt.Sprintf("  PASS  ")
    } else {
        s += fmt.Sprintf("   --   ");
    }
    s += fmt.Sprintf("At least one continuous measurable session of >%d hours.\n", goalHours);

    if !ds.MultiFirmware {
        s += fmt.Sprintf("  PASS  ")
    } else {
        s += fmt.Sprintf("   --   ");
    }
    s += fmt.Sprintf("One version of firmware used for the entire run.\n");

    if !ds.AnyErrors {
        s += fmt.Sprintf("  PASS  ")
    } else {
        s += fmt.Sprintf("   --   ");
    }
    s += fmt.Sprintf("No device errors.\n");

    if !ds.AnyConnectErrors {
        s += fmt.Sprintf("  PASS  ")
    } else {
        s += fmt.Sprintf("   --   ");
    }
    s += fmt.Sprintf("No connection errors.\n");

    diff := math.Abs(float64(ds.LoraTransports) - float64(ds.FonaTransports))
    pct := diff / float64(ds.Measurements)
    goal := 0.25
    if pct <= goal {
        s += fmt.Sprintf("  PASS  ")
    } else {
        s += fmt.Sprintf("   --   ");
    }
    s += fmt.Sprintf("Less than %.0f%% variation between transports. (%.0f%% actual)\n", goal*100, pct*100)

    if ds.GapsGt10m == 0 {
        s += fmt.Sprintf("  PASS  ")
    } else {
        s += fmt.Sprintf("   --   ");
    }
    s += fmt.Sprintf("No communications gaps of more than 10m.\n")

    if geigerConfig == "" {
        s += fmt.Sprintf("  PASS  ")
    } else {
        s += fmt.Sprintf("   --   ");
    }
    s += fmt.Sprintf("Both pancake tubes measured data. %s\n", geigerConfig);

    if ds.BatCount != 0 && ds.EnvCount != 0 && ds.EncCount != 0 && ds.PmsCount != 0 && ds.OpcCount != 0 && geigerConfig == "" {
        s += fmt.Sprintf("  PASS  ")
    } else {
        s += fmt.Sprintf("   --   ");
    }
    s += fmt.Sprintf("All sensors measured data.\n");

    if ds.BatWarningCount == 0 && ds.EnvWarningCount == 0 && ds.EncWarningCount == 0 && ds.PmsWarningCount == 0 && ds.OpcWarningCount == 0 && ds.GeigerWarningCount == 0 {
        s += fmt.Sprintf("  PASS  ")
    } else {
        s += fmt.Sprintf("   --   ");
    }
    s += fmt.Sprintf("All measured data was within valid ranges.\n");

    s += fmt.Sprintf("\n")

    // Done
    return s
}
