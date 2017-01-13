// Safecast inbound message handling and publishing
package main

import (
    "os"
    "os/user"
    "net/http"
    "fmt"
    "bytes"
    "sort"
    "time"
    "strings"
    "strconv"
    "encoding/json"
    "github.com/rayozzie/teletype-proto/golang"
)

// For dealing with transaction timeouts
var httpTransactionsInProgress int = 0
var httpTransactions = 0
const httpTransactionsRecorded = 250
var httpTransactionDurations[httpTransactionsRecorded] int
var httpTransactionTimes[httpTransactionsRecorded] time.Time
var httpTransactionErrorTime string
var httpTransactionErrorString string
var httpTransactionErrors = 0
var httpTransactionErrorFirst bool = true

// Describes every device that has sent us a message
type seenDevice struct {
    originalDeviceNo    uint32
    normalizedDeviceNo  uint32
    dualSeen            bool
    seen                time.Time
    notifiedAsUnseen    bool
    minutesAgo          int64
}
var seenDevices []seenDevice

// Checksums of recently-processed messages
type receivedMessage struct {
    checksum            uint32
    seen                time.Time
}
var recentlyReceived [25]receivedMessage

// Class used to sort seen devices
type ByKey []seenDevice
func (a ByKey) Len() int      { return len(a) }
func (a ByKey) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool {
    // Primary:
    // By capture time, most recent last (so that the most recent is nearest your attention, at the bottom in Slack)
    if a[i].seen.Before(a[j].seen) {
        return true
    } else if a[i].seen.After(a[j].seen) {
        return false
    }
    // Secondary
    // In an attempt to keep things reasonably deterministic, use device number
    if a[i].normalizedDeviceNo < a[j].normalizedDeviceNo {
        return true
    } else if a[i].normalizedDeviceNo > a[j].normalizedDeviceNo {
        return false
    }
    return false
}

