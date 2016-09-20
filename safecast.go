// Safecast inbound message handling & publishing
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

//////
var testLength = 1
//////

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
        fmt.Printf("%s Safecast message from %s/%s/%s\nn", time.Now().Format(logDateFormat), info.City, info.Region, info.Country)
    } else {
        fmt.Printf("%s Safecast message\n", time.Now().Format(logDateFormat))
    }

    // Log it
    trackDevice(TelecastDeviceID(msg))

    // Determine if the device itself happens to be suppressing "slowly-changing" metadata during this upload.
    // If it is, we ourselves will use this as a signal not to spam the server with other data that we know
    // is also slowly-changing.
    deviceIsSuppressingMetadata :=
        msg.BatteryVoltage == nil && msg.BatterySOC == nil && msg.EnvTemperature == nil && msg.EnvHumidity == nil

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

    // Note that if the telecast message header has a Message
    // field, we will ignore the unit & value and
    // instead override it with the message.  This is
    // the way that we transmit a boot/version message
    // to the Safecast service.

    if msg.Message == nil {

        // A safecast geiger upload
        if msg.Unit == nil {
            sc1.Unit = "cpm"
        } else {
            sc1.Unit = fmt.Sprintf("%s", msg.GetUnit())
        }
        if msg.Value == nil {
            sc1.Value = ""
        } else {
            sc1.Value = fmt.Sprintf("%d", msg.GetValue())
        }

    } else {

        // A text message
        sc1.Unit = "message"
        sc1.Value = msg.GetMessage()
        // We also place it in this field, because we want to retain the
        // full message while Safecast seems to parse the value as
        // a floating point number rather than a string.
        sc1.DeviceTypeID = msg.GetMessage();

    }

	//////////
	test := ""
	for i := 0; i < testLength; i++ {
		test = test + "123456789 "
	}
	testLength++
    sc1.DeviceTypeID = test
	//////////
	
    if !deviceIsSuppressingMetadata {

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

        if msg.PmsTsi_01_0 != nil {
            sc1.PmsTsi_01_0 = fmt.Sprintf("%d", msg.GetPmsTsi_01_0())
        }
        if msg.PmsTsi_02_5 != nil {
            sc1.PmsTsi_02_5 = fmt.Sprintf("%d", msg.GetPmsTsi_02_5())
        }
        if msg.PmsTsi_10_0 != nil {
            sc1.PmsTsi_10_0 = fmt.Sprintf("%d", msg.GetPmsTsi_10_0())
        }

        if msg.PmsStd_01_0 != nil {
            sc1.PmsStd_01_0 = fmt.Sprintf("%d", msg.GetPmsStd_01_0())
        }
        if msg.PmsStd_02_5 != nil {
            sc1.PmsStd_02_5 = fmt.Sprintf("%d", msg.GetPmsStd_02_5())
        }
        if msg.PmsStd_10_0 != nil {
            sc1.PmsStd_10_0 = fmt.Sprintf("%d", msg.GetPmsStd_10_0())
        }

        if msg.PmsCount_00_3 != nil {
            sc1.PmsCount_00_3 = fmt.Sprintf("%d", msg.GetPmsCount_00_3())
        }
        if msg.PmsCount_00_5 != nil {
            sc1.PmsCount_00_5 = fmt.Sprintf("%d", msg.GetPmsCount_00_5())
        }
        if msg.PmsCount_01_0 != nil {
            sc1.PmsCount_01_0 = fmt.Sprintf("%d", msg.GetPmsCount_01_0())
        }
        if msg.PmsCount_02_5 != nil {
            sc1.PmsCount_02_5 = fmt.Sprintf("%d", msg.GetPmsCount_02_5())
        }
        if msg.PmsCount_05_0 != nil {
            sc1.PmsCount_05_0 = fmt.Sprintf("%d", msg.GetPmsCount_05_0())
        }
        if msg.PmsCount_10_0 != nil {
            sc1.PmsCount_10_0 = fmt.Sprintf("%d", msg.GetPmsCount_10_0())
        }
    }
    go uploadToSafecast(sc1)

    // Due to Safecast API design limitations, upload the metadata as
    // discrete web uploads.  Once this API limitation is removed,
    // this code should be deleted.
    if !deviceIsSuppressingMetadata {
        if msg.BatteryVoltage != nil {
            sc2 := sc
            sc2.Unit = "bat_voltage"
            sc2.Value = sc1.BatVoltage
            go uploadToSafecast(sc2)
        }
        if msg.BatterySOC != nil {
            sc2 := sc
            sc2.Unit = "bat_soc"
            sc2.Value = sc1.BatSOC
            go uploadToSafecast(sc2)
        }
        if msg.EnvTemperature != nil {
            sc2 := sc
            sc2.Unit = "env_temp"
            sc2.Value = sc1.EnvTemp
            go uploadToSafecast(sc2)
        }
        if msg.EnvHumidity != nil {
            sc2 := sc
            sc2.Unit = "env_humid"
            sc2.Value = sc1.EnvHumid
            go uploadToSafecast(sc2)
        }
        if theSNR != 0.0 {
            sc2 := sc
            sc2.Unit = "wireless_snr"
            sc2.Value = sc1.WirelessSNR
            go uploadToSafecast(sc2)
        }
        if msg.PmsTsi_01_0 != nil {
			sc2 := sc
			sc2.Unit = "pms_tsi_01_0"
	        sc2.Value = sc1.PmsTsi_01_0
			go uploadToSafecast(sc2)
        }
        if msg.PmsTsi_02_5 != nil {
			sc2 := sc
			sc2.Unit = "pms_tsi_02_5"
	        sc2.Value = sc1.PmsTsi_02_5
			go uploadToSafecast(sc2)
        }
        if msg.PmsTsi_10_0 != nil {
			sc2 := sc
			sc2.Unit = "pms_tsi_10_0"
	        sc2.Value = sc1.PmsTsi_10_0
			go uploadToSafecast(sc2)
        }
        if msg.PmsStd_01_0 != nil {
			sc2 := sc
			sc2.Unit = "pms_std_01_0"
	        sc2.Value = sc1.PmsStd_01_0
			go uploadToSafecast(sc2)
        }
        if msg.PmsStd_02_5 != nil {
			sc2 := sc
			sc2.Unit = "pms_std_02_5"
	        sc2.Value = sc1.PmsStd_02_5
			go uploadToSafecast(sc2)
        }
        if msg.PmsStd_10_0 != nil {
			sc2 := sc
			sc2.Unit = "pms_std_10_0"
	        sc2.Value = sc1.PmsStd_10_0
			go uploadToSafecast(sc2)
        }
        if msg.PmsCount_00_3 != nil {
			sc2 := sc
			sc2.Unit = "pms_count_00_3"
	        sc2.Value = sc1.PmsCount_00_3
			go uploadToSafecast(sc2)
        }
        if msg.PmsCount_00_5 != nil {
			sc2 := sc
			sc2.Unit = "pms_count_00_5"
	        sc2.Value = sc1.PmsCount_00_5
			go uploadToSafecast(sc2)
        }
        if msg.PmsCount_01_0 != nil {
			sc2 := sc
			sc2.Unit = "pms_count_01_0"
	        sc2.Value = sc1.PmsCount_01_0
			go uploadToSafecast(sc2)
        }
        if msg.PmsCount_02_5 != nil {
			sc2 := sc
			sc2.Unit = "pms_count_02_5"
	        sc2.Value = sc1.PmsCount_02_5
			go uploadToSafecast(sc2)
        }
        if msg.PmsCount_05_0 != nil {
			sc2 := sc
			sc2.Unit = "pms_count_05_0"
	        sc2.Value = sc1.PmsCount_05_0
			go uploadToSafecast(sc2)
        }
        if msg.PmsCount_10_0 != nil {
			sc2 := sc
			sc2.Unit = "pms_count_10_0"
	        sc2.Value = sc1.PmsCount_10_0
			go uploadToSafecast(sc2)
        }

    }
}

