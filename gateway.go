// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Gateway monitoring
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// Warning behavior
const gatewayWarningAfterMinutes = 90

// Describes every device that has sent us a message
type seenGateway struct {
	label            string
	gatewayid        string
	seen             time.Time
	everRecentlySeen bool
	notifiedAsUnseen bool
	minutesAgo       int64
}

var seenGateways []seenGateway

// Class used to sort seen devices
type byGatewayKey []seenGateway

func (a byGatewayKey) Len() int      { return len(a) }
func (a byGatewayKey) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byGatewayKey) Less(i, j int) bool {
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
func trackGateway(GatewayID string, whenSeen time.Time) {
	var dev seenGateway
	dev.gatewayid = GatewayID

	// Attempt to update the existing entry if we can find it
	found := false
	for i := 0; i < len(seenGateways); i++ {
		if dev.gatewayid == seenGateways[i].gatewayid {
			// Only pay attention to things that have truly recently come or gone
			minutesAgo := int64(time.Since(whenSeen) / time.Minute)
			if minutesAgo < gatewayWarningAfterMinutes {
				seenGateways[i].everRecentlySeen = true
				// Notify when the device comes back
				if seenGateways[i].notifiedAsUnseen {
					message := AgoMinutes(uint32(time.Since(seenGateways[i].seen) / time.Minute))
					if seenGateways[i].label != "" {
						sendToSafecastOps(fmt.Sprintf("** NOTE ** Gateway %s \"%s\" has returned after %s", seenGateways[i].gatewayid, seenGateways[i].label, message), SlackMsgUnsolicitedOps)
					} else {
						sendToSafecastOps(fmt.Sprintf("** NOTE ** Gateway %s has returned after %s", seenGateways[i].gatewayid, message), SlackMsgUnsolicitedOps)
					}
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
		_, dev.label = GetGatewaySummary(dev.gatewayid, "")
		dev.seen = whenSeen
		minutesAgo := int64(time.Since(dev.seen) / time.Minute)
		dev.everRecentlySeen = minutesAgo < gatewayWarningAfterMinutes
		dev.notifiedAsUnseen = false
		seenGateways = append(seenGateways, dev)
	}

}

// Update the list of seen devices
func trackAllGateways() {

	// Loop over the file system, tracking all devices
	files, err := os.ReadDir(SafecastDirectory() + TTGatewayStatusPath)
	if err == nil {

		// Iterate over each of the values
		for _, file := range files {

			if !file.IsDir() {

				// Extract device ID from filename
				Str0 := file.Name()
				gatewayID := strings.Split(Str0, ".")[0]

				// Track the device
				if gatewayID != "" {
					info, err := file.Info()
					if err == nil {
						trackGateway(gatewayID, info.ModTime())
					}
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
	expiration := time.Now().Add(-(time.Duration(gatewayWarningAfterMinutes) * time.Minute))

	// Sweep through all gateways that we've seen
	for i := 0; i < len(seenGateways); i++ {

		// Update when we've last seen the device
		seenGateways[i].minutesAgo = int64(time.Since(seenGateways[i].seen) / time.Minute)

		// Notify Slack once and only once when a device has expired
		if !seenGateways[i].notifiedAsUnseen && seenGateways[i].everRecentlySeen {
			if seenGateways[i].seen.Before(expiration) {
				seenGateways[i].notifiedAsUnseen = true
				if seenGateways[i].label != "" {
					sendToSafecastOps(fmt.Sprintf("** Warning **  Gateway %s \"%s\" hasn't been seen for %s", seenGateways[i].gatewayid, seenGateways[i].label, AgoMinutes(uint32(seenGateways[i].minutesAgo))), SlackMsgUnsolicitedOps)
				} else {
					sendToSafecastOps(fmt.Sprintf("** Warning **  Gateway %s hasn't been seen for %s", seenGateways[i].gatewayid, AgoMinutes(uint32(seenGateways[i].minutesAgo))), SlackMsgUnsolicitedOps)
				}
			}
		}
	}
}

// Get a summary of devices that are older than this many minutes ago
func sendSafecastGatewaySummaryToSlack(header string) {

	// First, age out the expired devices and recompute when last seen
	sendExpiredSafecastGatewaysToSlack()

	// Next sort the device list
	sortedGateways := seenGateways
	sort.Sort(byGatewayKey(sortedGateways))

	// Build the summary string
	s := header

	// Finally, sweep over all these devices in sorted order,
	// generating a single large text string to be sent as a Slack message
	for i := 0; i < len(sortedGateways); i++ {
		gatewayID := sortedGateways[i].gatewayid

		// Emit info about the device
		summary, _ := GetGatewaySummary(gatewayID, "    ")
		if summary != "" {
			if s != "" {
				s += "\n"
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
	sendToSafecastOps(s, SlackMsgReply)

}
