// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Device monitoring
package main

import (
	"fmt"
	"io/ioutil"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Describes every device that has sent us a message
type seenDevice struct {
	deviceUID        string
	deviceID         uint32
	label            string
	seen             time.Time
	everRecentlySeen bool
	notifiedAsUnseen bool
	minutesAgo       int64
}

var seenDevices []seenDevice

// Class used to sort seen devices
type byDeviceKey []seenDevice

func (a byDeviceKey) Len() int      { return len(a) }
func (a byDeviceKey) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byDeviceKey) Less(i, j int) bool {
	// Primary:
	// By capture time, most recent last (so that the most recent is nearest your attention, at the bottom in Slack)
	if a[i].seen.Before(a[j].seen) {
		return true
	} else if a[i].seen.After(a[j].seen) {
		return false
	}
	// Secondary
	// In an attempt to keep things reasonably deterministic, use deviceUID
	if a[i].deviceUID < a[j].deviceUID {
		return true
	} else if a[i].deviceUID > a[j].deviceUID {
		return false
	}
	return false
}

// Keep track of all devices that have logged data via ttserve
func trackDevice(DeviceUID string, DeviceID uint32, whenSeen time.Time) {
	var dev seenDevice
	dev.deviceUID = DeviceUID
	dev.deviceID = DeviceID

	// Attempt to update the existing entry if we can find it
	found := false
	for i := 0; i < len(seenDevices); i++ {
		if dev.deviceUID == seenDevices[i].deviceUID {
			// Only pay attention to things that have truly recently come or gone
			minutesAgo := int64(time.Now().Sub(whenSeen) / time.Minute)
			if minutesAgo < deviceWarningAfterMinutes(dev.deviceUID) {
				seenDevices[i].everRecentlySeen = true
				// Notify when the device comes back
				if seenDevices[i].notifiedAsUnseen {
					message := AgoMinutes(uint32(time.Now().Sub(seenDevices[i].seen) / time.Minute))
					sendToSafecastOps(fmt.Sprintf("** NOTE ** Device %s has returned after %s", seenDevices[i].deviceUID, message), SlackMsgUnsolicitedOps)
				}
				// Mark as having been seen on the latest date of any file having that time
				seenDevices[i].notifiedAsUnseen = false
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
		dev.minutesAgo = int64(time.Now().Sub(dev.seen) / time.Minute)
		dev.everRecentlySeen = dev.minutesAgo < deviceWarningAfterMinutes(dev.deviceUID)
		dev.notifiedAsUnseen = false
		dev.label = SafecastDeviceUIDType(dev.deviceUID)
		seenDevices = append(seenDevices, dev)
	}

}

// Update the list of seen devices
func trackAllDevices() {

	// Loop over the file system, tracking all devices
	files, err := ioutil.ReadDir(SafecastDirectory() + TTDeviceLogPath)
	if err == nil {

		// Iterate over each of the values
		for _, file := range files {

			if !file.IsDir() {

				// ONLY pay attention to files from the current year/month
				deviceUID := ""
				Str0 := file.Name()
				Str1 := strings.Split(Str0, ".")[0]
				Str2 := strings.Split(Str1, DeviceLogSep())
				if len(Str2) >= 2 {
					Str3 := strings.Split(Str2[0], "-")
					if len(Str3) >= 2 {
						yr, _ := strconv.ParseUint(Str3[0], 10, 32)
						mo, _ := strconv.ParseUint(Str3[1], 10, 32)
						if int(yr) == time.Now().Year() && int(mo) == int(time.Now().Month()) {
							deviceUID = Str2[1]
						}
					}
				}

				// Track the device
				if deviceUID != "" {
					trackDevice(deviceUID, 0, file.ModTime())
				}

			}
		}
	}
}

// Get sheet records for all seen devices
func devicesSeenInfo() (allInfo []sheetInfo) {

	// Update the in-memory list of seen devices
	trackAllDevices()

	// Force a re-read of the sheet, just to ensure that it reflects the lastest changes
	sheetInvalidateCache()

	// Sweep through all devices that we've seen
	for i := 0; i < len(seenDevices); i++ {
		info, _ := sheetDeviceInfo(seenDevices[i].deviceID) // Ignore errors
		info.DeviceURN = seenDevices[i].deviceUID
		info.LastSeen = seenDevices[i].seen.UTC().Format("2006-01-02T15:04:05Z")
		_, _, info.LastSeenLat, info.LastSeenLon, info.LastSeenSummary = GetDeviceStatusSummary(seenDevices[i].deviceUID)
		allInfo = append(allInfo, info)
	}

	return
}

// Update message ages and notify
func sendExpiredSafecastDevicesToSlack() {

	// Update the in-memory list of seen devices
	trackAllDevices()

	// Compute an expiration time

	// Sweep through all devices that we've seen
	for i := 0; i < len(seenDevices); i++ {

		// Update when we've last seen the device
		expiration := time.Now().Add(-(time.Duration(deviceWarningAfterMinutes(seenDevices[i].deviceUID)) * time.Minute))
		seenDevices[i].minutesAgo = int64(time.Now().Sub(seenDevices[i].seen) / time.Minute)

		// Notify Slack once and only once when a device has expired
		if !seenDevices[i].notifiedAsUnseen && seenDevices[i].everRecentlySeen {
			if seenDevices[i].seen.Before(expiration) {
				seenDevices[i].notifiedAsUnseen = true
				sendToSafecastOps(fmt.Sprintf("** Warning **  Device %s hasn't been seen for %s",
					seenDevices[i].deviceUID,
					AgoMinutes(uint32(seenDevices[i].minutesAgo))), SlackMsgUnsolicitedOps)
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
	sort.Sort(byDeviceKey(sortedDevices))

	// Sweep over all these devices in sorted order, refreshing label
	for i := 0; i < len(sortedDevices); i++ {
		sortedDevices[i].label, _, _, _, _ = GetDeviceStatusSummary(sortedDevices[i].deviceUID)
	}

}

// Get the number of minutes after which to expire a device
func deviceWarningAfterMinutes(deviceUID string) int64 {

	// On 2017-08-14 Ray changed to only warn very rarely, because it was getting
	// far, far too noisy in the ops channel with lots of devices.
	return 24 * 60

}

// Get a summary of devices that are older than this many minutes ago
func sendSafecastDeviceSummaryToSlack(user string, header string, fOffline bool) {

	// Update the in-memory list of seen devices
	trackAllDevices()

	// Force a re-read of the sheet, just to ensure that it reflects the lastest changes
	sheetInvalidateCache()

	// First, age out the expired devices and recompute when last seen
	sendExpiredSafecastDevicesToSlack()

	// Next sort the device list
	sortedDevices := seenDevices
	sort.Sort(byDeviceKey(sortedDevices))

	// Finally, sweep over all these devices in sorted order,
	// generating a single large text string to be sent as a Slack message
	s := header
	numAdded := 0
	numPending := 0
	for i := 0; i < len(sortedDevices); i++ {

		// Skip if the online state doesn't match
		isOffline := sortedDevices[i].minutesAgo > (12 * 60)
		if isOffline != fOffline {
			continue
		}

		// Add it to the summary
		if s != "" {
			s += fmt.Sprintf("\n")
		}

		id := sortedDevices[i].deviceUID
		label := sortedDevices[i].label
		gps := ""
		summary := ""
		label, gps, _, _, summary = GetDeviceStatusSummary(id)
		// Refresh cached label
		sortedDevices[i].label = label

		s += fmt.Sprintf("<http://%s%s%s|%s> ", TTServerHTTPAddress, TTServerTopicDeviceStatus, id, id)
		s += fmt.Sprintf("<http://%s%s%s|chk> ", TTServerHTTPAddress, TTServerTopicDeviceCheck, id)
		s += fmt.Sprintf("<http://%s%s%s%s.json|log> ", TTServerHTTPAddress, TTServerTopicDeviceLog, time.Now().UTC().Format("2006-01"+DeviceLogSep()), DeviceUIDFilename(id))
		if gps != "" {
			s += gps + " "
		} else {
			s += "gps "
		}

		if sortedDevices[i].minutesAgo != 0 {
			s += fmt.Sprintf("%s ago", AgoMinutes(uint32(sortedDevices[i].minutesAgo)))
		}

		if label != "" {
			s += fmt.Sprintf(" %s", label)
		}

		if summary != "" {
			s += " " + summary
		}

		// Display
		numAdded++
		numPending++
		if numPending > 9 {
			sendToSafecastOps(s, SlackMsgReply)
			s = ""
			numPending = 0
		}

	}

	// None
	if numAdded == 0 {
		if fOffline {
			s = "All devices are currently online."
		} else {
			s = "All devices are currently offline."
		}
		numPending++
	}

	// Send it to Slack
	if numPending > 0 {
		sendToSafecastOps(s, SlackMsgReply)
	}

}