// Begin transaction and return the transaction ID
func beginTransaction(url string) int {
    httpTransactionsInProgress += 1
    httpTransactions += 1
    transaction := httpTransactions % httpTransactionsRecorded
    httpTransactionTimes[transaction] = time.Now()
    fmt.Printf("%s *** [%d] *** About to upload to Safecast\n", time.Now().Format(logDateFormat), transaction)
    return transaction
}

// End transaction and issue warnings
func endTransaction(transaction int, errstr string) {
    httpTransactionsInProgress -= 1
    duration := int(time.Now().Sub(httpTransactionTimes[transaction]) / time.Second)
    httpTransactionDurations[transaction] = duration

    if errstr != "" {
        fmt.Printf("%s *** [%d] *** After %d seconds, ERROR uploading to Safecast %s\n\n", time.Now().Format(logDateFormat), transaction, duration, errstr)
    } else {
        if (duration < 5) {
            fmt.Printf("%s *** [%d] *** Completed successfully.\n", time.Now().Format(logDateFormat), transaction);
        } else {
            fmt.Printf("%s *** [%d] *** After %d seconds, completed successfully.\n", time.Now().Format(logDateFormat), transaction, duration);
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

    transaction := beginTransaction(SafecastUploadURL)

    scJSON, _ := json.Marshal(sc)
    fmt.Printf("%s\n", scJSON)

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
                sendToSafecastOps(fmt.Sprintf("** Warning**  Device %d hasn't been seen for %d minutes",
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

        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements|%010d>", s, id, id)

        if (sortedDevices[i].dualSeen) {
            s = fmt.Sprintf("%s <http://dev.safecast.org/en-US/devices/%d/measurements|%010d>", s,
                sortedDevices[i].originalDeviceNo, sortedDevices[i].originalDeviceNo)
        }

        s = fmt.Sprintf("%s (", s)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=bat_voltage|V>", s, id)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=bat_soc|%%>", s, id)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=env_temp|T>", s, id)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=env_humid|H>", s, id)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=wireless_snr|S>", s, id)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=message|M>", s, id)
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
