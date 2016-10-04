// Safecast inbound message handling and publishing
package main

import (
    "net/http"
    "fmt"
    "bytes"
    "sort"
    "time"
    "strconv"
    "encoding/json"
    "github.com/rayozzie/teletype-proto/golang"
)

// For dealing with transaction timeouts
var httpTransactionsInProgress int = 0
var httpTransactions = 0
const httpTransactionsRecorded = 25
var httpTransactionDurations[httpTransactionsRecorded] int
var httpTransactionTimes[httpTransactionsRecorded] time.Time

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

// Safecast stats
type safecastStats struct {
    StatsUptimeMinutes    uint32 `json:"uptime_min,omitempty"`
    StatsAppVersion       string `json:"version,omitempty"`
    StatsDeviceParams     string `json:"config,omitempty"`
    StatsTransmittedBytes uint32 `json:"transmitted_bytes,omitempty"`
    StatsReceivedBytes    uint32 `json:"received_bytes,omitempty"`
    StatsCommsResets      uint32 `json:"comms_resets,omitempty"`
    StatsOneshots         uint32 `json:"oneshots,omitempty"`
}

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
        fmt.Printf("%s From %s/%s/%s\n", time.Now().Format(logDateFormat), info.City, info.Region, info.Country)
    }

    // Log it
    trackDevice(TelecastDeviceID(msg))

    // Generate the fields common to all uploads to safecast
    sc := SafecastData{}
    if msg.DeviceIDString != nil {
        sc.DeviceID = msg.GetDeviceIDString()
    } else if msg.DeviceIDNumber != nil {
        sc.DeviceID = strconv.FormatUint(uint64(msg.GetDeviceIDNumber()), 10)
    } else {
        sc.DeviceID = "UNKNOWN"
    }
    if msg.CapturedAt != nil {
        sc.CapturedAt = msg.GetCapturedAt()
    } else {
        sc.CapturedAt = defaultTime
    }

    // Include lat/lon/alt on all messages, including metadata
    if msg.Latitude != nil {
        sc.Latitude = fmt.Sprintf("%f", msg.GetLatitude())
    } else {
        if defaultLat != 0.0 {
            sc.Latitude = fmt.Sprintf("%f", defaultLat)
        } else {
            sc.Latitude = "1.23"
        }
    }
    if msg.Longitude != nil {
        sc.Longitude = fmt.Sprintf("%f", msg.GetLongitude())
    } else {
        if defaultLon != 0.0 {
            sc.Longitude = fmt.Sprintf("%f", defaultLon)
        } else {
            sc.Longitude = "1.23"
        }
    }
    if msg.Altitude != nil {
        sc.Height = fmt.Sprintf("%d", msg.GetAltitude())
    } else {
        if defaultAlt != 0.0 {
            sc.Height = fmt.Sprintf("%d", defaultAlt)
        }
    }

    // The first/primary upload has all known fields.  It is
    // our goal that someday this is the *only* upload,
    // after the Safecast service is upgraded.
    sc1 := sc

    // Process the most basic message types
    if msg.StatsUptimeMinutes != nil {

        // A stats message
        sc1.Unit = UnitStats
        sc1.Value = fmt.Sprintf("%d", msg.GetStatsUptimeMinutes())

        var scStats safecastStats
        scStats.StatsUptimeMinutes = msg.GetStatsUptimeMinutes()
        if (msg.StatsAppVersion != nil) {
            scStats.StatsAppVersion = msg.GetStatsAppVersion()
        }
        if (msg.StatsDeviceParams != nil) {
            scStats.StatsDeviceParams = msg.GetStatsDeviceParams()
        }
        if (msg.StatsTransmittedBytes != nil) {
            scStats.StatsTransmittedBytes = msg.GetStatsTransmittedBytes()
        }
        if (msg.StatsReceivedBytes != nil) {
            scStats.StatsReceivedBytes = msg.GetStatsReceivedBytes()
        }
        if (msg.StatsCommsResets != nil) {
            scStats.StatsCommsResets = msg.GetStatsCommsResets()
        }
        if (msg.StatsOneshots != nil) {
            scStats.StatsOneshots = msg.GetStatsOneshots()
        }

        scsJSON, _ := json.Marshal(scStats)
        sc1.DeviceTypeID = string(scsJSON)

    } else if msg.Message != nil {

        // A text message.  Since the Value in safecast
        // must be a number, we use a different text field instead.
        sc1.Unit = UnitMessage
        sc1.DeviceTypeID = msg.GetMessage()

    } else {

        // An old-style safecast geiger upload.  If it's lacking
        // a value, don't add a unit.  This means that
        // it was a metadata-only upload, or a new style upload
        // which only has cpm0/cpm1 fields.
        if msg.Value != nil {
            if msg.Unit == nil {
                sc1.Unit = UnitCPM
            } else {
                sc1.Unit = fmt.Sprintf("%s", msg.GetUnit())
            }
            sc1.Value = fmt.Sprintf("%d", msg.GetValue())
        }

    }

    if msg.BatteryVoltage != nil {
        sc1.BatVoltage = fmt.Sprintf("%.2f", msg.GetBatteryVoltage())
    }
    if msg.BatterySOC != nil {
        sc1.BatSOC = fmt.Sprintf("%.2f", msg.GetBatterySOC())
    }

    if msg.EnvTemperature != nil {
        sc1.EnvTemp = fmt.Sprintf("%.2f", msg.GetEnvTemperature())
    }
    if msg.EnvHumidity != nil {
        sc1.EnvHumid = fmt.Sprintf("%.2f", msg.GetEnvHumidity())
    }

    if msg.WirelessSNR != nil {
        theSNR = msg.GetWirelessSNR()
    } else {
        theSNR = defaultSNR
    }
    if defaultSNR != 0.0 {
        sc1.WirelessSNR = fmt.Sprintf("%.1f", theSNR)
    }

    if msg.PmsPm01_0 != nil {
        sc1.PmsPm01_0 = fmt.Sprintf("%d", msg.GetPmsPm01_0())
    }
    if msg.PmsPm02_5 != nil {
        sc1.PmsPm02_5 = fmt.Sprintf("%d", msg.GetPmsPm02_5())
    }
    if msg.PmsPm10_0 != nil {
        sc1.PmsPm10_0 = fmt.Sprintf("%d", msg.GetPmsPm10_0())
    }
    if msg.PmsC00_30 != nil {
        sc1.PmsC00_30 = fmt.Sprintf("%d", msg.GetPmsC00_30())
    }
    if msg.PmsC00_50 != nil {
        sc1.PmsC00_50 = fmt.Sprintf("%d", msg.GetPmsC00_50())
    }
    if msg.PmsC01_00 != nil {
        sc1.PmsC01_00 = fmt.Sprintf("%d", msg.GetPmsC01_00())
    }
    if msg.PmsC02_50 != nil {
        sc1.PmsC02_50 = fmt.Sprintf("%d", msg.GetPmsC02_50())
    }
    if msg.PmsC05_00 != nil {
        sc1.PmsC05_00 = fmt.Sprintf("%d", msg.GetPmsC05_00())
    }
    if msg.PmsC10_00 != nil {
        sc1.PmsC10_00 = fmt.Sprintf("%d", msg.GetPmsC10_00())
    }
    if msg.PmsCsecs != nil {
        sc1.PmsCsecs = fmt.Sprintf("%d", msg.GetPmsCsecs())
    }

    if msg.OpcPm01_0 != nil {
        sc1.OpcPm01_0 = fmt.Sprintf("%f", msg.GetOpcPm01_0())
    }
    if msg.OpcPm02_5 != nil {
        sc1.OpcPm02_5 = fmt.Sprintf("%f", msg.GetOpcPm02_5())
    }
    if msg.OpcPm10_0 != nil {
        sc1.OpcPm10_0 = fmt.Sprintf("%f", msg.GetOpcPm10_0())
    }
    if msg.OpcC00_38 != nil {
        sc1.OpcC00_38 = fmt.Sprintf("%d", msg.GetOpcC00_38())
    }
    if msg.OpcC00_54 != nil {
        sc1.OpcC00_54 = fmt.Sprintf("%d", msg.GetOpcC00_54())
    }
    if msg.OpcC01_00 != nil {
        sc1.OpcC01_00 = fmt.Sprintf("%d", msg.GetOpcC01_00())
    }
    if msg.OpcC02_10 != nil {
        sc1.OpcC02_10 = fmt.Sprintf("%d", msg.GetOpcC02_10())
    }
    if msg.OpcC05_00 != nil {
        sc1.OpcC05_00 = fmt.Sprintf("%d", msg.GetOpcC05_00())
    }
    if msg.OpcC10_00 != nil {
        sc1.OpcC10_00 = fmt.Sprintf("%d", msg.GetOpcC10_00())
    }
    if msg.OpcCsecs != nil {
        sc1.OpcCsecs = fmt.Sprintf("%d", msg.GetOpcCsecs())
    }

    // Upload differently based on how CPM is represented
    if msg.Cpm0 == nil && msg.Cpm1 == nil {

		// Either an old-style upload, and the kind used by bGeigies,
		// or an upload of metadata without any kind of CPM
        uploadToSafecast(sc1)

    } else if msg.DeviceIDNumber != nil {

		// A new style upload has "cpm0" or "cpm1" values, and
		// must have a numeric device ID
        if msg.Cpm0 != nil {
            sc2 := sc
			sc2.DeviceID = strconv.FormatUint(uint64(msg.GetDeviceIDNumber() & 0xfffffffe), 10)
            sc2.Unit = UnitCPM
            sc2.Value = fmt.Sprintf("%d", msg.GetCpm0())
            uploadToSafecast(sc2)
        }
        if msg.Cpm1 != nil {
            sc2 := sc
			sc2.DeviceID = strconv.FormatUint(uint64(msg.GetDeviceIDNumber() | 0x00000001), 10)
            sc2.Unit = UnitCPM
            sc2.Value = fmt.Sprintf("%d", msg.GetCpm1())
            uploadToSafecast(sc2)
        }

    }

    // Due to Safecast API design limitations, upload the metadata as
    // discrete web uploads.  Once this API limitation is removed,
    // this code should be deleted.
    if msg.BatteryVoltage != nil {
        sc2 := sc
        sc2.Unit = UnitBatVoltage
        sc2.Value = sc1.BatVoltage
        uploadToSafecast(sc2)
    }
    if msg.BatterySOC != nil {
        sc2 := sc
        sc2.Unit = UnitBatSOC
        sc2.Value = sc1.BatSOC
        uploadToSafecast(sc2)
    }
    if msg.EnvTemperature != nil {
        sc2 := sc
        sc2.Unit = UnitEnvTemp
        sc2.Value = sc1.EnvTemp
        uploadToSafecast(sc2)
    }
    if msg.EnvHumidity != nil {
        sc2 := sc
        sc2.Unit = UnitEnvHumid
        sc2.Value = sc1.EnvHumid
        uploadToSafecast(sc2)
    }

	// Only bother uploading a SNR value if it coincides with another
	// really low-occurrance feature, because the device just doesn't
	// move that much and so its SNR should remain reasonably constant
	// except for rain.
    if theSNR != 0.0 && msg.BatteryVoltage != nil {
        sc2 := sc
        sc2.Unit = UnitWirelessSNR
        sc2.Value = sc1.WirelessSNR
        uploadToSafecast(sc2)
    }

    if msg.PmsPm01_0 != nil {
        sc2 := sc
        sc2.Unit = UnitPmsPm01_0
        sc2.Value = sc1.PmsPm01_0
        uploadToSafecast(sc2)
    }
    if msg.PmsPm02_5 != nil {
        sc2 := sc
        sc2.Unit = UnitPmsPm02_5
        sc2.Value = sc1.PmsPm02_5
        uploadToSafecast(sc2)
    }
    if msg.PmsPm10_0 != nil {
        sc2 := sc
        sc2.Unit = UnitPmsPm10_0
        sc2.Value = sc1.PmsPm10_0
        uploadToSafecast(sc2)
    }
    if msg.PmsC00_30 != nil {
        sc2 := sc
        sc2.Unit = UnitPmsC00_30
        sc2.Value = sc1.PmsC00_30
        uploadToSafecast(sc2)
    }
    if msg.PmsC00_50 != nil {
        sc2 := sc
        sc2.Unit = UnitPmsC00_50
        sc2.Value = sc1.PmsC00_50
        uploadToSafecast(sc2)
    }
    if msg.PmsC01_00 != nil {
        sc2 := sc
        sc2.Unit = UnitPmsC01_00
        sc2.Value = sc1.PmsC01_00
        uploadToSafecast(sc2)
    }
    if msg.PmsC02_50 != nil {
        sc2 := sc
        sc2.Unit = UnitPmsC02_50
        sc2.Value = sc1.PmsC02_50
        uploadToSafecast(sc2)
    }
    if msg.PmsC05_00 != nil {
        sc2 := sc
        sc2.Unit = UnitPmsC05_00
        sc2.Value = sc1.PmsC05_00
        uploadToSafecast(sc2)
    }
    if msg.PmsC10_00 != nil {
        sc2 := sc
        sc2.Unit = UnitPmsC10_00
        sc2.Value = sc1.PmsC10_00
        uploadToSafecast(sc2)
    }
    if msg.PmsCsecs != nil {
        sc2 := sc
        sc2.Unit = UnitPmsCsecs
        sc2.Value = sc1.PmsCsecs
        uploadToSafecast(sc2)
    }

    if msg.OpcPm01_0 != nil {
        sc2 := sc
        sc2.Unit = UnitOpcPm01_0
        sc2.Value = sc1.OpcPm01_0
        uploadToSafecast(sc2)
    }
    if msg.OpcPm02_5 != nil {
        sc2 := sc
        sc2.Unit = UnitOpcPm02_5
        sc2.Value = sc1.OpcPm02_5
        uploadToSafecast(sc2)
    }
    if msg.OpcPm10_0 != nil {
        sc2 := sc
        sc2.Unit = UnitOpcPm10_0
        sc2.Value = sc1.OpcPm10_0
        uploadToSafecast(sc2)
    }
    if msg.OpcC00_38 != nil {
        sc2 := sc
        sc2.Unit = UnitOpcC00_38
        sc2.Value = sc1.OpcC00_38
        uploadToSafecast(sc2)
    }
    if msg.OpcC00_54 != nil {
        sc2 := sc
        sc2.Unit = UnitOpcC00_54
        sc2.Value = sc1.OpcC00_54
        uploadToSafecast(sc2)
    }
    if msg.OpcC01_00 != nil {
        sc2 := sc
        sc2.Unit = UnitOpcC01_00
        sc2.Value = sc1.OpcC01_00
        uploadToSafecast(sc2)
    }
    if msg.OpcC02_10 != nil {
        sc2 := sc
        sc2.Unit = UnitOpcC02_10
        sc2.Value = sc1.OpcC02_10
        uploadToSafecast(sc2)
    }
    if msg.OpcC05_00 != nil {
        sc2 := sc
        sc2.Unit = UnitOpcC05_00
        sc2.Value = sc1.OpcC05_00
        uploadToSafecast(sc2)
    }
    if msg.OpcC10_00 != nil {
        sc2 := sc
        sc2.Unit = UnitOpcC10_00
        sc2.Value = sc1.OpcC10_00
        uploadToSafecast(sc2)
    }
    if msg.OpcCsecs != nil {
        sc2 := sc
        sc2.Unit = UnitOpcCsecs
        sc2.Value = sc1.OpcCsecs
        uploadToSafecast(sc2)
    }

}

