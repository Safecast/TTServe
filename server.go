// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Server monitoring
package main

import (
	"sort"
    "time"
    "fmt"
    "strings"
    "io/ioutil"
)

// Warning behavior
const serverWarningAfterMinutes = 10

// Describes every device that has sent us a message
type seenServer struct {
    serverid            string
    seen                time.Time
    everRecentlySeen    bool
    notifiedAsUnseen    bool
    minutesAgo          int64
}
var seenServers []seenServer

// Class used to sort seen devices
type ByServerKey []seenServer
func (a ByServerKey) Len() int      { return len(a) }
func (a ByServerKey) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByServerKey) Less(i, j int) bool {
    // Primary:
    // By capture time, most recent last (so that the most recent is nearest your attention, at the bottom in Slack)
    if a[i].seen.Before(a[j].seen) {
        return true
    } else if a[i].seen.After(a[j].seen) {
        return false
    }
    // Secondary
    // In an attempt to keep things reasonably deterministic, compare strings
    if strings.Compare(a[i].serverid, a[j].serverid) < 0 {
        return true
    } else if strings.Compare(a[i].serverid, a[j].serverid) > 0 {
        return false
    }
    return false
}

// Keep track of all devices that have logged data via ttserve
func trackServer(ServerId string, whenSeen time.Time) {
    var dev seenServer
    dev.serverid = ServerId

    // Attempt to update the existing entry if we can find it
    found := false
    for i := 0; i < len(seenServers); i++ {
        if dev.serverid == seenServers[i].serverid {
            // Only pay attention to things that have truly recently come or gone
            minutesAgo := int64(time.Now().Sub(whenSeen) / time.Minute)
            if minutesAgo < deviceWarningAfterMinutes {
                seenServers[i].everRecentlySeen = true
                // Notify when the device comes back
                if seenServers[i].notifiedAsUnseen {
                    minutesAgo := int64(time.Now().Sub(seenServers[i].seen) / time.Minute)
                    hoursAgo := minutesAgo / 60
                    daysAgo := hoursAgo / 24
                    message := fmt.Sprintf("%d minutes", minutesAgo)
                    switch {
                    case daysAgo >= 2:
                        message = fmt.Sprintf("~%d days", daysAgo)
                    case minutesAgo >= 120:
                        message = fmt.Sprintf("~%d hours", hoursAgo)
                    }
                    sendToSafecastOps(fmt.Sprintf("** NOTE ** Server %s has returned after %s away", seenServers[i].serverid, message), SLACK_MSG_UNSOLICITED_OPS)
                }
                // Mark as having been seen on the latest date of any file having that time
                seenServers[i].notifiedAsUnseen = false
            }
            // Always track the most recent seen date
            if seenServers[i].seen.Before(whenSeen) {
                seenServers[i].seen = whenSeen
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
        seenServers = append(seenServers, dev)
    }

}

// Update the list of seen devices
func trackAllServers() {

    // Loop over the file system, tracking all devices
    files, err := ioutil.ReadDir(SafecastDirectory() + TTServerStatusPath)
    if err == nil {

        // Iterate over each of the values
        for _, file := range files {

            if !file.IsDir() {

                // Extract device ID from filename
                Str0 := file.Name()
                serverID := strings.Split(Str0, ".")[0]

                // Track the device
                if serverID != "" {
                    trackServer(serverID, file.ModTime())
                }

            }
        }
    }
}

// Update message ages and notify
func sendExpiredSafecastServersToSlack() {

    // Update the in-memory list of seen devices
    trackAllServers()

    // Compute an expiration time
    expiration := time.Now().Add(-(time.Duration(deviceWarningAfterMinutes) * time.Minute))

    // Sweep through all servers that we've seen
    for i := 0; i < len(seenServers); i++ {

        // Update when we've last seen the device
        seenServers[i].minutesAgo = int64(time.Now().Sub(seenServers[i].seen) / time.Minute)

        // Notify Slack once and only once when a device has expired
        if !seenServers[i].notifiedAsUnseen && seenServers[i].everRecentlySeen {
            if seenServers[i].seen.Before(expiration) {
                seenServers[i].notifiedAsUnseen = true
                sendToSafecastOps(fmt.Sprintf("** Warning **  Server %s hasn't been seen for %d minutes",
                    seenServers[i].serverid,
                    seenServers[i].minutesAgo), SLACK_MSG_UNSOLICITED_OPS)
            }
        }
    }
}

// Get a summary of devices that are older than this many minutes ago
func sendSafecastServerSummaryToSlack(header string, fWrap bool, fDetails bool) {

    // First, age out the expired devices and recompute when last seen
    sendExpiredSafecastServersToSlack()

    // Next sort the device list
    sortedServers := seenServers
    sort.Sort(ByServerKey(sortedServers))

    // Build the summary string
    s := header

    // Finally, sweep over all these devices in sorted order,
    // generating a single large text string to be sent as a Slack message
    for i := 0; i < len(sortedServers); i++ {
        serverID := sortedServers[i].serverid

        // Emit info about the device
        summary := SafecastGetServerSummary(serverID, "    ")
        if summary != "" {
            if s != "" {
                s += fmt.Sprintf("\n")
            }
            s += fmt.Sprintf("<http://%s%s%s|%s>", TTServerHTTPAddress, TTServerTopicServerStatus, serverID, serverID)
			s += " "
            s += fmt.Sprintf("<http://%s%s%s$%s|log>", TTServerHTTPAddress, TTServerTopicServerLog, ServerLogSecret(), ServerLogFilename(".log"))
            if summary != "" {
                s += fmt.Sprintf(" %s", summary)
            }
        }
    }

    // Send it to Slack
    if s == "" {
        s = "No servers have recently reported"
    }
    sendToSafecastOps(s, SLACK_MSG_REPLY)

}