// Process an inbound Safecast message
func ProcessSafecastMessage(msg *teletype.Telecast,
    checksum uint32,
    ipInfo string,
    Transport string,
    defaultTime string,
    defaultSNR float32,
    defaultLat float32, defaultLon float32, defaultAlt int32) {
    var theSNR float32

    // Discard it if it's a duplicate
    if isDuplicate(checksum) {
        fmt.Printf("%s DISCARDING duplicate message\n", time.Now().Format(logDateFormat));
        return
    }

    // Process IPINFO data
    var info IPInfoData
    if ipInfo != "" {
        err := json.Unmarshal([]byte(ipInfo), &info)
        if err != nil {
            ipInfo = ""
        }
    }
    if ipInfo != "" {
        fmt.Printf("%s from %s/%s/%s\n", time.Now().Format(logDateFormat), info.City, info.Region, info.Country)
    }

    // Log it
    trackDevice(TelecastDeviceID(msg))

    // Generate the fields common to all uploads to safecast
    scV1 := SafecastDataV1{}
    scV2 := SafecastDataV2{}
    if msg.DeviceIDString != nil {
        scV1.DeviceID = msg.GetDeviceIDString()
		// We do not support non-numeric device ID in the V2 format
        scV2.DeviceID = 0
    } else if msg.DeviceIDNumber != nil {
        scV1.DeviceID = strconv.FormatUint(uint64(msg.GetDeviceIDNumber()), 10)
        scV2.DeviceID = msg.GetDeviceIDNumber()
    } else {
        scV1.DeviceID = "UNKNOWN"
        scV2.DeviceID = 0
    }
    if msg.CapturedAt != nil {
        scV1.CapturedAt = msg.GetCapturedAt()
        scV2.CapturedAt = msg.GetCapturedAt()
    } else {
        scV1.CapturedAt = defaultTime
        scV2.CapturedAt = defaultTime
    }

    // Include lat/lon/alt on all messages, including metadata
    if msg.Latitude != nil {
        scV1.Latitude = fmt.Sprintf("%f", msg.GetLatitude())
        scV2.Latitude =  msg.GetLatitude()
    } else {
        if defaultLat != 0.0 {
            scV1.Latitude = fmt.Sprintf("%f", defaultLat)
            scV2.Latitude = defaultLat
        }
    }
    if msg.Longitude != nil {
        scV1.Longitude = fmt.Sprintf("%f", msg.GetLongitude())
        scV2.Longitude = msg.GetLongitude()
    } else {
        if defaultLon != 0.0 {
            scV1.Longitude = fmt.Sprintf("%f", defaultLon)
            scV2.Longitude = defaultLon
        }
    }
    if msg.Altitude != nil {
        scV1.Height = fmt.Sprintf("%d", msg.GetAltitude())
        scV2.Height = float32(msg.GetAltitude())
    } else {
        if defaultAlt != 0.0 {
            scV1.Height = fmt.Sprintf("%d", defaultAlt)
            scV2.Height = float32(defaultAlt)
        }
    }

    // The first/primary upload has all known fields.  It is
    // our goal that someday this is the *only* upload,
    // after the Safecast service is upgraded.
    scV1a := scV1
    scV2a := scV2

    // Process the most basic message types
    if msg.StatsUptimeMinutes != nil {

        // A stats message
        scV1a.Unit = UnitStats
        scV1a.Value = fmt.Sprintf("%d", msg.GetStatsUptimeMinutes())

        var scStats safecastStatsV1
        scStats.StatsUptimeMinutes = msg.GetStatsUptimeMinutes()
        scV2a.StatsUptimeMinutes = msg.GetStatsUptimeMinutes()
        if (msg.StatsAppVersion != nil) {
            scStats.StatsAppVersion = msg.GetStatsAppVersion()
            scV2a.StatsAppVersion = msg.GetStatsAppVersion()
        }
        if (msg.StatsDeviceParams != nil) {
            scStats.StatsDeviceParams = msg.GetStatsDeviceParams()
            scV2a.StatsDeviceParams = msg.GetStatsDeviceParams()
        }
        if (msg.StatsTransmittedBytes != nil) {
            scStats.StatsTransmittedBytes = msg.GetStatsTransmittedBytes()
            scV2a.StatsTransmittedBytes = msg.GetStatsTransmittedBytes()
        }
        if (msg.StatsReceivedBytes != nil) {
            scStats.StatsReceivedBytes = msg.GetStatsReceivedBytes()
            scV2a.StatsReceivedBytes = msg.GetStatsReceivedBytes()
        }
        if (msg.StatsCommsResets != nil) {
            scStats.StatsCommsResets = msg.GetStatsCommsResets()
            scV2a.StatsCommsResets = msg.GetStatsCommsResets()
        }
        if (msg.StatsCommsPowerFails != nil) {
            scStats.StatsCommsPowerFails = msg.GetStatsCommsPowerFails()
            scV2a.StatsCommsPowerFails = msg.GetStatsCommsPowerFails()
        }
        if (msg.StatsOneshots != nil) {
            scStats.StatsOneshots = msg.GetStatsOneshots()
            scV2a.StatsOneshots = msg.GetStatsOneshots()
        }
        if (msg.StatsMotiondrops != nil) {
            scStats.StatsMotiondrops = msg.GetStatsMotiondrops()
            scV2a.StatsMotiondrops = msg.GetStatsMotiondrops()
        }
        if (msg.StatsCell != nil) {
            scStats.StatsCell = msg.GetStatsCell()
            scV2a.StatsCell = msg.GetStatsCell()
        }
        if (msg.StatsDfu != nil) {
            scStats.StatsDfu = msg.GetStatsDfu()
            scV2a.StatsDfu = msg.GetStatsDfu()
        }

        scsJSON, _ := json.Marshal(scStats)
        scV1a.DeviceTypeID = string(scsJSON)

    } else if msg.Message != nil {

        // A text message.  Since the Value in safecast
        // must be a number, we use a different text field instead.
        scV1a.Unit = UnitMessage
        scV1a.DeviceTypeID = msg.GetMessage()
        scV2a.Message = msg.GetMessage()

    } else {

        // An old-style safecast geiger upload.  If it's lacking
        // a value, don't add a unit.  This means that
        // it was a metadata-only upload, or a new style upload
        // which only has cpm0/cpm1 fields.
        if msg.Value != nil {
            if msg.Unit == nil {
                scV1a.Unit = UnitCPM
            } else {
                scV1a.Unit = fmt.Sprintf("%s", msg.GetUnit())
            }
            scV1a.Value = fmt.Sprintf("%d", msg.GetValue())
        }

    }

    if msg.BatteryVoltage != nil {
        scV1a.BatVoltage = fmt.Sprintf("%.4f", msg.GetBatteryVoltage())
        scV2a.BatVoltage = msg.GetBatteryVoltage()
    }
    if msg.BatterySOC != nil {
        scV1a.BatSOC = fmt.Sprintf("%.2f", msg.GetBatterySOC())
        scV2a.BatSOC = msg.GetBatterySOC()
    }

    if msg.BatteryCurrent != nil {
        scV1a.BatCurrent = fmt.Sprintf("%.3f", msg.GetBatteryCurrent())
        scV2a.BatCurrent = msg.GetBatteryCurrent()
    }

    if msg.EnvTemperature != nil {
        scV1a.EnvTemp = fmt.Sprintf("%.2f", msg.GetEnvTemperature())
        scV2a.EnvTemp = msg.GetEnvTemperature()
    }
    if msg.EnvHumidity != nil {
        scV1a.EnvHumid = fmt.Sprintf("%.2f", msg.GetEnvHumidity())
        scV2a.EnvHumid = msg.GetEnvHumidity()
    }
    if msg.EnvPressure != nil {
        scV1a.EnvPress = fmt.Sprintf("%.2f", msg.GetEnvPressure())
        scV2a.EnvPress = msg.GetEnvPressure()
    }

    scV1a.Transport = Transport
    scV2a.Transport = Transport

    if msg.WirelessSNR != nil {
        theSNR = msg.GetWirelessSNR()
    } else {
        theSNR = defaultSNR
    }
    if defaultSNR != 0.0 {
        scV1a.WirelessSNR = fmt.Sprintf("%.1f", theSNR)
        scV2a.WirelessSNR = theSNR
    }

    if msg.PmsPm01_0 != nil {
        scV1a.PmsPm01_0 = fmt.Sprintf("%d", msg.GetPmsPm01_0())
        scV2a.PmsPm01_0 = float32(msg.GetPmsPm01_0())
    }
    if msg.PmsPm02_5 != nil {
        scV1a.PmsPm02_5 = fmt.Sprintf("%d", msg.GetPmsPm02_5())
        scV2a.PmsPm02_5 = float32(msg.GetPmsPm02_5())
    }
    if msg.PmsPm10_0 != nil {
        scV1a.PmsPm10_0 = fmt.Sprintf("%d", msg.GetPmsPm10_0())
        scV2a.PmsPm10_0 = float32(msg.GetPmsPm10_0())
    }
    if msg.PmsC00_30 != nil {
        scV1a.PmsC00_30 = fmt.Sprintf("%d", msg.GetPmsC00_30())
        scV2a.PmsC00_30 = msg.GetPmsC00_30()
    }
    if msg.PmsC00_50 != nil {
        scV1a.PmsC00_50 = fmt.Sprintf("%d", msg.GetPmsC00_50())
        scV2a.PmsC00_50 = msg.GetPmsC00_50()
    }
    if msg.PmsC01_00 != nil {
        scV1a.PmsC01_00 = fmt.Sprintf("%d", msg.GetPmsC01_00())
        scV2a.PmsC01_00 = msg.GetPmsC01_00()
    }
    if msg.PmsC02_50 != nil {
        scV1a.PmsC02_50 = fmt.Sprintf("%d", msg.GetPmsC02_50())
        scV2a.PmsC02_50 = msg.GetPmsC02_50()
    }
    if msg.PmsC05_00 != nil {
        scV1a.PmsC05_00 = fmt.Sprintf("%d", msg.GetPmsC05_00())
        scV2a.PmsC05_00 = msg.GetPmsC05_00()
    }
    if msg.PmsC10_00 != nil {
        scV1a.PmsC10_00 = fmt.Sprintf("%d", msg.GetPmsC10_00())
        scV2a.PmsC10_00 = msg.GetPmsC10_00()
    }
    if msg.PmsCsecs != nil {
        scV1a.PmsCsecs = fmt.Sprintf("%d", msg.GetPmsCsecs())
        scV2a.PmsCsecs = msg.GetPmsCsecs()
    }

    if msg.OpcPm01_0 != nil {
        scV1a.OpcPm01_0 = fmt.Sprintf("%f", msg.GetOpcPm01_0())
        scV2a.OpcPm01_0 = msg.GetOpcPm01_0()
    }
    if msg.OpcPm02_5 != nil {
        scV1a.OpcPm02_5 = fmt.Sprintf("%f", msg.GetOpcPm02_5())
        scV2a.OpcPm02_5 = msg.GetOpcPm02_5()
    }
    if msg.OpcPm10_0 != nil {
        scV1a.OpcPm10_0 = fmt.Sprintf("%f", msg.GetOpcPm10_0())
        scV2a.OpcPm10_0 = msg.GetOpcPm10_0()
    }
    if msg.OpcC00_38 != nil {
        scV1a.OpcC00_38 = fmt.Sprintf("%d", msg.GetOpcC00_38())
        scV2a.OpcC00_38 = msg.GetOpcC00_38()
    }
    if msg.OpcC00_54 != nil {
        scV1a.OpcC00_54 = fmt.Sprintf("%d", msg.GetOpcC00_54())
        scV2a.OpcC00_54 = msg.GetOpcC00_54()
    }
    if msg.OpcC01_00 != nil {
        scV1a.OpcC01_00 = fmt.Sprintf("%d", msg.GetOpcC01_00())
        scV2a.OpcC01_00 = msg.GetOpcC01_00()
    }
    if msg.OpcC02_10 != nil {
        scV1a.OpcC02_10 = fmt.Sprintf("%d", msg.GetOpcC02_10())
        scV2a.OpcC02_10 = msg.GetOpcC02_10()
    }
    if msg.OpcC05_00 != nil {
        scV1a.OpcC05_00 = fmt.Sprintf("%d", msg.GetOpcC05_00())
        scV2a.OpcC05_00 = msg.GetOpcC05_00()
    }
    if msg.OpcC10_00 != nil {
        scV1a.OpcC10_00 = fmt.Sprintf("%d", msg.GetOpcC10_00())
        scV2a.OpcC10_00 = msg.GetOpcC10_00()
    }
    if msg.OpcCsecs != nil {
        scV1a.OpcCsecs = fmt.Sprintf("%d", msg.GetOpcCsecs())
        scV2a.OpcCsecs = msg.GetOpcCsecs()
    }

    // Upload differently based on how CPM is represented
    if msg.Cpm0 == nil && msg.Cpm1 == nil {

        // Either an old-style upload, and the kind used by bGeigies,
        // or an upload of metadata without any kind of CPM
        if (!SafecastV1Upload(scV1a, SafecastV1QueryString)) {
            return
        }

        // Write a new-style entry to the log
        scV1b := scV1a
        scV2b := scV2a
        if msg.StatsUptimeMinutes == nil {
            did := uint64(msg.GetDeviceIDNumber())
            if ((did & 0x01) == 0) {
                scV1b.Cpm0 = scV1a.Value
				scV2b.Cpm0 = float32(msg.GetValue())
            } else {
                scV1b.DeviceID = strconv.FormatUint(did & 0xfffffffe, 10)
                scV1b.Cpm1 = scV1a.Value
                scV2b.DeviceID = uint32(did & 0xfffffffe)
				scV2b.Cpm1 = float32(msg.GetValue())
            }
            scV1b.Unit = ""
            scV1b.Value = ""
        }
        writeToLogs(scV1b, scV2b)

    } else if msg.DeviceIDNumber != nil {
		var uploaded = 0
		
        // A new style upload has "cpm0" or "cpm1" values, and
        // must have a numeric device ID
        if msg.Cpm0 != nil {
            scV1b := scV1a
            scV1b.DeviceID = strconv.FormatUint(uint64(msg.GetDeviceIDNumber() & 0xfffffffe), 10)
            scV1b.Unit = UnitCPM
            scV1b.Value = fmt.Sprintf("%d", msg.GetCpm0())
            if (SafecastV1Upload(scV1b, SafecastV1QueryString)) {
				uploaded = uploaded + 1
			}
        }
        if msg.Cpm1 != nil {
            scV1b := scV1
            scV1b.DeviceID = strconv.FormatUint(uint64(msg.GetDeviceIDNumber() | 0x00000001), 10)
            scV1b.Unit = UnitCPM
            scV1b.Value = fmt.Sprintf("%d", msg.GetCpm1())
            if (SafecastV1Upload(scV1b, SafecastV1QueryString)) {
				uploaded = uploaded + 1
			}
        }

        scV1c := scV1a
		scV2c := scV2a
		if (msg.Cpm0 != nil) {
            scV1c.Cpm0 = fmt.Sprintf("%d", msg.GetCpm0())
			scV2c.Cpm0 = float32(msg.GetCpm0())
		}
		if (msg.Cpm1 != nil) {
            scV1c.Cpm1 = fmt.Sprintf("%d", msg.GetCpm1())
			scV2c.Cpm1 = float32(msg.GetCpm1())
		}
        if (SafecastV2Upload(scV2c, SafecastV2QueryString)) {
			uploaded = uploaded + 1
		}

		// Log it
        writeToLogs(scV1c, scV2c)

		// Exit if not uploaded
		if (uploaded == 0) {
			return
		}

    }

    // Due to Safecast API design limitations, upload the metadata as
    // discrete web uploads.  Once this API limitation is removed,
    // this code should be deleted.
    if msg.BatteryVoltage != nil {
        scV1b := scV1
        scV1b.Unit = UnitBatVoltage
        scV1b.Value = scV1a.BatVoltage
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.BatterySOC != nil {
        scV1b := scV1
        scV1b.Unit = UnitBatSOC
        scV1b.Value = scV1a.BatSOC
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.BatteryCurrent != nil {
        scV1b := scV1
        scV1b.Unit = UnitBatCurrent
        scV1b.Value = scV1a.BatCurrent
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.EnvTemperature != nil {
        scV1b := scV1
        scV1b.Unit = UnitEnvTemp
        scV1b.Value = scV1a.EnvTemp
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.EnvHumidity != nil {
        scV1b := scV1
        scV1b.Unit = UnitEnvHumid
        scV1b.Value = scV1a.EnvHumid
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.EnvPressure != nil {
        scV1b := scV1
        scV1b.Unit = UnitEnvPress
        scV1b.Value = scV1a.EnvPress
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }

    // Only bother uploading certain values if they coincides with another
    // really low-occurrance feature, because the device just doesn't
    // move that much and so its SNR should remain reasonably constant
    // except for rain.
    if msg.BatteryVoltage != nil {

        if theSNR != 0.0  {
	        scV1b := scV1
            scV1b.Unit = UnitWirelessSNR
            scV1b.Value = scV1a.WirelessSNR
            if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
                return
            }
        }

        scV1b := scV1
        scV1b.Unit = UnitTransport
        scV1b.Value = scV1a.Transport
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }

    }

    if msg.PmsPm01_0 != nil {
        scV1b := scV1
        scV1b.Unit = UnitPmsPm01_0
        scV1b.Value = scV1a.PmsPm01_0
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.PmsPm02_5 != nil {
        scV1b := scV1
        scV1b.Unit = UnitPmsPm02_5
        scV1b.Value = scV1a.PmsPm02_5
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.PmsPm10_0 != nil {
        scV1b := scV1
        scV1b.Unit = UnitPmsPm10_0
        scV1b.Value = scV1a.PmsPm10_0
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.PmsC00_30 != nil {
        scV1b := scV1
        scV1b.Unit = UnitPmsC00_30
        scV1b.Value = scV1a.PmsC00_30
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.PmsC00_50 != nil {
        scV1b := scV1
        scV1b.Unit = UnitPmsC00_50
        scV1b.Value = scV1a.PmsC00_50
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.PmsC01_00 != nil {
        scV1b := scV1
        scV1b.Unit = UnitPmsC01_00
        scV1b.Value = scV1a.PmsC01_00
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.PmsC02_50 != nil {
        scV1b := scV1
        scV1b.Unit = UnitPmsC02_50
        scV1b.Value = scV1a.PmsC02_50
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.PmsC05_00 != nil {
        scV1b := scV1
        scV1b.Unit = UnitPmsC05_00
        scV1b.Value = scV1a.PmsC05_00
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.PmsC10_00 != nil {
        scV1b := scV1
        scV1b.Unit = UnitPmsC10_00
        scV1b.Value = scV1a.PmsC10_00
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.PmsCsecs != nil {
        scV1b := scV1
        scV1b.Unit = UnitPmsCsecs
        scV1b.Value = scV1a.PmsCsecs
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }

    if msg.OpcPm01_0 != nil {
        scV1b := scV1
        scV1b.Unit = UnitOpcPm01_0
        scV1b.Value = scV1a.OpcPm01_0
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.OpcPm02_5 != nil {
        scV1b := scV1
        scV1b.Unit = UnitOpcPm02_5
        scV1b.Value = scV1a.OpcPm02_5
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.OpcPm10_0 != nil {
        scV1b := scV1
        scV1b.Unit = UnitOpcPm10_0
        scV1b.Value = scV1a.OpcPm10_0
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.OpcC00_38 != nil {
        scV1b := scV1
        scV1b.Unit = UnitOpcC00_38
        scV1b.Value = scV1a.OpcC00_38
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.OpcC00_54 != nil {
        scV1b := scV1
        scV1b.Unit = UnitOpcC00_54
        scV1b.Value = scV1a.OpcC00_54
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.OpcC01_00 != nil {
        scV1b := scV1
        scV1b.Unit = UnitOpcC01_00
        scV1b.Value = scV1a.OpcC01_00
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.OpcC02_10 != nil {
        scV1b := scV1
        scV1b.Unit = UnitOpcC02_10
        scV1b.Value = scV1a.OpcC02_10
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.OpcC05_00 != nil {
        scV1b := scV1
        scV1b.Unit = UnitOpcC05_00
        scV1b.Value = scV1a.OpcC05_00
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.OpcC10_00 != nil {
        scV1b := scV1
        scV1b.Unit = UnitOpcC10_00
        scV1b.Value = scV1a.OpcC10_00
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }
    if msg.OpcCsecs != nil {
        scV1b := scV1
        scV1b.Unit = UnitOpcCsecs
        scV1b.Value = scV1a.OpcCsecs
        if (!SafecastV1Upload(scV1b, SafecastV1QueryString)) {
            return
        }
    }

}

// Begin transaction and return the transaction ID
func beginTransaction(url string, message1 string, message2 string) int {
    httpTransactionsInProgress += 1
    httpTransactions += 1
    transaction := httpTransactions % httpTransactionsRecorded
    httpTransactionTimes[transaction] = time.Now()
    fmt.Printf("%s >>> [%d] %s %s\n", time.Now().Format(logDateFormat), transaction, message1, message2)
    return transaction
}

// End transaction and issue warnings
func endTransaction(transaction int, errstr string) {
    httpTransactionsInProgress -= 1
    duration := int(time.Now().Sub(httpTransactionTimes[transaction]) / time.Second)
    httpTransactionDurations[transaction] = duration

    if errstr != "" {
        httpTransactionErrors = httpTransactionErrors + 1
        if (httpTransactionErrorFirst) {
            httpTransactionErrorTime = time.Now().Format(logDateFormat)
            httpTransactionErrorString = errstr
            httpTransactionErrorFirst = false
        }
        fmt.Printf("%s <<< [%d] *** after %d seconds, ERROR uploading to Safecast %s\n\n", time.Now().Format(logDateFormat), transaction, duration, errstr)
    } else {
        if (duration < 5) {
            fmt.Printf("%s <<< [%d]\n", time.Now().Format(logDateFormat), transaction);
        } else {
            fmt.Printf("%s <<< [%d] completed after %d seconds\n", time.Now().Format(logDateFormat), transaction, duration);
        }
    }

    theMin := 99999
    theMax := 0
    theTotal := 0
    theCount := 0
    for theCount < httpTransactions && theCount < httpTransactionsRecorded {
        theTotal += httpTransactionDurations[theCount]
        if httpTransactionDurations[theCount] < theMin {
            theMin = httpTransactionDurations[theCount]
        }
        if httpTransactionDurations[theCount] > theMax {
            theMax = httpTransactionDurations[theCount]
        }
        theCount += 1
    }
    theMean := theTotal / theCount

    // Output to console every time we are in a "slow mode"
    if (theMin > 5) {
        fmt.Printf("%s Safecast HTTP Upload Statistics\n", time.Now().Format(logDateFormat))
        fmt.Printf("%s *** %d total uploads since restart\n", time.Now().Format(logDateFormat), httpTransactions)
        if (httpTransactionsInProgress > 0) {
            fmt.Printf("%s *** %d uploads still in progress\n", time.Now().Format(logDateFormat), httpTransactionsInProgress)
        }
        fmt.Printf("%s *** Last %d: min=%ds, max=%ds, avg=%ds\n", time.Now().Format(logDateFormat), theCount, theMin, theMax, theMean)

    }

    // If there's a problem, output to Slack once every 25 transactions
    if (theMin > 5 && transaction == 0) {
        // If all of them have the same timeout value, the server must be down.
        s := ""
        if (theMin == theMax && theMin == theMean) {
            s = fmt.Sprintf("Safecast API: all of the most recent %d uploads failed. Please check the service.", theCount)
        } else {
            s = fmt.Sprintf("Safecast API: of the previous %d uploads, min=%ds, max=%ds, avg=%ds", theCount, theMin, theMax, theMean)
        }
        sendToSafecastApi(s);
    }

}

// Upload a Safecast data structure to the Safecast service, either serially or massively in parallel
func SafecastV2Upload(scV2 SafecastDataV2, query string) bool {
    go doUploadToSafecastV2(scV2, query)
	return true
}

// Upload a Safecast data structure to the Safecast service
func doUploadToSafecastV2(scV2 SafecastDataV2, query string) bool {

	// while waiting to know where to post
	if (true) {
		return false
	}
	
    transaction := beginTransaction(SafecastV2UploadURL, "(aggregate)", "")

    scJSON, _ := json.Marshal(scV2)

    if false {
        fmt.Printf("%s\n", scJSON)
    }

	url := SafecastV2UploadURL
	if (query != "") {
		url = fmt.Sprintf("%s?%s", SafecastV2UploadURL, query)
	}
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(scJSON))
    req.Header.Set("User-Agent", "TTSERVE")
    req.Header.Set("Content-Type", "application/json")
    httpclient := &http.Client{
        Timeout: time.Second * 15,
    }
    resp, err := httpclient.Do(req)

    errString := ""
    if (err == nil) {
        resp.Body.Close()
    } else {
        // Eliminate the URL from the string because exposing the API key is not secure.
        // Empirically we've seen that the actual error message is after the rightmost colon
        errString = fmt.Sprintf("%s", err)
        s := strings.Split(errString, ":")
        errString = s[len(s)-1]
    }

    endTransaction(transaction, errString)

    return errString == ""
}

// Upload a Safecast data structure to the Safecast service, either serially or massively in parallel
func SafecastV1Upload(scV1 SafecastDataV1, query string) bool {

    // We've found that in certain cases the server gets overloaded.  When we run into those cases,
    // turn this OFF and things will slow down.  (Obviously this is not the preferred mode of operation,
    // because it creates a huge queue of things waiting to be uploaded.)
    uploadInParallel := false;

    if (uploadInParallel) {
        go doUploadToSafecastV1(scV1, query)
    } else {
        if (!doUploadToSafecastV1(scV1, query)) {
            return false
        }
        time.Sleep(1 * time.Second)
    }

    return true

}

// Upload a Safecast data structure to the Safecast service
func doUploadToSafecastV1(scV1 SafecastDataV1, query string) bool {

    transaction := beginTransaction(SafecastV1UploadURL, scV1.Unit, scV1.Value)

    scJSON, _ := json.Marshal(scV1)

    if false {
        fmt.Printf("%s\n", scJSON)
    }

	url := SafecastV1UploadURL
	if (query != "") {
		url = fmt.Sprintf("%s?%s", SafecastV1UploadURL, query)
	}
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(scJSON))
    req.Header.Set("User-Agent", "TTSERVE")
    req.Header.Set("Content-Type", "application/json")
    httpclient := &http.Client{
        Timeout: time.Second * 15,
    }
    resp, err := httpclient.Do(req)

    errString := ""
    if (err == nil) {
        resp.Body.Close()
    } else {
        // Eliminate the URL from the string because exposing the API key is not secure.
        // Empirically we've seen that the actual error message is after the rightmost colon
        errString = fmt.Sprintf("%s", err)
        s := strings.Split(errString, ":")
        errString = s[len(s)-1]
    }

    endTransaction(transaction, errString)

    return errString == ""
}

// Check to see if this is a duplicate of a message we've recently seen
func isDuplicate(checksum uint32) bool {

    // Sweep through all recent messages, looking for a duplicate in the past minute
    for i := 0; i < len(recentlyReceived); i++ {
        if recentlyReceived[i].checksum == checksum {
            if (int64(time.Now().Sub(recentlyReceived[i].seen) / time.Second) < 60) {
                return true
            }
        }
    }

    // Shift them all down
    for i := len(recentlyReceived)-1; i > 0; i-- {
        recentlyReceived[i] = recentlyReceived[i-1]
    }

    // Insert this new one
    recentlyReceived[0].checksum = checksum;
    recentlyReceived[0].seen = time.Now().UTC()
    return false

}

// Keep track of all devices that have sent us a message
func trackDevice(DeviceID uint32) {
    var dev seenDevice

    // For dual-sensor devices, collapse them to a single entry
    dev.originalDeviceNo = DeviceID
    dev.normalizedDeviceNo = dev.originalDeviceNo
    if (dev.normalizedDeviceNo & 0x01) != 0 {
        dev.normalizedDeviceNo = dev.normalizedDeviceNo - 1
    }

    // Attempt to update the existing entry if we can find it
    found := false
    for i := 0; i < len(seenDevices); i++ {
        if dev.normalizedDeviceNo == seenDevices[i].normalizedDeviceNo {
            // Notify when the device comes back
            if seenDevices[i].notifiedAsUnseen {
                minutesAgo := int64(time.Now().Sub(seenDevices[i].seen) / time.Minute)
                hoursAgo := minutesAgo / 60
                daysAgo := hoursAgo / 24
                message := fmt.Sprintf("%d minutes", minutesAgo)
                switch {
                case daysAgo >= 2:
                    message = fmt.Sprintf("~%d days", daysAgo)
                case minutesAgo >= 120:
                    message = fmt.Sprintf("~%d hours", hoursAgo)
                }
                sendToSafecastOps(fmt.Sprintf("** NOTE ** Device %d has returned after %s away", seenDevices[i].normalizedDeviceNo, message))
            }
            // Mark as seen
            seenDevices[i].seen = time.Now().UTC()
            seenDevices[i].notifiedAsUnseen = false;
            // Keep note of whether  we've seen both devices of a set of dual-tube updates
            if (dev.originalDeviceNo != seenDevices[i].originalDeviceNo) {
                seenDevices[i].dualSeen = true
                if (seenDevices[i].originalDeviceNo == seenDevices[i].normalizedDeviceNo) {
                    seenDevices[i].originalDeviceNo = dev.originalDeviceNo
                }
            }
            found = true
            break
        }
    }

    // Add a new array entry if necessary
    if !found {
        dev.seen = time.Now().UTC()
        dev.notifiedAsUnseen = false
        dev.dualSeen = false
        seenDevices = append(seenDevices, dev)
    }

}

// Update message ages and notify
func sendSafecastCommsErrorsToSlack(PeriodMinutes uint32) {
    if (httpTransactionErrors != 0) {
        if (httpTransactionErrors == 1) {
            // As of 10/2016, I've chosen to suppress single-instance errors simply because they occur too frequently,
            // i.e. every day or few days.  When we ultimately move the dev server to AWS, we should re-enable this.
            if (false) {
                sendToSafecastOps(fmt.Sprintf("** Warning **  At %s UTC, one error uploading to Safecast:%s)",
                    httpTransactionErrorTime, httpTransactionErrorString));
            }
        } else {
            sendToSafecastOps(fmt.Sprintf("** Warning **  At %s UTC, %d errors uploading to Safecast in %d minutes:%s)",
                httpTransactionErrorTime, httpTransactionErrors, PeriodMinutes, httpTransactionErrorString));
            sendToSafecastApi(fmt.Sprintf("** Warning **  At %s UTC, %d errors uploading to Safecast in %d minutes:%s)",
                httpTransactionErrorTime, httpTransactionErrors, PeriodMinutes, httpTransactionErrorString));
        }
        httpTransactionErrors = 0
        httpTransactionErrorFirst = true;
    }
}

// Update message ages and notify
func sendExpiredSafecastDevicesToSlack() {

    // Compute an expiration time
    const deviceWarningAfterMinutes = 90
    expiration := time.Now().Add(-(time.Duration(deviceWarningAfterMinutes) * time.Minute))

    // Sweep through all devices that we've seen
    for i := 0; i < len(seenDevices); i++ {

        // Update when we've last seen the device
        seenDevices[i].minutesAgo = int64(time.Now().Sub(seenDevices[i].seen) / time.Minute)

        // Notify Slack once and only once when a device has expired
        if !seenDevices[i].notifiedAsUnseen {
            if seenDevices[i].seen.Before(expiration) {
                seenDevices[i].notifiedAsUnseen = true
                sendToSafecastOps(fmt.Sprintf("** Warning **  Device %d hasn't been seen for %d minutes",
                    seenDevices[i].normalizedDeviceNo,
                    seenDevices[i].minutesAgo))
            }
        }
    }
}

// Get a summary of devices that are older than this many minutes ago
func sendSafecastDeviceSummaryToSlack() {

    // First, age out the expired devices and recompute when last seen
    sendExpiredSafecastDevicesToSlack()

    // Next sort the device list
    sortedDevices := seenDevices
    sort.Sort(ByKey(sortedDevices))

    // Finally, sweep over all these devices in sorted order,
    // generating a single large text string to be sent as a Slack message
    s := "No devices yet."
    for i := 0; i < len(sortedDevices); i++ {
        id := sortedDevices[i].normalizedDeviceNo

        if i == 0 {
            s = ""
        } else {
            s = fmt.Sprintf("%s\n", s)
        }

        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc|%010d>", s, id, id)

        if (sortedDevices[i].dualSeen) {
            s = fmt.Sprintf("%s <http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc|%010d>", s,
                sortedDevices[i].originalDeviceNo, sortedDevices[i].originalDeviceNo)
        }

        s = fmt.Sprintf("%s (", s)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=bat_voltage|V>", s, id)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=bat_soc|%%>", s, id)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=bat_current|I>", s, id)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=env_temp|T>", s, id)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=env_humid|H>", s, id)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=env_press|P>", s, id)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=wireless_snr|S>", s, id)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=stats|X>", s, id)
        s = fmt.Sprintf("%s)", s)

        if sortedDevices[i].minutesAgo == 0 {
            s = fmt.Sprintf("%s last seen just now", s)
        } else {
            s = fmt.Sprintf("%s last seen %dm ago", s, sortedDevices[i].minutesAgo)
        }

    }

    // Send it to Slack
    sendToSafecastOps(s)

}

// Write to both logs
func writeToLogs(scV1 SafecastDataV1, scV2 SafecastDataV2) {
	SafecastV1Log(scV1)
	SafecastV2Log(scV2)
}

// Write the value to the log
func SafecastV1Log(scV1 SafecastDataV1) {

    // The file pathname on the server
    usr, _ := user.Current()
    directory := usr.HomeDir
    directory = directory + TTServerLogPath

    // Extract the device number and form a filename
    file := directory + "/" + scV1.DeviceID + ".csv"

    // Open it
    fd, err := os.OpenFile(file, os.O_RDWR|os.O_APPEND, 0666)
    if (err != nil) {

        // Attempt to create the file if it doesn't already exist
        fd, err = os.OpenFile(file, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
        if (err != nil) {
            fmt.Printf("Logging: error creating file %s: %s\n", file, err);
            return;
        }

        // Write the header
        fd.WriteString("Logged,Captured,Device ID,Stats,Uptime,CPM0,CPM1,Latitude,Longitude,Altitude,Bat V,Bat SOC,Bat I,SNR,Temp C,Humid %,Press Pa,PMS PM 1.0,PMS PM 2.5,PMS PM 10.0,PMS # 0.3,PMS # 0.5,PMS # 1.0,PMS # 2.5,PMS # 5.0,PMS # 10.0,PMS # Secs,OPC PM 1.0,OPC PM 2.5,OPC PM 10.0,OPC # 0.38,OPC # 0.54,OPC # 1.0,OPC # 2.1,OPC # 5.0,OPC # 10.0,OPC # Secs\r\n");

    }

    // Turn stats into a safe string for CSV
    stats := scV1.DeviceTypeID;
    if (stats != "") {
        stats = strings.Replace(stats, "\"", "", -1)
        stats = strings.Replace(stats, ",", " ", -1)
        stats = strings.Replace(stats, "{", "\"", -1)
        stats = strings.Replace(stats, "}", "\"", -1)
    }

    // Write the stuff
    s := fmt.Sprintf("%s", time.Now().Format("2006-01-02 15:04:05"))
    s = s + fmt.Sprintf(",%s", scV1.CapturedAt)
    s = s + fmt.Sprintf(",%s", scV1.DeviceID)
    s = s + fmt.Sprintf(",%s", stats)
    s = s + fmt.Sprintf(",%s", scV1.Value)
    s = s + fmt.Sprintf(",%s", scV1.Cpm0)
    s = s + fmt.Sprintf(",%s", scV1.Cpm1)
    s = s + fmt.Sprintf(",%s", scV1.Latitude)
    s = s + fmt.Sprintf(",%s", scV1.Longitude)
    s = s + fmt.Sprintf(",%s", scV1.Height)
    s = s + fmt.Sprintf(",%s", scV1.BatVoltage)
    s = s + fmt.Sprintf(",%s", scV1.BatSOC)
    s = s + fmt.Sprintf(",%s", scV1.BatCurrent)
    s = s + fmt.Sprintf(",%s", scV1.WirelessSNR)
    s = s + fmt.Sprintf(",%s", scV1.EnvTemp)
    s = s + fmt.Sprintf(",%s", scV1.EnvHumid)
    s = s + fmt.Sprintf(",%s", scV1.EnvPress)
    s = s + fmt.Sprintf(",%s", scV1.PmsPm01_0)
    s = s + fmt.Sprintf(",%s", scV1.PmsPm02_5)
    s = s + fmt.Sprintf(",%s", scV1.PmsPm10_0)
    s = s + fmt.Sprintf(",%s", scV1.PmsC00_30)
    s = s + fmt.Sprintf(",%s", scV1.PmsC00_50)
    s = s + fmt.Sprintf(",%s", scV1.PmsC01_00)
    s = s + fmt.Sprintf(",%s", scV1.PmsC02_50)
    s = s + fmt.Sprintf(",%s", scV1.PmsC05_00)
    s = s + fmt.Sprintf(",%s", scV1.PmsC10_00)
    s = s + fmt.Sprintf(",%s", scV1.PmsCsecs)
    s = s + fmt.Sprintf(",%s", scV1.OpcPm01_0)
    s = s + fmt.Sprintf(",%s", scV1.OpcPm02_5)
    s = s + fmt.Sprintf(",%s", scV1.OpcPm10_0)
    s = s + fmt.Sprintf(",%s", scV1.OpcC00_38)
    s = s + fmt.Sprintf(",%s", scV1.OpcC00_54)
    s = s + fmt.Sprintf(",%s", scV1.OpcC01_00)
    s = s + fmt.Sprintf(",%s", scV1.OpcC02_10)
    s = s + fmt.Sprintf(",%s", scV1.OpcC05_00)
    s = s + fmt.Sprintf(",%s", scV1.OpcC10_00)
    s = s + fmt.Sprintf(",%s", scV1.OpcCsecs)
    s = s + "\r\n"

    fd.WriteString(s);

    // Close and exit
    fd.Close();

}

// Write the value to the log
func SafecastV2Log(scV2 SafecastDataV2) {

    // The file pathname on the server
    usr, _ := user.Current()
    directory := usr.HomeDir
    directory = directory + TTServerLogPath

    // Extract the device number and form a filename
    file := directory + "/" + fmt.Sprintf("%d", scV2.DeviceID) + ".json"

    // Open it
    fd, err := os.OpenFile(file, os.O_RDWR|os.O_APPEND, 0666)
    if (err != nil) {

        // Attempt to create the file if it doesn't already exist
        fd, err = os.OpenFile(file, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
        if (err != nil) {
            fmt.Printf("Logging: error creating file %s: %s\n", file, err);
            return;
        }

    }

    // Turn stats into a safe string writing
    scJSON, _ := json.Marshal(scV2)
    fd.WriteString(string(scJSON));
	fd.WriteString("\r\n\r\n");

    // Close and exit
    fd.Close();

}

// Convert v1 to v2
func SafecastV1toV2(v1 SafecastDataV1) SafecastDataV2 {
	var v2 SafecastDataV2
	var i64 uint64
	var f64 float64
	var subtype uint32

	v2.CapturedAt = v1.CapturedAt

	i64, _ = strconv.ParseUint(v1.DeviceID, 10, 32)
	subtype = uint32(i64) % 10
	v2.DeviceID = uint32(i64) - subtype

	f64, _ = strconv.ParseFloat(v1.Height, 32)
	v2.Height = float32(f64)

	f64, _ = strconv.ParseFloat(v1.Latitude, 32)
	v2.Latitude = float32(f64)

	f64, _ = strconv.ParseFloat(v1.Longitude, 32)
	v2.Longitude = float32(f64)

	switch (v1.Unit) {

	case "pm2.5":
		f64, _ = strconv.ParseFloat(v1.Value, 32)
		v2.PmsPm02_5 = float32(f64)

	case "cpm":
		f64, _ = strconv.ParseFloat(v1.Value, 32)
		if (subtype == 1) {
			v2.Cpm0 = float32(f64)
		} else if (subtype == 2) {
			v2.Cpm1 = float32(f64)
		} else {
			fmt.Sprintf("*** V1toV2 %d cpm not understood for this subtype\n", v2.DeviceID);
		}

	case "status":
		v2.Status = v1.DeviceTypeID
		f64, _ = strconv.ParseFloat(v1.Value, 32)
		v2.EnvTemp = float32(f64)

	default:
		fmt.Sprintf("*** Warning ***\n*** Unit %s = Value %s UNRECOGNIZED\n", v1.Unit, v1.Value)

	}

	return v2
}
