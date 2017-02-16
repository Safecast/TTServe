// Safecast inbound message handling and publishing
package main

import (
    "fmt"
    "sort"
    "time"
    "io/ioutil"
    "strings"
    "strconv"
)

// Warning behavior
const deviceWarningAfterMinutes = 90

// Describes every device that has sent us a message
type seenDevice struct {
    deviceid            uint32
    seen                time.Time
    everRecentlySeen    bool
    notifiedAsUnseen    bool
    minutesAgo          int64
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
    if a[i].deviceid < a[j].deviceid {
        return true
    } else if a[i].deviceid > a[j].deviceid {
        return false
    }
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