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

// Warning behavior
const deviceWarningAfterMinutes = 90

// Debug
const debugFormatConversions = false

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

// The data structure for the "Value" files
type SafecastValue struct {
    SafecastData            `json:"current_values,omitempty"`
    LocationHistory         [5]SafecastData `json:"location_history,omitempty"`
    GeigerHistory           [5]SafecastData `json:"geiger_history,omitempty"`
    OpcHistory              [5]SafecastData `json:"opc_history,omitempty"`
    PmsHistory              [5]SafecastData `json:"pms_history,omitempty"`
    IPInfo                  IPInfoData      `json:"transport_ip_info,omitempty"`
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
    if a[i].deviceid < a[j].deviceid {
        return true
    } else if a[i].deviceid > a[j].deviceid {
        return false
    }
    return false
}

// Process an inbound Safecast message, as an asynchronous goroutine
func ProcessSafecastMessage(SeqNo int, msg teletype.Telecast, checksum uint32, UploadedAt string, Transport string) {

    // To ensure a best-efforts sequencing in log, impose a delay in proportion to sequencing
    if SeqNo != 0 {
        time.Sleep(time.Duration(SeqNo) * time.Minute)
    }

    // Discard it if it's a duplicate
    if isDuplicate(checksum) {
        fmt.Printf("%s DISCARDING duplicate message\n", time.Now().Format(logDateFormat));
        return
    }

    // Process stamps by adding or removing fields from the message
    if (!stampSetOrApply(&msg)) {
        fmt.Printf("%s DISCARDING un-stampable message\n", time.Now().Format(logDateFormat));
        return
    }

    // This is the ONLY required field
    if msg.DeviceID == nil {
        fmt.Printf("%s DISCARDING message with no DeviceID\n", time.Now().Format(logDateFormat));
        return
    }

    // Generate the fields common to all uploads to safecast
    sd := SafecastData{}

    sd.DeviceID = uint64(msg.GetDeviceID())

    // CapturedAt
    if msg.CapturedAt != nil {
        sd.CapturedAt = msg.CapturedAt
    } else if msg.CapturedAtDate != nil && msg.CapturedAtTime != nil {
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
        sd.CapturedAt = &tstr
    }

    // Loc
    if msg.Latitude != nil && msg.Longitude != nil {
        var loc Loc
        loc.Lat = msg.GetLatitude()
        loc.Lon = msg.GetLongitude()
        if msg.Altitude != nil {
            var alt float32
            alt = float32(msg.GetAltitude())
            loc.Alt = &alt
        }
        sd.Loc = &loc
    }

    // Dev
    var dev Dev
    var dodev = false

    if msg.StatsUptimeMinutes != nil {
        mins := msg.GetStatsUptimeMinutes()
        if msg.StatsUptimeDays != nil {
            mins += msg.GetStatsUptimeDays() * 24 * 60
        }
        dev.UptimeMinutes = &mins
        dodev = true
    }
    if msg.StatsAppVersion != nil {
        dev.AppVersion = msg.StatsAppVersion
        dodev = true
    }
    if msg.StatsDeviceParams != nil {
        dev.DeviceParams = msg.StatsDeviceParams
        dodev = true
    }
    if msg.StatsTransmittedBytes != nil {
        dev.TransmittedBytes = msg.StatsTransmittedBytes
        dodev = true
    }
    if msg.StatsReceivedBytes != nil {
        dev.ReceivedBytes = msg.StatsReceivedBytes
        dodev = true
    }
    if msg.StatsCommsResets != nil {
        dev.CommsResets = msg.StatsCommsResets
        dodev = true
    }
    if msg.StatsCommsPowerFails != nil {
        dev.CommsPowerFails = msg.StatsCommsPowerFails
        dodev = true
    }
    if msg.StatsOneshots != nil {
        dev.Oneshots = msg.StatsOneshots
        dodev = true
    }
    if msg.StatsOneshotSeconds != nil {
        dev.OneshotSeconds = msg.StatsOneshotSeconds
        dodev = true
    }
    if msg.StatsMotiondrops != nil {
        dev.Motiondrops = msg.StatsMotiondrops
        dodev = true
    }
    if msg.StatsIccid != nil {
        dev.Iccid = msg.StatsIccid
        dodev = true
    }
    if msg.StatsCpsi != nil {
        dev.Cpsi = msg.StatsCpsi
        dodev = true
    }
    if msg.StatsDfu != nil {
        dev.Dfu = msg.StatsDfu
        dodev = true
    }
    if msg.StatsDeviceLabel != nil {
        dev.DeviceLabel = msg.StatsDeviceLabel
        dodev = true
    }
    if msg.StatsGpsParams != nil {
        dev.GpsParams = msg.StatsGpsParams
        dodev = true
    }
    if msg.StatsServiceParams != nil {
        dev.ServiceParams = msg.StatsServiceParams
        dodev = true
    }
    if msg.StatsTtnParams != nil {
        dev.TtnParams = msg.StatsTtnParams
        dodev = true
    }
    if msg.StatsSensorParams != nil {
        dev.SensorParams = msg.StatsSensorParams
        dodev = true
    }
    if dodev {
        sd.Dev = &dev
    }

    // Bat
    var bat Bat
    var dobat = false

    if msg.BatteryVoltage != nil {
        bat.Voltage = msg.BatteryVoltage
        dobat = true
    }
    if msg.BatterySOC != nil {
        bat.Charge = msg.BatterySOC
        dobat = true;
    }
    if msg.BatteryCurrent != nil {
        bat.Current = msg.BatteryCurrent
        dobat = true;
    }

    if dobat {
        sd.Bat = &bat
    }

    // Env
    var env Env
    var doenv = false

    if msg.EnvTemperature != nil {
        env.Temp = msg.EnvTemperature
        doenv = true
    }
    if msg.EnvHumidity != nil {
        env.Humid = msg.EnvHumidity
    }
    if msg.EnvPressure != nil {
        env.Press = msg.EnvPressure
    }

    if doenv {
        sd.Env = &env
    }

    // Net
    var net Net
    var donet = false

    if Transport != "" {
        net.Transport = &Transport
        donet = true
    }
    if msg.WirelessSNR != nil {
        net.SNR = msg.WirelessSNR
        donet = true
    }

    if donet {
        sd.Net = &net
    }

    // Pms
    var pms Pms
    var dopms = false

    if msg.PmsPm01_0 != nil && msg.PmsPm02_5 != nil && msg.PmsPm10_0 != nil {
        Pm01_0 := float32(msg.GetPmsPm01_0())
        pms.Pm01_0 = &Pm01_0
        Pm02_5 := float32(msg.GetPmsPm02_5())
        pms.Pm02_5 = &Pm02_5
        Pm10_0 := float32(msg.GetPmsPm10_0())
        pms.Pm10_0 = &Pm10_0
        dopms = true
    }

    if dopms {
        if msg.PmsC00_30 != nil {
            pms.Count00_30 = msg.PmsC00_30
        }
        if msg.PmsC00_50 != nil {
            pms.Count00_50 = msg.PmsC00_50
        }
        if msg.PmsC01_00 != nil {
            pms.Count01_00 = msg.PmsC01_00
        }
        if msg.PmsC02_50 != nil {
            pms.Count02_50 = msg.PmsC02_50
        }
        if msg.PmsC05_00 != nil {
            pms.Count05_00 = msg.PmsC05_00
        }
        if msg.PmsC10_00 != nil {
            pms.Count10_00 = msg.PmsC10_00
        }
        if msg.PmsCsecs != nil {
            pms.CountSecs = msg.PmsCsecs
        }
    }

    if dopms {
        sd.Pms = &pms
    }

    // Opc
    var opc Opc
    var doopc = false

    if msg.OpcPm01_0 != nil && msg.OpcPm02_5 != nil && msg.OpcPm10_0 != nil {
        opc.Pm01_0 = msg.OpcPm01_0
        opc.Pm02_5 = msg.OpcPm02_5
        opc.Pm10_0 = msg.OpcPm10_0
        doopc = true
    }

    if doopc {
        if msg.OpcC00_38 != nil {
            opc.Count00_38 = msg.OpcC00_38
        }
        if msg.OpcC00_54 != nil {
            opc.Count00_54 = msg.OpcC00_54
        }
        if msg.OpcC01_00 != nil {
            opc.Count01_00 = msg.OpcC01_00
        }
        if msg.OpcC02_10 != nil {
            opc.Count02_10 = msg.OpcC02_10
        }
        if msg.OpcC05_00 != nil {
            opc.Count05_00 = msg.OpcC05_00
        }
        if msg.OpcC10_00 != nil {
            opc.Count10_00 = msg.OpcC10_00
        }
        if msg.OpcCsecs != nil {
            opc.CountSecs = msg.OpcCsecs
        }
    }

    if doopc {
        sd.Opc = &opc
    }

    // Lnd, assuming a pair of 7318s
    var lnd Lnd
    var dolnd = false

    if msg.Lnd_7318U != nil {
        var cpm float32 = float32(msg.GetLnd_7318U())
        lnd.U7318 = &cpm
        dolnd = true
    }
    if msg.Lnd_7318C != nil {
        var cpm float32 = float32(msg.GetLnd_7318C())
        lnd.C7318 = &cpm
        dolnd = true
    }
    if msg.Lnd_7128Ec != nil {
        var cpm float32 = float32(msg.GetLnd_7128Ec())
        lnd.EC7128 = &cpm
        dolnd = true
    }

    if dolnd {
        sd.Lnd = &lnd
    }

    // Log as accurately as we can with regard to what came in
    SafecastWriteToLogs(UploadedAt, sd)

    // Upload
    SafecastUpload(UploadedAt, sd)

}

