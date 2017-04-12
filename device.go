// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Device monitoring
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
    label               string
    seen                time.Time
    everRecentlySeen    bool
    notifiedAsUnseen    bool
    minutesAgo          int64
}
var seenDevices []seenDevice

// Class used to sort seen devices
type ByDeviceKey []seenDevice
func (a ByDeviceKey) Len() int      { return len(a) }
func (a ByDeviceKey) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByDeviceKey) Less(i, j int) bool {
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
func trackDevice(DeviceId uint32, whenSeen time.Time) {
    var dev seenDevice
    dev.deviceid = DeviceId

    // Attempt to update the existing entry if we can find it
    found := false
    for i := 0; i < len(seenDevices); i++ {
        if dev.deviceid == seenDevices[i].deviceid {
            // Only pay attention to things that have truly recently come or gone
            minutesAgo := int64(time.Now().Sub(whenSeen) / time.Minute)
            if minutesAgo < deviceWarningAfterMinutes {
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
                    sendToSafecastOps(fmt.Sprintf("** NOTE ** Device %d has returned after %s away", seenDevices[i].deviceid, message), SLACK_MSG_UNSOLICITED_OPS)
                }
                // Mark as having been seen on the latest date of any file having that time
                seenDevices[i].notifiedAsUnseen = false;
            }
            // Always track the most recent seen date
            if seenDevices[i].seen.Before(whenSeen) {
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
        dev.label = SafecastV1DeviceType(dev.deviceid)
        seenDevices = append(seenDevices, dev)
    }

}

// Update the list of seen devices
func trackAllDevices() {

    // Loop over the file system, tracking all devices
    files, err := ioutil.ReadDir(SafecastDirectory() + TTDeviceStatusPath)
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
                if deviceID != 0 {
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
            _, _, value := SafecastReadDeviceStatus(deviceID)

            // Check to see if it has a required stats field
            if value.Dev == nil || value.Dev.AppVersion == nil {

                // See if there's a pending command already waiting for this device
                isValid, _ := getCommand(deviceID)
                if !isValid {

                    sendToSafecastOps(fmt.Sprintf("** NOTE ** Sending hello to newly-detected device %d", deviceID), SLACK_MSG_UNSOLICITED_OPS)
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
                    seenDevices[i].minutesAgo), SLACK_MSG_UNSOLICITED_OPS)
            }
        }
    }
}

// Refresh the labels on cached devices
func refreshDeviceSummaryLabels() {

    // First, age out the expired devices and recompute when last seen
    sendExpiredSafecastDevicesToSlack()

    // Next sort the device list
    sortedDevices := seenDevices
    sort.Sort(ByDeviceKey(sortedDevices))

    // Sweep over all these devices in sorted order, refreshing label
    for i := 0; i < len(sortedDevices); i++ {
        _, sortedDevices[i].label, _, _ = SafecastGetDeviceStatusSummary(sortedDevices[i].deviceid)
    }

}

// Get a summary of devices that are older than this many minutes ago
func sendSafecastDeviceSummaryToSlack(user string, header string, devicelist string, fOffline bool, fDetails bool) {

    // Get the device list if one was specified
    valid, _, devices, ranges, _ := DeviceList(user, devicelist)
    if !valid {
        devices = nil
    }

    // First, age out the expired devices and recompute when last seen
    sendExpiredSafecastDevicesToSlack()

    // Next sort the device list
    sortedDevices := seenDevices
    sort.Sort(ByDeviceKey(sortedDevices))

    // Finally, sweep over all these devices in sorted order,
    // generating a single large text string to be sent as a Slack message
    s := header
    for i := 0; i < len(sortedDevices); i++ {

		// Skip if the online state doesn't match
        isOffline := sortedDevices[i].minutesAgo > (2 * 60)
        if isOffline != fOffline {
            continue
        }

		// Skip if this device isn't within a supplied list or range
        id := sortedDevices[i].deviceid

		found := true
		if devices != nil || ranges != nil {
			found = false
		}
		
        if !found && devices != nil {
            for _, did := range devices {
                if did == id {
                    found = true
                    break
                }
            }
        }

        if !found && ranges != nil {
            for _, r := range ranges {
                if id >= r.Low && id <= r.High {
                    found = true
                    break
                }
            }
        }

        if !found {
            continue
        }

		// Add it to the summary
        if s != "" {
            s += fmt.Sprintf("\n")
        }

        label := sortedDevices[i].label
        gps := ""
        summary := ""
        if fDetails {
            _, label, gps, summary = SafecastGetDeviceStatusSummary(id)
            // Refresh cached label
            sortedDevices[i].label = label
        }

        words := WordsFromNumber(id)

        s += fmt.Sprintf("<http://%s%s%d|%010d> ", TTServerHTTPAddress, TTServerTopicDeviceStatus, id, id)

        s += fmt.Sprintf("<http://%s%s%d|chk> ", TTServerHTTPAddress, TTServerTopicDeviceCheck, id)
        s += fmt.Sprintf("<http://%s%s%s%d.json|log> ", TTServerHTTPAddress, TTServerTopicDeviceLog, time.Now().UTC().Format("2006-01-"), id)
        if (false) {    // Removed 2017-03-19 to discourage people from using csv, which does not have full data
            s += fmt.Sprintf("<http://%s%s%s%d.csv|csv>", TTServerHTTPAddress, TTServerTopicDeviceLog, time.Now().UTC().Format("2006-01-"), id)
        }
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
            s = fmt.Sprintf("%s %s ago", s, AgoMinutes(uint32(sortedDevices[i].minutesAgo)))
        }

        sn, _ := SafecastDeviceIDToSN(id)
        if sn != 0 {
            s += fmt.Sprintf(" #%d", sn)
        }
        if label != "" {
            s += fmt.Sprintf(" \"%s\"", label)
        }
        if words != "" {
            s += fmt.Sprintf(" %s", words)
        }

        if summary != "" {
            s += " ( "
            if summary != "" {
                s += summary
            }
            s += ")"
        }

    }

    // Send it to Slack
    sendToSafecastOps(s, SLACK_MSG_REPLY)

}

// Get a summary of devices that are older than this many minutes ago
func generateTTNCTLDeviceRegistrationScript() {

    // First, age out the expired devices and recompute when last seen
    sendExpiredSafecastDevicesToSlack()

    // Next sort the device list
    sortedDevices := seenDevices
    sort.Sort(ByDeviceKey(sortedDevices))

    // Sweep over devices and generate the TTNCTL commands, newest first
    s := ""
    for i := 0; i < len(sortedDevices); i++ {
        id := sortedDevices[i].deviceid
        deveui, _, _, _ := SafecastGetDeviceStatusSummary(id)
        if deveui != "" {
            s += fmt.Sprintf("ttnctl devices register %s\n", strings.ToLower(deveui))
        }
    }

    // Send it to Slack
    if s != "" {
        sendToSafecastOps(s, SLACK_MSG_REPLY)
    } else {
        sendToSafecastOps("None found.", SLACK_MSG_REPLY)
    }

}
