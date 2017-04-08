// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Gateway monitoring
package main

import (
    "sort"
    "time"
    "fmt"
    "strings"
    "io/ioutil"
)

// Warning behavior
const gatewayWarningAfterMinutes = 90

// Describes every device that has sent us a message
type seenGateway struct {
    gatewayid           string
    seen                time.Time
    everRecentlySeen    bool
    notifiedAsUnseen    bool
    minutesAgo          int64
}
var seenGateways []seenGateway

// Class used to sort seen devices
type ByGatewayKey []seenGateway
func (a ByGatewayKey) Len() int      { return len(a) }
func (a ByGatewayKey) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByGatewayKey) Less(i, j int) bool {
    // Primary:
    // By capture time, most recent last (so that the most recent is nearest your attention, at the bottom in Slack)
    if a[i].seen.Before(a[j].seen) {
        return true
    } else if a[i].seen.After(a[j].seen) {
        return false
    }
    // Secondary
    // In an attempt to keep things reasonably deterministic, compare strings
    if strings.Compare(a[i].gatewayid, a[j].gatewayid) < 0 {
        return true
    } else if strings.Compare(a[i].gatewayid, a[j].gatewayid) > 0 {
        return false
    }
    return false
}

// Keep track of all devices that have logged data via ttserve
func trackGateway(GatewayId string, whenSeen time.Time) {
    var dev seenGateway
    dev.gatewayid = GatewayId

    // Attempt to update the existing entry if we can find it
    found := false
    for i := 0; i < len(seenGateways); i++ {
        if dev.gatewayid == seenGateways[i].gatewayid {
            // Only pay attention to things that have truly recently come or gone
            minutesAgo := int64(time.Now().Sub(whenSeen) / time.Minute)
            if minutesAgo < deviceWarningAfterMinutes {
                seenGateways[i].everRecentlySeen = true
                // Notify when the device comes back
                if seenGateways[i].notifiedAsUnseen {
                    minutesAgo := int64(time.Now().Sub(seenGateways[i].seen) / time.Minute)
                    hoursAgo := minutesAgo / 60
                    daysAgo := hoursAgo / 24
                    message := fmt.Sprintf("%d minutes", minutesAgo)
                    switch {
                    case daysAgo >= 2:
                        message = fmt.Sprintf("~%d days", daysAgo)
                    case minutesAgo >= 120:
                        message = fmt.Sprintf("~%d hours", hoursAgo)
                    }
                    sendToSafecastOps(fmt.Sprintf("** NOTE ** Gateway %s has returned after %s away", seenGateways[i].gatewayid, message), SLACK_MSG_UNSOLICITED_OPS)
                }
                // Mark as having been seen on the latest date of any file having that time
                seenGateways[i].notifiedAsUnseen = false
            }
            // Always track the most recent seen date
            if seenGateways[i].seen.Before(whenSeen) {
                seenGateways[i].seen = whenSeen
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
        seenGateways = append(seenGateways, dev)
    }

}

// Update the list of seen devices
func trackAllGateways() {

    // Loop over the file system, tracking all devices
    files, err := ioutil.ReadDir(SafecastDirectory() + TTGatewayStatusPath)
    if err == nil {

        // Iterate over each of the values
        for _, file := range files {

            if !file.IsDir() {

                // Extract device ID from filename
                Str0 := file.Name()
                gatewayID := strings.Split(Str0, ".")[0]

                // Track the device
                if gatewayID != "" {
                    trackGateway(gatewayID, file.ModTime())
                }

            }
        }
    }
}

// Update message ages and notify
func sendExpiredSafecastGatewaysToSlack() {

    // Update the in-memory list of seen devices
    trackAllGateways()

    // Compute an expiration time
    expiration := time.Now().Add(-(time.Duration(deviceWarningAfterMinutes) * time.Minute))

    // Sweep through all gateways that we've seen
    for i := 0; i < len(seenGateways); i++ {

        // Update when we've last seen the device
        seenGateways[i].minutesAgo = int64(time.Now().Sub(seenGateways[i].seen) / time.Minute)

        // Notify Slack once and only once when a device has expired
        if !seenGateways[i].notifiedAsUnseen && seenGateways[i].everRecentlySeen {
            if seenGateways[i].seen.Before(expiration) {
                seenGateways[i].notifiedAsUnseen = true
                sendToSafecastOps(fmt.Sprintf("** Warning **  Gateway %s hasn't been seen for %d minutes",
                    seenGateways[i].gatewayid,
                    seenGateways[i].minutesAgo), SLACK_MSG_UNSOLICITED_OPS)
            }
        }
    }
}

// Get a summary of devices that are older than this many minutes ago
func sendSafecastGatewaySummaryToSlack(header string, fDetails bool) {

    // First, age out the expired devices and recompute when last seen
    sendExpiredSafecastGatewaysToSlack()

    // Next sort the device list
    sortedGateways := seenGateways
    sort.Sort(ByGatewayKey(sortedGateways))

    // Build the summary string
    s := header

    // Finally, sweep over all these devices in sorted order,
    // generating a single large text string to be sent as a Slack message
    for i := 0; i < len(sortedGateways); i++ {
        gatewayID := sortedGateways[i].gatewayid

        // Emit info about the device
        summary := SafecastGetGatewaySummary(gatewayID, "    ", fDetails)
        if summary != "" {
            if s != "" {
                s += fmt.Sprintf("\n")
            }
            s += fmt.Sprintf("<http://%s%s%s|%s>", TTServerHTTPAddress, TTServerTopicGatewayStatus, gatewayID, gatewayID)
            if summary != "" {
                s += fmt.Sprintf(" %s", summary)
            }
        }
    }

    // Send it to Slack
    if s == "" {
        s = "No gateways have recently reported"
    }
    sendToSafecastOps(s, SLACK_MSG_REPLY)

}