// Begin transaction and return the transaction ID
func beginTransaction(version string,  message1 string, message2 string) int {
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

// Update the list of seen devices
func trackAllDevices() {

    // Loop over the file system, tracking all devices
    files, err := ioutil.ReadDir(SafecastDirectory() + TTServerValuePath)
    if err == nil {

        // Iterate over each of the values
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
}

// Send a hello to devices that have never reported full stats
func sendHelloToNewDevices() {

    // Make sure the list of devices is up to date
    trackAllDevices()

    // Sweep through all devices that we've seen
    for i := 0; i < len(seenDevices); i++ {

        // Get the device ID
        deviceID := seenDevices[i].deviceid

        // Only do this for Safecast devices, as opposed to pointcast or air
        if deviceID > 1048576 {

            // Read the current value, or a blank value structure if it's blank
            _, value := SafecastReadValue(deviceID)

            // Check to see if it has a required stats field
            if value.Dev == nil || value.Dev.AppVersion == nil {

                // See if there's a pending command already waiting for this device
                isValid, _ := getCommand(deviceID)
                if !isValid {

                    sendToSafecastOps(fmt.Sprintf("** NOTE ** Sending hello to newly-detected device %d", deviceID))
                    sendCommand("New device detected", deviceID, "hello")

                }

            }
        }
    }

}

// Update message ages and notify
func sendExpiredSafecastDevicesToSlack() {

    // Update the in-memory list of seen devices
    trackAllDevices()

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
func sendSafecastDeviceSummaryToSlack(fWrap bool, fDetails bool) {

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

        label := ""
        gps := ""
        summary := ""
        if fDetails {
            label, gps, summary = SafecastGetSummary(id)
        }

        s += fmt.Sprintf("<http://%s%s%d|%010d> ", TTServerHTTPAddress, TTServerTopicValue, id, id)

        if fWrap {
            if label != "" {
                s += label
            }
            s += "\n        "
        }

        s += fmt.Sprintf("<http://%s%s%s%d.json|log> ", TTServerHTTPAddress, TTServerTopicLog, time.Now().UTC().Format("2006-01-"), id)
        s += fmt.Sprintf("<http://%s%s%s%d.csv|csv>", TTServerHTTPAddress, TTServerTopicLog, time.Now().UTC().Format("2006-01-"), id)

        if fDetails {
            if gps != "" {
                s += " " + gps
            } else {
                s += " gps"
            }
        }

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
                s = fmt.Sprintf("%s %02dm ago", s, minutesAgo)
            }
        }

        if summary != "" {
            if (fWrap) {
                s += "\n        "
            } else {
                s += " ( "
            }
            if summary != "" {
                s += summary
            }
            if (!fWrap) {
                if label != "" {
                    s += "\"" + label + "\" "
                }
                s += ")"
            }
        }

    }

    // Send it to Slack
    sendToSafecastOps(s)

}

