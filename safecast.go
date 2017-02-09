// Safecast inbound message handling and publishing
package main

import (
    "os"
    "net/http"
    "fmt"
    "bytes"
    "sort"
    "time"
    "io/ioutil"
    "strings"
    "strconv"
    "encoding/json"
    "github.com/rayozzie/teletype-proto/golang"
)

// Lat/Lon/Alt behavior at the API
const addFakeLocation = false

// Warning behavior
const deviceWarningAfterMinutes = 90

// For dealing with transaction timeouts
var httpTransactionsInProgress int = 0
var httpTransactions = 0
const httpTransactionsRecorded = 500
var httpTransactionDurations[httpTransactionsRecorded] int
var httpTransactionTimes[httpTransactionsRecorded] time.Time
var httpTransactionErrorTime string
var httpTransactionErrorString string
var httpTransactionErrors = 0
var httpTransactionErrorFirst bool = true

// Describes every device that has sent us a message
type seenDevice struct {
    deviceid            uint32
	deviceSummary	    string
    seen                time.Time
    everRecentlySeen    bool
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
    if a[i].deviceid < a[j].deviceid {
        return true
    } else if a[i].deviceid > a[j].deviceid {
        return false
    }
    return false
}

// Process an inbound Safecast message
func ProcessSafecastMessage(msg *teletype.Telecast,
    checksum uint32,
    ipInfo string,
    UploadedAt string,
    Transport string,
    defaultTime string,
    defaultSNR float32,
    defaultLat float32, defaultLon float32, defaultAlt float32) {

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

    // Process stamps by adding or removing fields from the message
    if (!stampSetOrApply(msg)) {
        fmt.Printf("%s DISCARDING un-stampable message\n", time.Now().Format(logDateFormat));
        return
    }

    // Generate the fields common to all uploads to safecast
    scV2 := SafecastDataV2{}
    if msg.DeviceID != nil {
        scV2.DeviceID = msg.GetDeviceID()
    } else {
        scV2.DeviceID = 0
    }
    if msg.CapturedAt != nil {
        scV2.CapturedAt = msg.GetCapturedAt()
    } else {
        scV2.CapturedAt = defaultTime
    }

    // Handle the GPS capture fields
    if (msg.CapturedAtDate != nil && msg.CapturedAtTime != nil) {
        var i64 uint64
        var offset uint32 = 0
        if (msg.CapturedAtOffset != nil) {
            offset = msg.GetCapturedAtOffset()
        }
        s := fmt.Sprintf("%06d%06d", msg.GetCapturedAtDate(), msg.GetCapturedAtTime())
        i64, _ = strconv.ParseUint(fmt.Sprintf("%c%c", s[0], s[1]), 10, 32)
        day := uint32(i64)
        i64, _ = strconv.ParseUint(fmt.Sprintf("%c%c", s[2], s[3]), 10, 32)
        month := uint32(i64)
        i64, _ = strconv.ParseUint(fmt.Sprintf("%c%c", s[4], s[5]), 10, 32)
        year := uint32(i64) + 2000
        i64, _ = strconv.ParseUint(fmt.Sprintf("%c%c", s[6], s[7]), 10, 32)
        hour := uint32(i64)
        i64, _ = strconv.ParseUint(fmt.Sprintf("%c%c", s[8], s[9]), 10, 32)
        minute := uint32(i64)
        i64, _ = strconv.ParseUint(fmt.Sprintf("%c%c", s[10], s[11]), 10, 32)
        second := uint32(i64)
        tbefore := time.Date(int(year), time.Month(month), int(day), int(hour), int(minute), int(second), 0, time.UTC)
        tafter := tbefore.Add(time.Duration(offset) * time.Second)
        tstr := tafter.UTC().Format("2006-01-02T15:04:05Z")
        scV2.CapturedAt = tstr
    }

    // Include lat/lon/alt on all messages, including metadata
    if msg.Latitude != nil {
        scV2.Latitude =  msg.GetLatitude()
    } else {
		if (addFakeLocation) {
            scV2.Latitude = defaultLat
		}
    }
    if msg.Longitude != nil {
        scV2.Longitude = msg.GetLongitude()
    } else {
		if (addFakeLocation) {
            scV2.Longitude = defaultLon
        }
    }
    if msg.Altitude != nil {
        scV2.Height = float32(msg.GetAltitude())
    } else {
		if (addFakeLocation) {
            scV2.Height = defaultAlt
        }
    }

    // The first/primary upload has all known fields.  It is
    // our goal that someday this is the *only* upload,
    // after the Safecast service is upgraded.
    scV2a := scV2

    // Process the most basic message types
    if msg.StatsUptimeMinutes != nil {

        // A stats message
        scV2a.StatsUptimeMinutes = msg.GetStatsUptimeMinutes()
        if (msg.StatsUptimeDays != nil) {
            scV2a.StatsUptimeMinutes += msg.GetStatsUptimeDays() * 24 * 60
        }
        if (msg.StatsAppVersion != nil) {
            scV2a.StatsAppVersion = msg.GetStatsAppVersion()
        }
        if (msg.StatsDeviceParams != nil) {
            scV2a.StatsDeviceParams = msg.GetStatsDeviceParams()
        }
        if (msg.StatsTransmittedBytes != nil) {
            scV2a.StatsTransmittedBytes = msg.GetStatsTransmittedBytes()
        }
        if (msg.StatsReceivedBytes != nil) {
            scV2a.StatsReceivedBytes = msg.GetStatsReceivedBytes()
        }
        if (msg.StatsCommsResets != nil) {
            scV2a.StatsCommsResets = msg.GetStatsCommsResets()
        }
        if (msg.StatsCommsPowerFails != nil) {
            scV2a.StatsCommsPowerFails = msg.GetStatsCommsPowerFails()
        }
        if (msg.StatsOneshots != nil) {
            scV2a.StatsOneshots = msg.GetStatsOneshots()
        }
        if (msg.StatsOneshotSeconds != nil) {
            scV2a.StatsOneshotSeconds = msg.GetStatsOneshotSeconds()
        }
        if (msg.StatsMotiondrops != nil) {
            scV2a.StatsMotiondrops = msg.GetStatsMotiondrops()
        }
        if (msg.StatsIccid != nil) {
            scV2a.StatsIccid = msg.GetStatsIccid()
        }
        if (msg.StatsCpsi != nil) {
            scV2a.StatsCpsi = msg.GetStatsCpsi()
        }
        if (msg.StatsDfu != nil) {
            scV2a.StatsDfu = msg.GetStatsDfu()
        }
        if (msg.StatsDeviceInfo != nil) {
            scV2a.StatsDeviceInfo = msg.GetStatsDeviceInfo()
        }

    } else if msg.Message != nil {

        // A text message.
        scV2a.Message = msg.GetMessage()

    }

    if msg.BatteryVoltage != nil {
        scV2a.BatVoltage = msg.GetBatteryVoltage()
    }
    if msg.BatterySOC != nil {
        scV2a.BatSOC = msg.GetBatterySOC()
    }

    if msg.BatteryCurrent != nil {
        scV2a.BatCurrent = msg.GetBatteryCurrent()
    }

    if msg.EnvTemperature != nil {
        scV2a.EnvTemp = msg.GetEnvTemperature()
    }
    if msg.EnvHumidity != nil {
        scV2a.EnvHumid = msg.GetEnvHumidity()
    }
    if msg.EnvPressure != nil {
        scV2a.EnvPress = msg.GetEnvPressure()
    }

    scV2a.Transport = Transport

    if msg.WirelessSNR != nil {
        scV2a.WirelessSNR = msg.GetWirelessSNR()
    } else {
		if (defaultSNR != 0.0) {
			scV2a.WirelessSNR = defaultSNR
		}
    }

    if msg.PmsPm01_0 != nil {
        scV2a.PmsPm01_0 = float32(msg.GetPmsPm01_0())
    }
    if msg.PmsPm02_5 != nil {
        scV2a.PmsPm02_5 = float32(msg.GetPmsPm02_5())
    }
    if msg.PmsPm10_0 != nil {
        scV2a.PmsPm10_0 = float32(msg.GetPmsPm10_0())
    }
    if msg.PmsC00_30 != nil {
        scV2a.PmsC00_30 = msg.GetPmsC00_30()
    }
    if msg.PmsC00_50 != nil {
        scV2a.PmsC00_50 = msg.GetPmsC00_50()
    }
    if msg.PmsC01_00 != nil {
        scV2a.PmsC01_00 = msg.GetPmsC01_00()
    }
    if msg.PmsC02_50 != nil {
        scV2a.PmsC02_50 = msg.GetPmsC02_50()
    }
    if msg.PmsC05_00 != nil {
        scV2a.PmsC05_00 = msg.GetPmsC05_00()
    }
    if msg.PmsC10_00 != nil {
        scV2a.PmsC10_00 = msg.GetPmsC10_00()
    }
    if msg.PmsCsecs != nil {
        scV2a.PmsCsecs = msg.GetPmsCsecs()
    }

    if msg.OpcPm01_0 != nil {
        scV2a.OpcPm01_0 = msg.GetOpcPm01_0()
    }
    if msg.OpcPm02_5 != nil {
        scV2a.OpcPm02_5 = msg.GetOpcPm02_5()
    }
    if msg.OpcPm10_0 != nil {
        scV2a.OpcPm10_0 = msg.GetOpcPm10_0()
    }
    if msg.OpcC00_38 != nil {
        scV2a.OpcC00_38 = msg.GetOpcC00_38()
    }
    if msg.OpcC00_54 != nil {
        scV2a.OpcC00_54 = msg.GetOpcC00_54()
    }
    if msg.OpcC01_00 != nil {
        scV2a.OpcC01_00 = msg.GetOpcC01_00()
    }
    if msg.OpcC02_10 != nil {
        scV2a.OpcC02_10 = msg.GetOpcC02_10()
    }
    if msg.OpcC05_00 != nil {
        scV2a.OpcC05_00 = msg.GetOpcC05_00()
    }
    if msg.OpcC10_00 != nil {
        scV2a.OpcC10_00 = msg.GetOpcC10_00()
    }
    if msg.OpcCsecs != nil {
        scV2a.OpcCsecs = msg.GetOpcCsecs()
    }

	// Bring CPM over
    if (msg.Cpm0 != nil) {
        scV2a.Cpm0 = float32(msg.GetCpm0())
	}
    if (msg.Cpm1 != nil) {
        scV2a.Cpm1 = float32(msg.GetCpm1())
    }
	
	// Log and upload
    SafecastWriteToLogs(UploadedAt, scV2a)
    SafecastV2Upload(UploadedAt, scV2a)

}