// Begin transaction and return the transaction ID
func beginTransaction(url string, message string) int {
    httpTransactionsInProgress += 1
    httpTransactions += 1
    transaction := httpTransactions % httpTransactionsRecorded
    httpTransactionTimes[transaction] = time.Now()
    fmt.Printf("%s >>> [%d] %s\n", time.Now().Format(logDateFormat), transaction, message)
    return transaction
}

// End transaction and issue warnings
func endTransaction(transaction int, errstr string) {
    httpTransactionsInProgress -= 1
    duration := int(time.Now().Sub(httpTransactionTimes[transaction]) / time.Second)
    httpTransactionDurations[transaction] = duration

    if errstr != "" {
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
    if (theMin > 10) {
        fmt.Printf("%s Safecast HTTP Upload Statistics\n", time.Now().Format(logDateFormat))
        fmt.Printf("%s *** %d total uploads since restart\n", time.Now().Format(logDateFormat), httpTransactions)
        if (httpTransactionsInProgress > 0) {
            fmt.Printf("%s *** %d uploads still in progress\n", time.Now().Format(logDateFormat), httpTransactionsInProgress)
        }
        fmt.Printf("%s *** Last %d: min=%ds, max=%ds, avg=%ds\n", time.Now().Format(logDateFormat), theCount, theMin, theMax, theMean)

    }

    // If there's a problem, output to Slack once every 25 transactions
    if (theMin > 10 && transaction == 0) {
        // If all of them have the same timeout value, the server must be down.
        s := ""
        if (theMin == theMax && theMin == theMean) {
            s = fmt.Sprintf("Safecast API on %s: all of the most recent %d uploads failed. Please check the service.", SafecastUploadIP, theCount)
        } else {
            s = fmt.Sprintf("Safecast API on %s: of the previous %d uploads, min=%ds, max=%ds, avg=%ds", SafecastUploadIP, theCount, theMin, theMax, theMean)
        }
        sendToSafecastApi(s);
    }

}

// Upload a Safecast data structure to the Safecast service
func uploadToSafecast(sc SafecastData) {

    transaction := beginTransaction(SafecastUploadURL, sc.Unit)

    scJSON, _ := json.Marshal(sc)

	if false {
	    fmt.Printf("%s\n", scJSON)
	}

    req, err := http.NewRequest("POST", fmt.Sprintf(SafecastUploadURL, SafecastAppKey), bytes.NewBuffer(scJSON))
    req.Header.Set("User-Agent", "TTSERVE")
    req.Header.Set("Content-Type", "application/json")
    httpclient := &http.Client{}
    resp, err := httpclient.Do(req)

    errString := ""
    if (err == nil) {
        resp.Body.Close()
    } else {
        errString = fmt.Sprintf("%s", err)
    }

    endTransaction(transaction, errString)

    // The service gets overloaded if we jam it too quickly
    time.Sleep(2 * time.Second)

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
				seenDevices[i].notifiedAsUnseen = false;
                sendToSafecastOps(fmt.Sprintf("** NOTE ** Device %d has returned after away for %d minutes", seenDevices[i].normalizedDeviceNo, minutesAgo))
            }
			// Mark as seen
            seenDevices[i].seen = time.Now().UTC()
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
func sendExpiredSafecastDevicesToSlack() {

    // Compute an expiration time
    const deviceWarningAfterMinutes = 30
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
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=env_temp|T>", s, id)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=env_humid|H>", s, id)
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