// Write to both logs
func SafecastWriteToLogs(UploadedAt string, sd SafecastData) {
    SafecastJSONLog(UploadedAt, sd)
    SafecastCSVLog(UploadedAt, sd)
    SafecastWriteValue(UploadedAt, sd)
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
func SafecastCSVLog(UploadedAt string, sd SafecastData) {

    // Extract the device number and form a filename
    file := SafecastLogFilename(fmt.Sprintf("%d", sd.DeviceID), ".csv")

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
        fd.WriteString("Uploaded,Captured,Device ID,Stats,Uptime,7318U,7318C,7128EC,Latitude,Longitude,Altitude,Bat V,Bat SOC,Bat I,SNR,Temp C,Humid %,Press Pa,PMS PM 1.0,PMS PM 2.5,PMS PM 10.0,PMS # 0.3,PMS # 0.5,PMS # 1.0,PMS # 2.5,PMS # 5.0,PMS # 10.0,PMS # Secs,OPC PM 1.0,OPC PM 2.5,OPC PM 10.0,OPC # 0.38,OPC # 0.54,OPC # 1.0,OPC # 2.1,OPC # 5.0,OPC # 10.0,OPC # Secs\r\n");

    }

    // Turn stats into a safe string for CSV
    stats := ""
    if sd.Dev != nil {
        if sd.Dev.UptimeMinutes != nil {
            stats += fmt.Sprintf("Uptime:%d ", *sd.Dev.UptimeMinutes)
        }
        if sd.Dev.AppVersion != nil {
            stats += fmt.Sprintf("AppVersion:%s ", *sd.Dev.AppVersion)
        }
        if sd.Dev.DeviceParams != nil {
            stats += fmt.Sprintf("DevParams:%s ", *sd.Dev.DeviceParams)
        }
        if sd.Dev.GpsParams != nil {
            stats += fmt.Sprintf("GpsParams:%s ", *sd.Dev.GpsParams)
        }
        if sd.Dev.ServiceParams != nil {
            stats += fmt.Sprintf("ServiceParams:%s ", *sd.Dev.ServiceParams)
        }
        if sd.Dev.TtnParams != nil {
            stats += fmt.Sprintf("TtnParams:%s ", *sd.Dev.TtnParams)
        }
        if sd.Dev.SensorParams != nil {
            stats += fmt.Sprintf("SensorParams:%s ", *sd.Dev.SensorParams)
        }
        if sd.Dev.TransmittedBytes != nil {
            stats += fmt.Sprintf("Sent:%d ", *sd.Dev.TransmittedBytes)
        }
        if sd.Dev.ReceivedBytes != nil {
            stats += fmt.Sprintf("Rcvd:%d ", *sd.Dev.ReceivedBytes)
        }
        if sd.Dev.CommsResets != nil {
            stats += fmt.Sprintf("CommsResets:%d ", *sd.Dev.CommsResets)
        }
        if sd.Dev.CommsFails != nil {
            stats += fmt.Sprintf("CommsFails:%d ", *sd.Dev.CommsFails)
        }
        if sd.Dev.CommsPowerFails != nil {
            stats += fmt.Sprintf("CommsPowerFails:%d ", *sd.Dev.CommsPowerFails)
        }
        if sd.Dev.DeviceRestarts != nil {
            stats += fmt.Sprintf("Restarts:%d ", *sd.Dev.DeviceRestarts)
        }
        if sd.Dev.Motiondrops != nil {
            stats += fmt.Sprintf("Motiondrops:%d ", *sd.Dev.Motiondrops)
        }
        if sd.Dev.Oneshots != nil {
            stats += fmt.Sprintf("Oneshots:%d ", *sd.Dev.Oneshots)
        }
        if sd.Dev.OneshotSeconds != nil {
            stats += fmt.Sprintf("OneshotSecs:%d ", *sd.Dev.OneshotSeconds)
        }
        if sd.Dev.Iccid != nil {
            stats += fmt.Sprintf("Iccid:%s ", *sd.Dev.Iccid)
        }
        if sd.Dev.Cpsi != nil {
            stats += fmt.Sprintf("Cpsi:%s ", *sd.Dev.Cpsi)
        }
        if sd.Dev.Dfu != nil {
            stats += fmt.Sprintf("DFU:%s ", *sd.Dev.Dfu)
        }
        if sd.Dev.DeviceLabel != nil {
            stats += fmt.Sprintf("Label:%s ", *sd.Dev.DeviceLabel)
        }
        if sd.Dev.FreeMem != nil {
            stats += fmt.Sprintf("FreeMem:%d ", *sd.Dev.FreeMem)
        }
        if sd.Dev.NTPCount != nil {
            stats += fmt.Sprintf("NTPCount:%d ", *sd.Dev.NTPCount)
        }
        if sd.Dev.LastFailure != nil {
            stats += fmt.Sprintf("LastFailure:%s ", *sd.Dev.LastFailure)
        }
        if sd.Dev.Status != nil {
            stats += fmt.Sprintf("Status:%s ", *sd.Dev.Status)
        }
    }

    // Write the stuff
    s := ""

    // Convert the times to something that can be parsed by Excel
    zTime := ""
    if sd.UploadedAt != nil {
        zTime = fmt.Sprintf("%s", *sd.UploadedAt)
    } else if UploadedAt != "" {
        zTime = UploadedAt
    }
    t, err := time.Parse("2006-01-02T15:04:05Z", zTime)
    if err == nil {
        zTime = t.UTC().Format("2006-01-02 15:04:05")
    }
    s += zTime

    s += ","
    if sd.CapturedAt != nil {
        t, err = time.Parse("2006-01-02T15:04:05Z", *sd.CapturedAt)
        if err == nil {
            s += t.UTC().Format("2006-01-02 15:04:05")
        } else {
            s += *sd.CapturedAt
        }
    }

    s = s + fmt.Sprintf(",%d", sd.DeviceID)
    s = s + fmt.Sprintf(",%s", stats)
    s = s + fmt.Sprintf(",%s", "")          // Value
    if sd.Lnd == nil {
        s += ",,,"
    } else {
        if sd.U7318 != nil {
            s = s + fmt.Sprintf(",%f", *sd.U7318)
        } else {
            s += ","
        }
        if sd.C7318 != nil {
            s = s + fmt.Sprintf(",%f", *sd.C7318)
        } else {
            s += ","
        }
        if sd.EC7128 != nil {
            s = s + fmt.Sprintf(",%f", *sd.EC7128)
        } else {
            s += ","
        }
    }
    if sd.Loc == nil {
        s += ",,,"
    } else {
        s = s + fmt.Sprintf(",%f", sd.Loc.Lat)
        s = s + fmt.Sprintf(",%f", sd.Loc.Lon)
        if sd.Loc.Alt != nil {
            s = s + fmt.Sprintf(",%f", *sd.Loc.Alt)
        } else {
            s += ","
        }
    }
    if sd.Bat == nil {
        s += ",,,"
    } else {
        if sd.Bat.Voltage == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Bat.Voltage)
        }
        if sd.Bat.Charge == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Bat.Charge)
        }
        if sd.Bat.Current == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Bat.Current)
        }
    }
    if sd.Net == nil {
        s += ","
    } else {
        if sd.Net.SNR == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Net.SNR)
        }
    }
    if sd.Env == nil {
        s += ",,,"
    } else {
        if sd.Env.Temp == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Env.Temp)
        }
        if sd.Env.Humid == nil {
            s += ","
        } else {
            s = s + fmt.Sprintf(",%f", *sd.Env.Humid)
        }
        if sd.Env.Press == nil {
            s += ","
        } else {
            s = s + fmt.Sprintf(",%f", *sd.Env.Press)
        }
    }
    if sd.Pms == nil {
        s += ",,,,,,,,,,"
    } else {
        if sd.Pms.Pm01_0 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Pms.Pm01_0)
        }
        if sd.Pms.Pm02_5 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Pms.Pm02_5)
        }
        if sd.Pms.Pm10_0 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Pms.Pm10_0)
        }
        if sd.Pms.Count00_30 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Pms.Count00_30)
        }
        if sd.Pms.Count00_50 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Pms.Count00_50)
        }
        if sd.Pms.Count01_00 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Pms.Count01_00)
        }
        if sd.Pms.Count02_50 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Pms.Count02_50)
        }
        if sd.Pms.Count05_00 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Pms.Count05_00)
        }
        if sd.Pms.Count10_00 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Pms.Count10_00)
        }
        if sd.Pms.CountSecs == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Pms.CountSecs)
        }
    }
    if sd.Opc == nil {
        s += ",,,,,,,,,,"
    } else {
        if sd.Opc.Pm01_0 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Opc.Pm01_0)
        }
        if sd.Opc.Pm02_5 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Opc.Pm02_5)
        }
        if sd.Opc.Pm10_0 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Opc.Pm10_0)
        }
        if sd.Opc.Count00_38 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Opc.Count00_38)
        }
        if sd.Opc.Count00_54 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Opc.Count00_54)
        }
        if sd.Opc.Count01_00 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Opc.Count01_00)
        }
        if sd.Opc.Count02_10 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Opc.Count02_10)
        }
        if sd.Opc.Count05_00 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Opc.Count05_00)
        }
        if sd.Opc.Count10_00 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Opc.Count10_00)
        }
        if sd.Opc.CountSecs == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Opc.CountSecs)
        }
    }
    s = s + "\r\n"

    fd.WriteString(s);

    // Close and exit
    fd.Close();

}