// Begin transaction and return the transaction ID
func beginTransaction(version string, url string, message1 string, message2 string) int {
    httpTransactionsInProgress += 1
    httpTransactions += 1
    transaction := httpTransactions % httpTransactionsRecorded
    httpTransactionTimes[transaction] = time.Now()
    fmt.Printf("%s >>> %s [%d] %s %s\n", time.Now().Format(logDateFormat), version, transaction, message1, message2)
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
        fmt.Printf("%s <<<    [%d] *** after %d seconds, ERROR uploading to Safecast %s\n\n", time.Now().Format(logDateFormat), transaction, duration, errstr)
    } else {
        if (duration < 5) {
            fmt.Printf("%s <<<    [%d]\n", time.Now().Format(logDateFormat), transaction);
        } else {
            fmt.Printf("%s <<<    [%d] completed after %d seconds\n", time.Now().Format(logDateFormat), transaction, duration);
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

// Keep track of all devices that have logged data via ttserve
func trackDevice(DeviceID uint32, whenSeen time.Time) {
    var dev seenDevice
    dev.deviceid = DeviceID

    // Attempt to update the existing entry if we can find it
    found := false
    for i := 0; i < len(seenDevices); i++ {
        if dev.deviceid == seenDevices[i].deviceid {
            // Only pay attention to things that have truly recently come or gone
            minutesAgo := int64(time.Now().Sub(whenSeen) / time.Minute)
            if (minutesAgo < deviceWarningAfterMinutes) {
                seenDevices[i].everRecentlySeen = true
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
                    sendToSafecastOps(fmt.Sprintf("** NOTE ** Device %d has returned after %s away", seenDevices[i].deviceid, message))
                }
                // Mark as having been seen on the latest date of any file having that time
                seenDevices[i].notifiedAsUnseen = false;
            }
            // Always track the most recent seen date
            if (seenDevices[i].seen.Before(whenSeen)) {
                seenDevices[i].seen = whenSeen
            }
            found = true
            break
        }
    }

    // Add a new array entry if necessary
    if !found {
        dev.seen = whenSeen
        minutesAgo := int64(time.Now().Sub(dev.seen) / time.Minute)
        dev.everRecentlySeen = minutesAgo < deviceWarningAfterMinutes
        dev.notifiedAsUnseen = false
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

    // Loop over the file system, tracking all devices
    // Open the directory
    files, err := ioutil.ReadDir(SafecastDirectory() + TTServerValuePath)
    if err == nil {

        // Iterate over each of the pending commands
        for _, file := range files {

            if !file.IsDir() {

                // Extract device ID from filename
                Str0 := file.Name()
                Str1 := strings.Split(Str0, ".")[0]
                i64, _ := strconv.ParseUint(Str1, 10, 32)
				deviceID := uint32(i64)

                // Track the device
                if (deviceID != 0) {
                    trackDevice(deviceID, file.ModTime())
                }

            }
        }
    }

    // Compute an expiration time
    expiration := time.Now().Add(-(time.Duration(deviceWarningAfterMinutes) * time.Minute))

    // Sweep through all devices that we've seen
    for i := 0; i < len(seenDevices); i++ {

        // Update when we've last seen the device
        seenDevices[i].minutesAgo = int64(time.Now().Sub(seenDevices[i].seen) / time.Minute)

        // Notify Slack once and only once when a device has expired
        if !seenDevices[i].notifiedAsUnseen && seenDevices[i].everRecentlySeen {
            if seenDevices[i].seen.Before(expiration) {
                seenDevices[i].notifiedAsUnseen = true
                sendToSafecastOps(fmt.Sprintf("** Warning **  Device %d hasn't been seen for %d minutes",
                    seenDevices[i].deviceid,
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
        id := sortedDevices[i].deviceid

        if i == 0 {
            s = ""
        } else {
            s = fmt.Sprintf("%s\n", s)
        }

        s += fmt.Sprintf("%010d ", id)
        s += fmt.Sprintf("<http://%s%s%d|now> ", TTServerHTTPAddress, TTServerTopicValue, id)
        s += fmt.Sprintf("<http://%s%s%s%d.json|log> ", TTServerHTTPAddress, TTServerTopicLog, time.Now().UTC().Format("2006-01-"), id)
        s += fmt.Sprintf("<http://%s%s%s%d.csv|csv>", TTServerHTTPAddress, TTServerTopicLog, time.Now().UTC().Format("2006-01-"), id)

        if sortedDevices[i].minutesAgo == 0 {
            s = fmt.Sprintf("%s just now", s)
        } else {
            var minutesAgo uint32 = uint32(sortedDevices[i].minutesAgo)
            var hoursAgo uint32 = minutesAgo / 60
            var daysAgo uint32 = hoursAgo / 24
            minutesAgo -= hoursAgo * 60
            hoursAgo -= daysAgo * 24
            if daysAgo != 0 {
                s = fmt.Sprintf("%s %dd %dh %dm ago", s, daysAgo, hoursAgo, minutesAgo)
            } else if hoursAgo != 0 {
                s = fmt.Sprintf("%s %dh %dm ago", s, hoursAgo, minutesAgo)
            } else {
                s = fmt.Sprintf("%s %dm ago", s, minutesAgo)
            }
        }

		// Append device summary
		summary := SafecastGetSummary(id)
		if summary != "" {
			s += " " + summary
		}

    }

    // Send it to Slack
    sendToSafecastOps(s)

}

// Write to both logs
func SafecastWriteToLogs(UploadedAt string, scV2 SafecastDataV2) {
    SafecastJSONLog(UploadedAt, scV2)
    SafecastCSVLog(UploadedAt, scV2)
    SafecastWriteValue(UploadedAt, scV2)
}

// Get path of the safecast directory
func SafecastDirectory() string {
    directory := os.Args[1]
    if (directory == "") {
        fmt.Printf("TTSERVE: first argument must be folder containing safecast data!\n")
        os.Exit(0)
    }
    return(directory)
}

// Construct path of a log file
func SafecastLogFilename(DeviceID string, Extension string) string {
    directory := SafecastDirectory()
    prefix := time.Now().UTC().Format("2006-01-")
    file := directory + TTServerLogPath + "/" + prefix + DeviceID + Extension
    return file
}

// Write the value to the log
func SafecastCSVLog(UploadedAt string, scV2 SafecastDataV2) {

    // Extract the device number and form a filename
    file := SafecastLogFilename(fmt.Sprintf("%d", scV2.DeviceID), ".csv")

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
        fd.WriteString("Uploaded,Captured,Device ID,Stats,Uptime,CPM0,CPM1,Latitude,Longitude,Altitude,Bat V,Bat SOC,Bat I,SNR,Temp C,Humid %,Press Pa,PMS PM 1.0,PMS PM 2.5,PMS PM 10.0,PMS # 0.3,PMS # 0.5,PMS # 1.0,PMS # 2.5,PMS # 5.0,PMS # 10.0,PMS # Secs,OPC PM 1.0,OPC PM 2.5,OPC PM 10.0,OPC # 0.38,OPC # 0.54,OPC # 1.0,OPC # 2.1,OPC # 5.0,OPC # 10.0,OPC # Secs\r\n");

    }

    // Turn stats into a safe string for CSV
    stats := ""
    if (scV2.StatsUptimeMinutes != 0) {
        stats += fmt.Sprintf("Uptime:%d ", scV2.StatsUptimeMinutes)
    }
    if (scV2.StatsAppVersion != "") {
        stats += fmt.Sprintf("AppVersion:%s ", scV2.StatsAppVersion)
    }
    if (scV2.StatsDeviceParams != "") {
        stats += fmt.Sprintf("AppVersion:%s ", scV2.StatsDeviceParams)
    }
    if (scV2.StatsTransmittedBytes != 0) {
        stats += fmt.Sprintf("Sent:%d ", scV2.StatsTransmittedBytes)
    }
    if (scV2.StatsReceivedBytes != 0) {
        stats += fmt.Sprintf("Rcvd:%d ", scV2.StatsReceivedBytes)
    }
    if (scV2.StatsCommsResets != 0) {
        stats += fmt.Sprintf("CommsResets:%d ", scV2.StatsCommsResets)
    }
    if (scV2.StatsCommsFails != 0) {
        stats += fmt.Sprintf("CommsFails:%d ", scV2.StatsCommsFails)
    }
    if (scV2.StatsCommsPowerFails != 0) {
        stats += fmt.Sprintf("CommsPowerFails:%d ", scV2.StatsCommsPowerFails)
    }
    if (scV2.StatsDeviceRestarts != 0) {
        stats += fmt.Sprintf("Restarts:%d ", scV2.StatsDeviceRestarts)
    }
    if (scV2.StatsMotiondrops != 0) {
        stats += fmt.Sprintf("Motiondrops:%d ", scV2.StatsMotiondrops)
    }
    if (scV2.StatsOneshots != 0) {
        stats += fmt.Sprintf("Oneshots:%d ", scV2.StatsOneshots)
    }
    if (scV2.StatsOneshotSeconds != 0) {
        stats += fmt.Sprintf("OneshotSecs:%d ", scV2.StatsOneshotSeconds)
    }
    if (scV2.StatsIccid != "") {
        stats += fmt.Sprintf("Iccid:%s ", scV2.StatsIccid)
    }
    if (scV2.StatsCpsi != "") {
        stats += fmt.Sprintf("Cpsi:%s ", scV2.StatsCpsi)
    }
    if (scV2.StatsDfu != "") {
        stats += fmt.Sprintf("DFU:%s ", scV2.StatsDfu)
    }
    if (scV2.StatsDeviceInfo != "") {
        stats += fmt.Sprintf("Label:%s ", scV2.StatsDeviceInfo)
    }
    if (scV2.Message != "") {
        stats += fmt.Sprintf("Msg:%s ", scV2.Message)
    }
    if (scV2.StatsFreeMem != 0) {
        stats += fmt.Sprintf("FreeMem:%d ", scV2.StatsFreeMem)
    }
    if (scV2.StatsNTPCount != 0) {
        stats += fmt.Sprintf("NTPCount:%d ", scV2.StatsNTPCount)
    }
    if (scV2.StatsLastFailure != "") {
        stats += fmt.Sprintf("LastFailure:%s ", scV2.StatsLastFailure)
    }
    if (scV2.StatsStatus != "") {
        stats += fmt.Sprintf("Status:%s ", scV2.StatsStatus)
    }

    // Write the stuff
    s := UploadedAt
    s = s + fmt.Sprintf(",%s", scV2.CapturedAt)
    s = s + fmt.Sprintf(",%d", scV2.DeviceID)
    s = s + fmt.Sprintf(",%s", stats)
    s = s + fmt.Sprintf(",%s", "")          // Value
    if scV2.Cpm0 == 0 {
        s = s + fmt.Sprintf(",%s", "")
    } else {
        s = s + fmt.Sprintf(",%f", scV2.Cpm0)
    }
    if scV2.Cpm1 == 0 {
        s = s + fmt.Sprintf(",%s", "")
    } else {
        s = s + fmt.Sprintf(",%f", scV2.Cpm1)
    }
    s = s + fmt.Sprintf(",%f", scV2.Latitude)
    s = s + fmt.Sprintf(",%f", scV2.Longitude)
    s = s + fmt.Sprintf(",%f", scV2.Height)
    if scV2.BatVoltage == 0 {
        s = s + fmt.Sprintf(",%s", "")
    } else {
        s = s + fmt.Sprintf(",%f", scV2.BatVoltage)
    }
    if scV2.BatSOC == 0 {
        s = s + fmt.Sprintf(",%s", "")
    } else {
        s = s + fmt.Sprintf(",%f", scV2.BatSOC)
    }
    if scV2.BatCurrent == 0 {
        s = s + fmt.Sprintf(",%s", "")
    } else {
        s = s + fmt.Sprintf(",%f", scV2.BatCurrent)
    }
    if scV2.WirelessSNR == 0 {
        s = s + fmt.Sprintf(",%s", "")
    } else {
        s = s + fmt.Sprintf(",%f", scV2.WirelessSNR)
    }
    if scV2.EnvTemp == 0 {
        s = s + fmt.Sprintf(",%s", "")
    } else {
        s = s + fmt.Sprintf(",%f", scV2.EnvTemp)
    }
    if scV2.EnvHumid == 0 {
        s = s + fmt.Sprintf(",%s", "")
    } else {
        s = s + fmt.Sprintf(",%f", scV2.EnvHumid)
    }
    if scV2.EnvPress == 0 {
        s = s + fmt.Sprintf(",%s", "")
    } else {
        s = s + fmt.Sprintf(",%f", scV2.EnvPress)
    }
    if (float32(scV2.PmsCsecs) + scV2.PmsPm01_0 + scV2.PmsPm02_5 + scV2.PmsPm10_0) == 0   {
        s = s + fmt.Sprintf(",%s", "")
        s = s + fmt.Sprintf(",%s", "")
        s = s + fmt.Sprintf(",%s", "")
        s = s + fmt.Sprintf(",%s", "")
        s = s + fmt.Sprintf(",%s", "")
        s = s + fmt.Sprintf(",%s", "")
        s = s + fmt.Sprintf(",%s", "")
        s = s + fmt.Sprintf(",%s", "")
        s = s + fmt.Sprintf(",%s", "")
        s = s + fmt.Sprintf(",%s", "")
    } else {
        s = s + fmt.Sprintf(",%f", scV2.PmsPm01_0)
        s = s + fmt.Sprintf(",%f", scV2.PmsPm02_5)
        s = s + fmt.Sprintf(",%f", scV2.PmsPm10_0)
        s = s + fmt.Sprintf(",%d", scV2.PmsC00_30)
        s = s + fmt.Sprintf(",%d", scV2.PmsC00_50)
        s = s + fmt.Sprintf(",%d", scV2.PmsC01_00)
        s = s + fmt.Sprintf(",%d", scV2.PmsC02_50)
        s = s + fmt.Sprintf(",%d", scV2.PmsC05_00)
        s = s + fmt.Sprintf(",%d", scV2.PmsC10_00)
        s = s + fmt.Sprintf(",%d", scV2.PmsCsecs)
    }
    if (float32(scV2.OpcCsecs) + scV2.OpcPm01_0 + scV2.OpcPm02_5 + scV2.OpcPm10_0) == 0   {
        s = s + fmt.Sprintf(",%s", "")
        s = s + fmt.Sprintf(",%s", "")
        s = s + fmt.Sprintf(",%s", "")
        s = s + fmt.Sprintf(",%s", "")
        s = s + fmt.Sprintf(",%s", "")
        s = s + fmt.Sprintf(",%s", "")
        s = s + fmt.Sprintf(",%s", "")
        s = s + fmt.Sprintf(",%s", "")
        s = s + fmt.Sprintf(",%s", "")
        s = s + fmt.Sprintf(",%s", "")
    } else {
        s = s + fmt.Sprintf(",%f", scV2.OpcPm01_0)
        s = s + fmt.Sprintf(",%f", scV2.OpcPm02_5)
        s = s + fmt.Sprintf(",%f", scV2.OpcPm10_0)
        s = s + fmt.Sprintf(",%d", scV2.OpcC00_38)
        s = s + fmt.Sprintf(",%d", scV2.OpcC00_54)
        s = s + fmt.Sprintf(",%d", scV2.OpcC01_00)
        s = s + fmt.Sprintf(",%d", scV2.OpcC02_10)
        s = s + fmt.Sprintf(",%d", scV2.OpcC05_00)
        s = s + fmt.Sprintf(",%d", scV2.OpcC10_00)
        s = s + fmt.Sprintf(",%d", scV2.OpcCsecs)
    }
    s = s + "\r\n"

    fd.WriteString(s);

    // Close and exit
    fd.Close();

}

// Write the value to the log
func SafecastJSONLog(UploadedAt string, scV2 SafecastDataV2) {

    file := SafecastLogFilename(fmt.Sprintf("%d", scV2.DeviceID), ".json")

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
    scV2.UploadedAt = UploadedAt
    scJSON, _ := json.Marshal(scV2)
    fd.WriteString(string(scJSON));
    fd.WriteString("\r\n,\r\n");

    // Close and exit
    fd.Close();

}

// Convert v1 to v2
func SafecastV1toV2(v1 SafecastDataV1) SafecastDataV2 {
    var v2 SafecastDataV2
    var i64 uint64
    var f64 float64
    var subtype uint32

    if (v1.CapturedAt == "") {
        v2.CapturedAt = time.Now().UTC().Format("2006-01-02T15:04:05Z")
    } else {
        v2.CapturedAt = v1.CapturedAt
    }

    i64, _ = strconv.ParseUint(v1.DeviceID, 10, 32)
    subtype = uint32(i64) % 10
    v2.DeviceID = uint32(i64) - subtype

    f64, _ = strconv.ParseFloat(v1.Height, 32)
    v2.Height = float32(f64)

    f64, _ = strconv.ParseFloat(v1.Latitude, 32)
    v2.Latitude = float32(f64)

    f64, _ = strconv.ParseFloat(v1.Longitude, 32)
    v2.Longitude = float32(f64)

    switch (strings.ToLower(v1.Unit)) {

    case "pm1":
        f64, _ = strconv.ParseFloat(v1.Value, 32)
        v2.OpcPm01_0 = float32(f64)

    case "pm2.5":
        f64, _ = strconv.ParseFloat(v1.Value, 32)
        v2.OpcPm02_5 = float32(f64)

    case "pm10":
        f64, _ = strconv.ParseFloat(v1.Value, 32)
        v2.OpcPm10_0 = float32(f64)

    case "humd%":
        f64, _ = strconv.ParseFloat(v1.Value, 32)
        v2.EnvHumid = float32(f64)

    case "tempc":
        f64, _ = strconv.ParseFloat(v1.Value, 32)
        v2.EnvTemp = float32(f64)

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
        f64, _ = strconv.ParseFloat(v1.Value, 32)
        v2.EnvTemp = float32(f64)

        // Parse and split into its sub-fields
        unrecognized := ""
        status := v1.DeviceTypeID
        fields := strings.Split(status, ",")
        for v := range fields {
            field := strings.Split(fields[v], ":")
            switch (field[0]) {
            case "Battery Voltage":
                f64, _ = strconv.ParseFloat(field[1], 32)
                v2.BatVoltage = float32(f64)
            case "Fails":
                i64, _ = strconv.ParseUint(field[1], 10, 32)
                v2.StatsCommsFails = uint32(i64)
            case "Restarts":
                i64, _ = strconv.ParseUint(field[1], 10, 32)
                v2.StatsDeviceRestarts = uint32(i64)
            case "FreeRam":
                i64, _ = strconv.ParseUint(field[1], 10, 32)
                v2.StatsFreeMem = uint32(i64)
            case "NTP count":
                i64, _ = strconv.ParseUint(field[1], 10, 32)
                v2.StatsNTPCount = uint32(i64)
            case "Last failure":
                v2.StatsLastFailure = field[1]
            default:
                if (unrecognized == "") {
                    unrecognized = "{"
                } else {
                    unrecognized = unrecognized + ","
                }
                unrecognized = unrecognized + "\"" + field[0] + "\":\"" + field[1] + "\""
            case "DeviceID":
            case "Temperature":
            }
        }

        // If we found unrecognized fields, emit them
        if (unrecognized != "") {
            unrecognized = unrecognized + "}"
            v2.StatsStatus = unrecognized
        }

    default:
        fmt.Sprintf("*** Warning ***\n*** Unit %s = Value %s UNRECOGNIZED\n", v1.Unit, v1.Value)

    }

    return v2
}

// Upload a Safecast data structure to the Safecast service, either serially or massively in parallel
func SafecastV1Upload(scV1 SafecastDataV1, url string) bool {

	// For V1, We've found that in certain cases the server gets overloaded.  When we run into those cases,
	// turn this OFF and things will slow down.  (Obviously this is not the preferred mode of operation,
	// because it creates a huge queue of things waiting to be uploaded.)
	var parallelV1Uploads = false

    if (parallelV1Uploads) {
        go doUploadToSafecastV1(scV1, url)
    } else {
        if (!doUploadToSafecastV1(scV1, url)) {
            return false
        }
        time.Sleep(1 * time.Second)
    }

    return true

}

// Upload a Safecast data structure to the Safecast service
func doUploadToSafecastV1(scV1 SafecastDataV1, url string) bool {

    transaction := beginTransaction("V1", SafecastV1UploadURL, scV1.Unit, scV1.Value)

    scJSON, _ := json.Marshal(scV1)

    if false {
        fmt.Printf("%s\n", scJSON)
    }

    urlForUpload := fmt.Sprintf("%s?%s", SafecastV1UploadURL, SafecastV1QueryString)
    if (url != "") {
        urlForUpload = url
    }
    req, err := http.NewRequest("POST", urlForUpload, bytes.NewBuffer(scJSON))
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
func SafecastV2Upload(UploadedAt string, scV2 SafecastDataV2) bool {

    // Upload to all URLs
    for _, url := range SafecastV2UploadURLs {
        go doUploadToSafecastV2(UploadedAt, scV2, url)
    }

    return true
}

// Upload a Safecast data structure to the Safecast service
func doUploadToSafecastV2(UploadedAt string, scV2 SafecastDataV2, url string) bool {

    transaction := beginTransaction("V2", url, "captured", scV2.CapturedAt)

    scV2.UploadedAt = UploadedAt
    scJSON, _ := json.Marshal(scV2)

    if false {
        fmt.Printf("%s\n", scJSON)
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

// Save the last value in a file
func SafecastWriteValue(UploadedAt string, sc SafecastDataV2) {
	var ChangedLocation = false
	var ChangedPms = false
	var ChangedOpc = false
	var ChangedGeiger = false
	var ChangedTransport = false

	// Use the supplied upload time as our modification time
	sc.UploadedAt = UploadedAt
	
	// Generate the filename, which we'll use twice
    filename := SafecastDirectory() + TTServerValuePath + "/" + fmt.Sprintf("%d", sc.DeviceID) + ".json"

    // Read the file if it exists, else blank out value
    value := SafecastValue{}
    file, err := ioutil.ReadFile(filename)
    if err == nil {
	    // Read it as JSON
	    err = json.Unmarshal(file, &value)
	    if err != nil {
		    value = SafecastValue{}
		}
    }

	// Update the current values, but only if modified
	value.DeviceID = sc.DeviceID;	
	if sc.UploadedAt != "" {
		value.UploadedAt = sc.UploadedAt
	}
	if sc.CapturedAt != "" {
		value.CapturedAt = sc.CapturedAt
	}
	if sc.Latitude != 0 && sc.Longitude != 0 {
		if (sc.Latitude != value.Latitude || sc.Longitude != value.Longitude) {
			ChangedLocation = true
		}
		value.Latitude = sc.Latitude
		value.Longitude = sc.Longitude
		value.Height = sc.Height
	}
	if sc.BatVoltage != 0 {
		value.BatVoltage = sc.BatVoltage
		value.BatSOC = sc.BatSOC
		value.BatCurrent = sc.BatCurrent
	}
	if sc.EnvTemp != 0 {
		value.EnvTemp = sc.EnvTemp
		value.EnvHumid = sc.EnvHumid
		value.EnvPress = sc.EnvPress
	}
    if (float32(sc.PmsCsecs) + sc.PmsPm01_0 + sc.PmsPm02_5 + sc.PmsPm10_0) != 0 {
		if sc.PmsCsecs != value.PmsCsecs || sc.PmsPm01_0 != value.PmsPm01_0 || sc.PmsPm02_5 != value.PmsPm02_5 || sc.PmsPm10_0 != value.PmsPm10_0 {
			ChangedPms = true
		}
		value.PmsPm01_0 = sc.PmsPm01_0
		value.PmsPm02_5 = sc.PmsPm02_5
		value.PmsPm10_0 = sc.PmsPm10_0
		value.PmsC00_30 = sc.PmsC00_30
		value.PmsC00_50 = sc.PmsC00_50
		value.PmsC01_00 = sc.PmsC01_00
		value.PmsC02_50 = sc.PmsC02_50
		value.PmsC05_00 = sc.PmsC05_00
		value.PmsC10_00 = sc.PmsC10_00
		value.PmsCsecs = sc.PmsCsecs
	}
    if (float32(sc.OpcCsecs) + sc.OpcPm01_0 + sc.OpcPm02_5 + sc.OpcPm10_0) != 0 {
		if sc.OpcCsecs != value.OpcCsecs || sc.OpcPm01_0 != value.OpcPm01_0 || sc.OpcPm02_5 != value.OpcPm02_5 || sc.OpcPm10_0 != value.OpcPm10_0 {
			ChangedOpc = true
		}
		value.OpcPm01_0 = sc.OpcPm01_0
		value.OpcPm02_5 = sc.OpcPm02_5
		value.OpcPm10_0 = sc.OpcPm10_0
		value.OpcC00_38 = sc.OpcC00_38
		value.OpcC00_54 = sc.OpcC00_54
		value.OpcC01_00 = sc.OpcC01_00
		value.OpcC02_10 = sc.OpcC02_10
		value.OpcC05_00 = sc.OpcC05_00
		value.OpcC10_00 = sc.OpcC10_00
		value.OpcCsecs = sc.OpcCsecs
	}
	if sc.Cpm0 != 0 {
		if sc.Cpm0 != value.Cpm0 {
			ChangedGeiger = true
		}
		value.Cpm0 = sc.Cpm0
	}
	if sc.Cpm1 != 0 {
		if sc.Cpm1 != value.Cpm1 {
			ChangedGeiger = true
		}
		value.Cpm1 = sc.Cpm1
	}
	if sc.Transport != "" {
		if sc.Transport != value.Transport {
			ChangedTransport = true
		}
		value.Transport = sc.Transport
	}
	if sc.StatsUptimeMinutes != 0 {
		value.StatsUptimeMinutes = sc.StatsUptimeMinutes
	}
	if sc.StatsAppVersion != "" {
		value.StatsAppVersion = sc.StatsAppVersion
	}
	if sc.StatsDeviceParams != "" {
		value.StatsDeviceParams = sc.StatsDeviceParams
	}
	if sc.StatsTransmittedBytes != 0 {
		value.StatsTransmittedBytes = sc.StatsTransmittedBytes
	}
	if sc.StatsReceivedBytes != 0 {
		value.StatsReceivedBytes = sc.StatsReceivedBytes
	}
	if sc.StatsCommsResets != 0 {
		value.StatsCommsResets = sc.StatsCommsResets
	}
	if sc.StatsCommsFails != 0 {
		value.StatsCommsFails = sc.StatsCommsFails
	}
	if sc.StatsCommsPowerFails != 0 {
		value.StatsCommsPowerFails = sc.StatsCommsPowerFails
	}
	if sc.StatsDeviceRestarts != 0 {
		value.StatsDeviceRestarts = sc.StatsDeviceRestarts
	}
	if sc.StatsMotiondrops != 0 {
		value.StatsMotiondrops = sc.StatsMotiondrops
	}
	if sc.StatsOneshots != 0 {
		value.StatsOneshots = sc.StatsOneshots
	}
	if sc.StatsOneshotSeconds != 0 {
		value.StatsOneshotSeconds = sc.StatsOneshotSeconds
	}
	if sc.StatsIccid != "" {
		value.StatsIccid = sc.StatsIccid
	}
	if sc.StatsCpsi != "" {
		value.StatsCpsi = sc.StatsCpsi
	}
	if sc.StatsDfu != "" {
		value.StatsDfu = sc.StatsDfu
	}
	if sc.StatsDeviceInfo != "" {
		value.StatsDeviceInfo = sc.StatsDeviceInfo
	}
	if sc.StatsFreeMem != 0 {
		value.StatsFreeMem = sc.StatsFreeMem
	}
	if sc.StatsNTPCount != 0 {
		value.StatsNTPCount = sc.StatsNTPCount
	}
	if sc.StatsLastFailure != "" {
		value.StatsLastFailure = sc.StatsLastFailure
	}
	if sc.StatsStatus != "" {
		value.StatsStatus = sc.StatsStatus
	}
	if sc.Message != "" {
		value.Message = sc.Message
	}

	// Shuffle
	if ChangedLocation {
		for i:=len(value.LocationHistory)-1; i>0; i-- {
			value.LocationHistory[i] = value.LocationHistory[i-1]
		}
	    new := SafecastDataV2{}
		new.CapturedAt = value.CapturedAt
		new.Latitude = value.Latitude
		new.Longitude = value.Longitude
		new.Height = value.Height
		value.LocationHistory[0] = new
	}

	// Shuffle
	if ChangedPms {
		for i:=len(value.PmsHistory)-1; i>0; i-- {
			value.PmsHistory[i] = value.PmsHistory[i-1]
		}
	    new := SafecastDataV2{}
		new.CapturedAt = value.CapturedAt
		new.PmsPm01_0 = value.PmsPm01_0
		new.PmsPm02_5 = value.PmsPm02_5
		new.PmsPm10_0 = value.PmsPm10_0
		new.PmsC00_30 = value.PmsC00_30
		new.PmsC00_50 = value.PmsC00_50
		new.PmsC01_00 = value.PmsC01_00
		new.PmsC02_50 = value.PmsC02_50
		new.PmsC05_00 = value.PmsC05_00
		new.PmsC10_00 = value.PmsC10_00
		new.PmsCsecs = value.PmsCsecs
		value.PmsHistory[0] = new
	}

	// Shuffle
	if ChangedOpc {
		for i:=len(value.OpcHistory)-1; i>0; i-- {
			value.OpcHistory[i] = value.OpcHistory[i-1]
		}
	    new := SafecastDataV2{}
		new.CapturedAt = value.CapturedAt
		new.OpcPm01_0 = value.OpcPm01_0
		new.OpcPm02_5 = value.OpcPm02_5
		new.OpcPm10_0 = value.OpcPm10_0
		new.OpcC00_38 = value.OpcC00_38
		new.OpcC00_54 = value.OpcC00_54
		new.OpcC01_00 = value.OpcC01_00
		new.OpcC02_10 = value.OpcC02_10
		new.OpcC05_00 = value.OpcC05_00
		new.OpcC10_00 = value.OpcC10_00
		new.OpcCsecs = value.OpcCsecs
		value.OpcHistory[0] = new
	}

	// Shuffle
	if ChangedGeiger {
		for i:=len(value.GeigerHistory)-1; i>0; i-- {
			value.GeigerHistory[i] = value.GeigerHistory[i-1]
		}
	    new := SafecastDataV2{}
		new.CapturedAt = value.CapturedAt
		new.Cpm0 = value.Cpm0
		new.Cpm1 = value.Cpm1
		value.GeigerHistory[0] = new
	}

	// Shuffle
	if ChangedTransport {
		for i:=len(value.TransportHistory)-1; i>0; i-- {
			value.TransportHistory[i] = value.TransportHistory[i-1]
		}
	    new := SafecastDataV2{}
		new.CapturedAt = value.CapturedAt
		new.Transport = value.Transport
		value.TransportHistory[0] = new
	}

	// Write it to the file
    valueJSON, _ := json.MarshalIndent(value, "", "    ")
    fd, err := os.OpenFile(filename, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0666)
    if err == nil {
	    fd.WriteString(string(valueJSON));
	    fd.Close();
	}
	
}

// Get summary of a device
func SafecastGetSummary(DeviceID uint32) string {
	
	// Generate the filename, which we'll use twice
    filename := SafecastDirectory() + TTServerValuePath + "/" + fmt.Sprintf("%d", DeviceID) + ".json"

    // Read the file if it exists, else blank out value
    value := SafecastValue{}
    file, err := ioutil.ReadFile(filename)
    if err != nil {
		return ""
	}
	
    // Read it as JSON
    err = json.Unmarshal(file, &value)
    if err != nil {
		return ""
	}

	// Build the summary
	s := ""
	
	if value.StatsDeviceInfo != "" {
		s += " " + value.StatsDeviceInfo
	}
	if value.BatVoltage != 0 {
		s += fmt.Sprintf(" %.2fv", value.BatVoltage)
	}
	if value.Cpm0 != 0 {
		s += fmt.Sprintf(" %.0fcpm", value.Cpm0)
	}
	if value.Cpm1 != 0 {
		s += fmt.Sprintf(" %.0fcpm", value.Cpm1)
	}
    if (float32(value.OpcCsecs) + value.OpcPm01_0 + value.OpcPm02_5 + value.OpcPm10_0) != 0   {
		s += fmt.Sprintf(" %.2f/%.2f/%.2f", value.OpcPm01_0, value.OpcPm02_5, value.OpcPm10_0)
	} else if (float32(value.PmsCsecs) + value.PmsPm01_0 + value.PmsPm02_5 + value.PmsPm10_0) != 0 {
		s += fmt.Sprintf(" %.0f/%.0f/%.0f", value.PmsPm01_0, value.PmsPm02_5, value.PmsPm10_0)
	}
	if value.Latitude >= 2 {
        s += fmt.Sprintf(" <http://maps.google.com/maps?z=12&t=m&q=loc:%f+%f|gps>", value.Latitude, value.Longitude)
	}
	
	if (s == "") {
		return ""
	}

	str := "(" + s + " )"

	return str
}
