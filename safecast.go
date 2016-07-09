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

// Describes every device that has sent us a message
type seenDevice struct {
    originalDeviceNo   uint32
    normalizedDeviceNo uint32
    dualSeen           bool
    seen               time.Time
    notifiedAsUnseen   bool
    minutesAgo         int64
}
var seenDevices []seenDevice

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
    ipInfo string,
    defaultTime string,
    defaultSNR float32,
    defaultLat float32, defaultLon float32, defaultAlt int32) {
    var theSNR float32

    // Process IPINFO data
    var info IPInfoData
    if ipInfo != "" {
        err := json.Unmarshal([]byte(ipInfo), &info)
        if err != nil {
            ipInfo = ""
        }
    }
    if ipInfo != "" {
        fmt.Printf("Safecast message from %s/%s/%s:\n%s\n", info.City, info.Region, info.Country, msg)
    } else {
        fmt.Printf("Safecast message:\n%s\n", msg)
    }

    // Log it
    if msg.DeviceIDNumber != nil {
        trackDevice(msg.GetDeviceIDNumber())
    } else if msg.DeviceIDString != nil {
        i64, err := strconv.ParseInt(msg.GetDeviceIDString(), 10, 64)
        if err == nil {
            trackDevice(uint32(i64))
        }
    }

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
        }
    }
    if msg.Longitude != nil {
        sc.Longitude = fmt.Sprintf("%f", msg.GetLongitude())
    } else {
        if defaultLon != 0.0 {
            sc.Longitude = fmt.Sprintf("%f", defaultLon)
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
    }
    uploadToSafecast(&sc1)

    // Due to Safecast API design limitations, upload the metadata as
    // discrete web uploads.  Once this API limitation is removed,
    // this code should be deleted.
    if !deviceIsSuppressingMetadata {
        if msg.BatteryVoltage != nil {
            sc2 := sc
            sc2.Unit = "bat_voltage"
            sc2.Value = sc1.BatVoltage
            uploadToSafecast(&sc2)
        }
        if msg.BatterySOC != nil {
            sc3 := sc
            sc3.Unit = "bat_soc"
            sc3.Value = sc1.BatSOC
            uploadToSafecast(&sc3)
        }
        if msg.EnvTemperature != nil {
            sc4 := sc
            sc4.Unit = "env_temp"
            sc4.Value = sc1.EnvTemp
            uploadToSafecast(&sc4)
        }
        if msg.EnvHumidity != nil {
            sc5 := sc
            sc5.Unit = "env_humid"
            sc5.Value = sc1.EnvHumid
            uploadToSafecast(&sc5)
        }
        if theSNR != 0.0 {
            sc6 := sc
            sc6.Unit = "wireless_snr"
            sc6.Value = sc1.WirelessSNR
            uploadToSafecast(&sc6)
        }

    }
}

// Upload a Safecast data structure to the Safecast service
func uploadToSafecast(sc *SafecastData) {

    scJSON, _ := json.Marshal(sc)
    fmt.Printf("About to upload to %s:\n%s\n", SafecastUploadURL, scJSON)
    req, err := http.NewRequest("POST", fmt.Sprintf(SafecastUploadURL, SafecastAppKey), bytes.NewBuffer(scJSON))
    req.Header.Set("User-Agent", "TTSERVE")
    req.Header.Set("Content-Type", "application/json")
    httpclient := &http.Client{}

    resp, err := httpclient.Do(req)
    if err != nil {
        fmt.Printf("*** Error uploading %s to Safecast %s\n\n", sc.Unit, err)
    } else {
        resp.Body.Close()
    }

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
                sendToSafecastOps(fmt.Sprintf("** Warning**  Device %d hasn't been seen for %d minutes!",
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