// Write the value to the log
func SafecastJSONLog(UploadedAt string, sd SafecastData) {

    file := SafecastLogFilename(fmt.Sprintf("%d", sd.DeviceID), ".json")

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
    sd.UploadedAt = &UploadedAt
    scJSON, _ := json.Marshal(sd)
    fd.WriteString(string(scJSON));
    fd.WriteString("\r\n,\r\n");

    // Debug
    if debugFormatConversions {
        fmt.Printf("*** About to log:\n%s\n", string(scJSON))
    }

    // Close and exit
    fd.Close();

}

// Reformat a special V1 payload to Current
func SafecastReformat(v1 SafecastDataV1) (deviceid uint32, devtype string, data SafecastData) {
    var sd SafecastData
    var devicetype = ""

    // Required field
    if v1.DeviceID == nil {
        fmt.Sprintf("*** Reformat: Missing Device ID\n");
        return 0, "", sd
    }

    // Detect what range it is within, and process the conversion differently
    isPointcast := false
    if (*v1.DeviceID >= 100000 && *v1.DeviceID < 199999) {
        isPointcast = true
        devicetype = "pointcast"
        sd.DeviceID = uint64(*v1.DeviceID / 10)
    }
    isSafecastAir := false
    if (*v1.DeviceID >= 50000 && *v1.DeviceID < 59999) {
        isSafecastAir = true
        devicetype = "safecast-air"
        sd.DeviceID = uint64(*v1.DeviceID)
    }
    if !isPointcast && !isSafecastAir {
        fmt.Sprintf("*** Reformat: unsuccessful attempt to reformat Device ID %d\n", *v1.DeviceID);
        return 0, "", sd
    }

    // Captured
    if v1.CapturedAt != nil {
        sd.CapturedAt = v1.CapturedAt
    }

    // Loc
    if v1.Latitude != nil && v1.Longitude != nil {
        var loc Loc
        loc.Lat = *v1.Latitude
        loc.Lon = *v1.Longitude
        if v1.Height != nil {
            alt := float32(*v1.Height)
            loc.Alt = &alt
        }
        sd.Loc = &loc
    }

    // Reverse-engineer Unit/Value to yield the good stuff
    if v1.Unit != nil && v1.Value != nil {

        switch (strings.ToLower(*v1.Unit)) {

        case "pm1":
            var opc Opc
            pm := *v1.Value
            opc.Pm01_0 = &pm
            sd.Opc = &opc

        case "pm2.5":
            var opc Opc
            pm := *v1.Value
            opc.Pm02_5 = &pm
            sd.Opc = &opc

        case "pm10":
            var opc Opc
            pm := *v1.Value
            opc.Pm10_0 = &pm
            sd.Opc = &opc

        case "humd%":
            var env Env
            humid := *v1.Value
            env.Humid = &humid
            sd.Env = &env

        case "tempc":
            var env Env
            temp := *v1.Value
            env.Temp = &temp
            sd.Env = &env

        case "cpm":
            if !isPointcast {
                fmt.Sprintf("*** Reformat: Received CPM for non-Pointcast\n", sd.DeviceID)
            } else {
                if (*v1.DeviceID % 10) == 1 {
                    var lnd Lnd
                    cpm := *v1.Value
                    lnd.U7318 = &cpm
                    sd.Lnd = &lnd

                } else if (*v1.DeviceID % 10) == 2 {
                    var lnd Lnd
                    cpm := *v1.Value
                    lnd.EC7128 = &cpm
                    sd.Lnd = &lnd
                } else {
                    fmt.Sprintf("*** Reformat: %d cpm not understood for this subtype\n", sd.DeviceID);
                }
            }
        case "status":
            // The value is the temp
            var env Env
            TempC := *v1.Value
            env.Temp = &TempC
            sd.Env = &env

            // Parse subfields
            var bat Bat
            var dobat = false
            var dev Dev
            var dodev = false

            unrecognized := ""
			status := ""
			if v1.DeviceTypeID != nil {
	            status = *v1.DeviceTypeID
			}
            fields := strings.Split(status, ",")
            for v := range fields {
                field := strings.Split(fields[v], ":")
                switch (field[0]) {
                case "Battery Voltage":
                    f64, _ := strconv.ParseFloat(field[1], 32)
                    f32 := float32(f64)
                    bat.Voltage = &f32
                    dobat = true
                case "Fails":
                    u64, _ := strconv.ParseUint(field[1], 10, 32)
                    u32 := uint32(u64)
                    dev.CommsFails = &u32
                    dodev = true
                case "Restarts":
                    u64, _ := strconv.ParseUint(field[1], 10, 32)
                    u32 := uint32(u64)
                    dev.DeviceRestarts = &u32
                    dodev = true
                case "FreeRam":
                    u64, _ := strconv.ParseUint(field[1], 10, 32)
                    u32 := uint32(u64)
                    dev.FreeMem = &u32
                    dodev = true
                case "NTP count":
                    u64, _ := strconv.ParseUint(field[1], 10, 32)
                    u32 := uint32(u64)
                    dev.NTPCount = &u32
                    dodev = true
                case "Last failure":
                    var LastFailure string = field[1]
                    dev.LastFailure = &LastFailure
                    dodev = true
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
                dev.Status = &unrecognized
                dodev = true
            }

            // Include in  the uploads
            if dobat {
                sd.Bat = &bat
            }
            if dodev {
                sd.Dev = &dev
            }

        default:
            fmt.Sprintf("*** Reformat Warning ***\n*** %s id=%d Unit %s = Value %f UNRECOGNIZED\n", devicetype, *v1.DeviceID, *v1.Unit, *v1.Value)

        }
    }

    return uint32(sd.DeviceID), devicetype, sd

}

// Upload a Safecast data structure to the Safecast service
func SafecastV1Upload(body []byte, url string, unit string, value string) bool {

    transaction := beginTransaction("V1", unit, value)

    req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
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
func SafecastUpload(UploadedAt string, sd SafecastData) bool {

    // Upload to all URLs
    for _, url := range SafecastUploadURLs {
        go doUploadToSafecast(UploadedAt, sd, url)
    }

    return true
}

// Upload a Safecast data structure to the Safecast service
func doUploadToSafecast(UploadedAt string, sd SafecastData, url string) bool {

    var CapturedAt string = ""
    if sd.CapturedAt != nil {
        CapturedAt = *sd.CapturedAt
    }
    transaction := beginTransaction("V2", "captured", CapturedAt)

    sd.UploadedAt = &UploadedAt
    scJSON, _ := json.Marshal(sd)

    if (false) {
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

// Get the current value
func SafecastReadValue(deviceID uint32) (isAvail bool, sv SafecastValue) {
    value := SafecastValue{}

    // Generate the filename, which we'll use twice
    filename := SafecastDirectory() + TTServerValuePath + "/" + fmt.Sprintf("%d", deviceID) + ".json"

    // Read the file if it exists
    file, err := ioutil.ReadFile(filename)
    if err != nil {
        value = SafecastValue{}
        value.DeviceID = uint64(deviceID);
        return false, value
    }

    // Read it as JSON
    err = json.Unmarshal(file, &value)
    if err != nil {
        value = SafecastValue{}
        value.DeviceID = uint64(deviceID);
        return false, value
    }

    // Got it
    return true, value

}

// Save the last value in a file
func SafecastWriteValue(UploadedAt string, sc SafecastData) {
    var ChangedLocation = false
    var ChangedPms = false
    var ChangedOpc = false
    var ChangedGeiger = false

    // Use the supplied upload time as our modification time
    sc.UploadedAt = &UploadedAt

    // Read the current value, or a blank value structure if it's blank
    _, value := SafecastReadValue(uint32(sc.DeviceID))

    // Update the current values, but only if modified
    if sc.UploadedAt != nil {
        value.UploadedAt = sc.UploadedAt
    }
    if sc.CapturedAt != nil {
        value.CapturedAt = sc.CapturedAt
    }
    if sc.Bat != nil {
        var bat Bat
        if value.Bat == nil {
            value.Bat = &bat
        }
        if sc.Voltage != nil {
            value.Bat.Voltage = sc.Bat.Voltage
        }
        if sc.Current != nil {
            value.Bat.Current = sc.Bat.Current
        }
        if sc.Charge != nil {
            value.Bat.Charge = sc.Bat.Charge
        }
    }
    if sc.Env != nil {
        var env Env
        if value.Env == nil {
            value.Env = &env
        }
        if sc.Temp != nil {
            value.Env.Temp = sc.Env.Temp
        }
        if sc.Humid != nil {
            value.Env.Humid = sc.Env.Humid
        }
        if sc.Press != nil {
            value.Env.Press = sc.Env.Press
        }
    }
    if sc.Net != nil {
        var net Net
        if value.Net == nil {
            value.Net = &net
        }
        if sc.SNR != nil {
            value.Net.SNR = sc.Net.SNR
        }
        if sc.Transport != nil {
            value.Net.Transport = sc.Net.Transport
        }
    }
    if sc.Loc != nil {
        var loc Loc
        if (value.Loc == nil) {
            value.Loc = &loc
        }
        if (value.Loc.Lat != sc.Loc.Lat || value.Loc.Lon != sc.Loc.Lon) {
            ChangedLocation = true
        }
        value.Loc = sc.Loc
    }
    if sc.Pms != nil {
        var pms Pms
        if (value.Pms == nil) {
            value.Pms = &pms
        }
        if sc.Pms.Pm01_0 != nil {
            value.Pms.Pm01_0 = sc.Pms.Pm01_0
        }
        if sc.Pms.Pm02_5 != nil {
            value.Pms.Pm02_5 = sc.Pms.Pm02_5
        }
        if sc.Pms.Pm10_0 != nil {
            value.Pms.Pm10_0 = sc.Pms.Pm10_0
        }
        if sc.Pms.CountSecs != nil {
            value.Pms.Count00_30 = sc.Pms.Count00_30
            value.Pms.Count00_50 = sc.Pms.Count00_50
            value.Pms.Count01_00 = sc.Pms.Count01_00
            value.Pms.Count02_50 = sc.Pms.Count02_50
            value.Pms.Count05_00 = sc.Pms.Count05_00
            value.Pms.Count10_00 = sc.Pms.Count10_00
            value.Pms.CountSecs = sc.Pms.CountSecs
        }
        ChangedPms = true
    }
    if sc.Opc != nil {
        var opc Opc
        if (value.Opc == nil) {
            value.Opc = &opc
        }
        if sc.Opc.Pm01_0 != nil {
            value.Opc.Pm01_0 = sc.Opc.Pm01_0
        }
        if sc.Opc.Pm02_5 != nil {
            value.Opc.Pm02_5 = sc.Opc.Pm02_5
        }
        if sc.Opc.Pm10_0 != nil {
            value.Opc.Pm10_0 = sc.Opc.Pm10_0
        }
        if sc.Opc.CountSecs != nil {
            value.Opc.Count00_38 = sc.Opc.Count00_38
            value.Opc.Count00_54 = sc.Opc.Count00_54
            value.Opc.Count01_00 = sc.Opc.Count01_00
            value.Opc.Count02_10 = sc.Opc.Count02_10
            value.Opc.Count05_00 = sc.Opc.Count05_00
            value.Opc.Count10_00 = sc.Opc.Count10_00
            value.Opc.CountSecs = sc.Opc.CountSecs
        }
        ChangedOpc = true
    }
    if sc.Lnd != nil {
        var lnd Lnd
        if value.Lnd == nil {
            value.Lnd = &lnd
        }
        if sc.Lnd.U7318 != nil {
            var val float32
            if value.Lnd.U7318 == nil {
                value.Lnd.U7318 = &val
            }
            if (*value.Lnd.U7318 != *sc.Lnd.U7318) {
                ChangedGeiger = true
            }
            value.Lnd.U7318 = sc.Lnd.U7318
        }
        if sc.Lnd.C7318 != nil {
            var val float32
            if value.Lnd.C7318 == nil {
                value.Lnd.C7318 = &val
            }
            if (*value.Lnd.C7318 != *sc.Lnd.C7318) {
                ChangedGeiger = true
            }
            value.Lnd.C7318 = sc.Lnd.C7318
        }
        if sc.Lnd.EC7128 != nil {
            var val float32
            if value.Lnd.EC7128 == nil {
                value.Lnd.EC7128 = &val
            }
            if (*value.Lnd.EC7128 != *sc.Lnd.EC7128) {
                ChangedGeiger = true
            }
            value.Lnd.EC7128 = sc.Lnd.EC7128
        }
    }
    if sc.Dev != nil {
        var dev Dev
        if value.Dev == nil {
            value.Dev = &dev
        }
        if sc.Dev.UptimeMinutes != nil {
            value.Dev.UptimeMinutes = sc.Dev.UptimeMinutes
        }
        if sc.Dev.AppVersion != nil {
            value.Dev.AppVersion = sc.Dev.AppVersion
        }
        if sc.Dev.DeviceParams != nil {
            value.Dev.DeviceParams = sc.Dev.DeviceParams
        }
        if sc.Dev.GpsParams != nil {
            value.Dev.GpsParams = sc.Dev.GpsParams
        }
        if sc.Dev.ServiceParams != nil {
            value.Dev.ServiceParams = sc.Dev.ServiceParams
        }
        if sc.Dev.TtnParams != nil {
            value.Dev.TtnParams = sc.Dev.TtnParams
        }
        if sc.Dev.SensorParams != nil {
            value.Dev.SensorParams = sc.Dev.SensorParams
        }
        if sc.Dev.TransmittedBytes != nil {
            value.Dev.TransmittedBytes = sc.Dev.TransmittedBytes
        }
        if sc.Dev.ReceivedBytes != nil {
            value.Dev.ReceivedBytes = sc.Dev.ReceivedBytes
        }
        if sc.Dev.CommsResets != nil {
            value.Dev.CommsResets = sc.Dev.CommsResets
        }
        if sc.Dev.CommsFails != nil {
            value.Dev.CommsFails = sc.Dev.CommsFails
        }
        if sc.Dev.CommsPowerFails != nil {
            value.Dev.CommsPowerFails = sc.Dev.CommsPowerFails
        }
        if sc.Dev.DeviceRestarts != nil {
            value.Dev.DeviceRestarts = sc.Dev.DeviceRestarts
        }
        if sc.Dev.Motiondrops != nil {
            value.Dev.Motiondrops = sc.Dev.Motiondrops
        }
        if sc.Dev.Oneshots != nil {
            value.Dev.Oneshots = sc.Dev.Oneshots
        }
        if sc.Dev.OneshotSeconds != nil {
            value.Dev.OneshotSeconds = sc.Dev.OneshotSeconds
        }
        if sc.Dev.Iccid != nil {
            value.Dev.Iccid = sc.Dev.Iccid
        }
        if sc.Dev.Cpsi != nil {
            value.Dev.Cpsi = sc.Dev.Cpsi
        }
        if sc.Dev.Dfu != nil {
            value.Dev.Dfu = sc.Dev.Dfu
        }
        if sc.Dev.DeviceLabel != nil {
            value.Dev.DeviceLabel = sc.Dev.DeviceLabel
        }
        if sc.Dev.FreeMem != nil {
            value.Dev.FreeMem = sc.Dev.FreeMem
        }
        if sc.Dev.NTPCount != nil {
            value.Dev.NTPCount = sc.Dev.NTPCount
        }
        if sc.Dev.LastFailure != nil {
            value.Dev.LastFailure = sc.Dev.LastFailure
        }
        if sc.Dev.Status != nil {
            value.Dev.Status = sc.Dev.Status
        }
    }

    // Calculate a time of the shuffle, allowing for the fact that our preferred time
    // CapturedAt may not be available.
    ShuffledAt := value.UploadedAt
    if value.CapturedAt != nil {
        ShuffledAt = value.CapturedAt
    }

    // Shuffle
    if ChangedLocation {
        for i:=len(value.LocationHistory)-1; i>0; i-- {
            value.LocationHistory[i] = value.LocationHistory[i-1]
        }
        new := SafecastData{}
        new.DeviceID = value.DeviceID
        new.CapturedAt = ShuffledAt
        new.Loc = value.Loc
        value.LocationHistory[0] = new
    }

    // Shuffle
    if ChangedPms {
        for i:=len(value.PmsHistory)-1; i>0; i-- {
            value.PmsHistory[i] = value.PmsHistory[i-1]
        }
        new := SafecastData{}
        new.DeviceID = value.DeviceID
        new.CapturedAt = ShuffledAt
        new.Pms = value.Pms
        value.PmsHistory[0] = new
    }

    // Shuffle
    if ChangedOpc {
        for i:=len(value.OpcHistory)-1; i>0; i-- {
            value.OpcHistory[i] = value.OpcHistory[i-1]
        }
        new := SafecastData{}
        new.DeviceID = value.DeviceID
        new.CapturedAt = ShuffledAt
        new.Opc = value.Opc
        value.OpcHistory[0] = new
    }

    // Shuffle
    if ChangedGeiger {
        for i:=len(value.GeigerHistory)-1; i>0; i-- {
            value.GeigerHistory[i] = value.GeigerHistory[i-1]
        }
        new := SafecastData{}
        new.DeviceID = value.DeviceID
        new.CapturedAt = ShuffledAt
        new.Lnd = value.Lnd
        value.GeigerHistory[0] = new
    }

    // If the current transport has an IP address, try to
    // get the IP info

    if value.Net != nil && value.Net.Transport != nil {
        ipInfo := IPInfoData{}
        Str1 := strings.Split(*value.Net.Transport, ":")
        IP := Str1[len(Str1)-1]
        Str2 := strings.Split(IP, ".")
        isValidIP := len(Str1) > 1 && len(Str2) == 4
        if (isValidIP) {
            response, err := http.Get("http://ip-api.com/json/" + IP)
            if err == nil {
                defer response.Body.Close()
                contents, err := ioutil.ReadAll(response.Body)
                if err == nil {
                    var info IPInfoData
                    err = json.Unmarshal(contents, &info)
                    if err == nil {
                        ipInfo = info
                    }
                }
            }
        }
        value.IPInfo = ipInfo
    }

    // Write it to the file
    filename := SafecastDirectory() + TTServerValuePath + "/" + fmt.Sprintf("%d", sc.DeviceID) + ".json"
    valueJSON, _ := json.MarshalIndent(value, "", "    ")
    fd, err := os.OpenFile(filename, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0666)
    if err == nil {
        fd.WriteString(string(valueJSON));
        fd.Close();
    }

}

// Get summary of a device
func SafecastGetSummary(DeviceID uint32) (Label string, Gps string, Summary string) {

    // Generate the filename, which we'll use twice
    filename := SafecastDirectory() + TTServerValuePath + "/" + fmt.Sprintf("%d", DeviceID) + ".json"

    // Read the file if it exists, else blank out value
    value := SafecastValue{}
    file, err := ioutil.ReadFile(filename)
    if err != nil {
        return "", "", ""
    }

    // Read it as JSON
    err = json.Unmarshal(file, &value)
    if err != nil {
        return "", "", ""
    }

    // Get the label
    label := ""
    if value.Dev != nil && value.Dev.DeviceLabel != nil {
        label = *value.Dev.DeviceLabel
    }

    gps := ""
    if value.Loc != nil {
        gps = fmt.Sprintf("<http://maps.google.com/maps?z=12&t=m&q=loc:%f+%f|gps>", value.Loc.Lat, value.Loc.Lon)
    }

    // Build the summary
    s := ""

    if value.Bat != nil && value.Bat.Voltage != nil {
        s += fmt.Sprintf("%.1fv ", *value.Bat.Voltage)
    }

    if value.Lnd != nil {
        didlnd := false
        if value.Lnd.U7318 != nil {
            s += fmt.Sprintf("%.0f", *value.Lnd.U7318)
            didlnd = true;
        }
        if value.Lnd.C7318 != nil {
            if (didlnd) {
                s += "|"
            }
            s += fmt.Sprintf("%.0f", *value.Lnd.C7318)
            didlnd = true;
        }
        if value.Lnd.EC7128 != nil {
            if (didlnd) {
                s += "|"
            }
            s += fmt.Sprintf("%.0f", *value.Lnd.EC7128)
            didlnd = true;
        }
        if (didlnd) {
            s += "cpm "
        }
    }
    if value.Opc != nil {
        if value.Opc.Pm01_0 != nil && value.Opc.Pm02_5 != nil && value.Opc.Pm10_0 != nil {
            s += fmt.Sprintf("%.1f|%.1f|%.1fug/m3 ", *value.Opc.Pm01_0, *value.Opc.Pm02_5, *value.Opc.Pm10_0)
        }
    } else if value.Pms != nil {
        if value.Pms.Pm01_0 != nil && value.Pms.Pm02_5 != nil && value.Pms.Pm10_0 != nil {
            s += fmt.Sprintf("%.1f|%.1f|%.1fug/m3 ", *value.Pms.Pm01_0, *value.Pms.Pm02_5, *value.Pms.Pm10_0)
        }
    }

    // Done
    return label, gps, s

}
