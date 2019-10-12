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

// Describes every device that has sent us a message
type seenDevice struct {
	deviceid			uint32
	normalizedSN		string
	label				string
	seen				time.Time
	everRecentlySeen	bool
	notifiedAsUnseen	bool
	minutesAgo			int64
}
var seenDevices []seenDevice

// Class used to sort seen devices
type byDeviceKey []seenDevice
func (a byDeviceKey) Len() int		{ return len(a) }
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
	// In an attempt to keep things reasonably deterministic, use device number
	if a[i].deviceid < a[j].deviceid {
		return true
	} else if a[i].deviceid > a[j].deviceid {
		return false
	}
	return false
}

// Keep track of all devices that have logged data via ttserve
func trackDevice(DeviceID uint32, whenSeen time.Time, normalizedSN string) {
	var dev seenDevice
	dev.deviceid = DeviceID
	dev.normalizedSN = normalizedSN

	fmt.Printf("OZZIE trackDevice %d %s %s\n", DeviceID, normalizedSN, whenSeen.Format("2006-01-02-15-04-05"))

	// Attempt to update the existing entry if we can find it
	found := false
	for i := 0; i < len(seenDevices); i++ {
		if dev.deviceid == seenDevices[i].deviceid || dev.normalizedSN == seenDevices[i].normalizedSN {
			// Only pay attention to things that have truly recently come or gone
			minutesAgo := int64(time.Now().Sub(whenSeen) / time.Minute)
			if minutesAgo < deviceWarningAfterMinutes(dev.deviceid) {
				seenDevices[i].everRecentlySeen = true
				// Notify when the device comes back
				if seenDevices[i].notifiedAsUnseen {
					message := AgoMinutes(uint32(time.Now().Sub(seenDevices[i].seen) / time.Minute))
					sendToSafecastOps(fmt.Sprintf("** NOTE ** Device %d has returned after %s", seenDevices[i].deviceid, message), SlackMsgUnsolicitedOps)
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
		dev.everRecentlySeen = minutesAgo < deviceWarningAfterMinutes(dev.deviceid)
		dev.notifiedAsUnseen = false
		dev.label, _ = SafecastDeviceType(dev.deviceid)
		seenDevices = append(seenDevices, dev)
		fmt.Printf("OZZIE added to list (len=%d)\n", len(seenDevices))
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
				deviceID := uint32(0)
				Str0 := file.Name()
				Str1 := strings.Split(Str0, ".")[0]
				Str2 := strings.Split(Str1, "-")
				if len(Str2) >= 3 {
					yr, _ := strconv.ParseUint(Str2[0], 10, 32)
					mo, _ := strconv.ParseUint(Str2[1], 10, 32)
					if int(yr) == time.Now().Year() && int(mo) == int(time.Now().Month()) {
						i64, _ := strconv.ParseUint(Str2[2], 10, 32)
						deviceID = uint32(i64)
					}
				}
				normalizedSN := ""
				if len(Str2) >= 4 {
					normalizedSN = Str2[3]
				}
				fmt.Printf("OZZIE %s %s %v\n", normalizedSN, Str0, Str2)

				// Track the device
				if deviceID != 0 || normalizedSN != "" {
					trackDevice(deviceID, file.ModTime(), normalizedSN)
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
			_, _, value := ReadDeviceStatus(deviceID)

			// Check to see if it has a required stats field
			if value.Dev == nil || value.Dev.AppVersion == nil {

				// See if there's a pending command already waiting for this device
				isValid, _ := getCommand(deviceID)
				if !isValid {

					sendToSafecastOps(fmt.Sprintf("** NOTE ** Sending hello to newly-detected device %d", deviceID), SlackMsgUnsolicitedOps)
					sendCommand("New device detected", deviceID, "hello")

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
		info, err := sheetDeviceInfo(seenDevices[i].deviceid, seenDevices[i].normalizedSN)
		if err == nil {
			info.LastSeen = seenDevices[i].seen.UTC().Format("2006-01-02T15:04:05Z")
			allInfo = append(allInfo, info)
		}
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
		expiration := time.Now().Add(-(time.Duration(deviceWarningAfterMinutes(seenDevices[i].deviceid)) * time.Minute))
		seenDevices[i].minutesAgo = int64(time.Now().Sub(seenDevices[i].seen) / time.Minute)

		// Notify Slack once and only once when a device has expired
		if !seenDevices[i].notifiedAsUnseen && seenDevices[i].everRecentlySeen {
			if seenDevices[i].seen.Before(expiration) {
				seenDevices[i].notifiedAsUnseen = true
				sendToSafecastOps(fmt.Sprintf("** Warning **  Device %d hasn't been seen for %s",
					seenDevices[i].deviceid,
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
		sortedDevices[i].label, _, _ = GetDeviceStatusSummary(sortedDevices[i].deviceid, sortedDevices[i].normalizedSN)
	}

}

// Send a command to a list of known devices
func sendSafecastDeviceCommand(user string, devicelist string, command string) {

	// Get the device list if one was specified
	valid, _, devices, ranges, _ := DeviceList(user, devicelist)
	if !valid {
		devices = nil
	}

	// First, age out the expired devices and recompute when last seen
	sendExpiredSafecastDevicesToSlack()

	// Next sort the device list
	sortedDevices := seenDevices
	sort.Sort(byDeviceKey(sortedDevices))

	// Finally, sweep over all these devices in sorted order
	s := ""
	for i := 0; i < len(sortedDevices); i++ {

		// Skip if this device isn't within a supplied list or range
		id := sortedDevices[i].deviceid

		found := false

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

		if s != "" {
			s += "\n"
		}

		// The null command means cancel
		if command == "" {

			// Cancel the command
			isValid, cmd := getCommand(id)
			if isValid {
				s += fmt.Sprintf("'%s' will not be sent to %d %s", cmd.Command, id, WordsFromNumber(id))
				cancelCommand(id)
			} else {
				s += fmt.Sprintf("Nothing pending for %d %s", id, WordsFromNumber(id))
			}

		} else {

			devicetype, _ := SafecastDeviceType(id)
			switch devicetype {

				// Don't send to devices that we cannot
			case "pointcast":
				fallthrough
			case "safecast-air":
				fallthrough
			case "ngeigie":
				s += fmt.Sprintf("Cannot send to %d which is a %s device", id, devicetype)

			default:
				s += fmt.Sprintf("Sending '%s' to %d %s", command, id, WordsFromNumber(id))
				sendCommand(user, id, command)

			}
		}

	}

	// Done
	if s == "" {

		// If device was not found and we're trying to cancel, do it anyway because
		// the device may have gone away by now and isn't in the sorted list.
		if command == "" {
			if devices != nil {
				for _, did := range devices {
					isValid, cmd := getCommand(did)
					cancelCommand(did)
					if isValid {
						s += fmt.Sprintf("'%s' will not be sent to %d %s", cmd.Command, did, WordsFromNumber(did))
					} else {
						s += fmt.Sprintf("Nothing pending for %d %s", did, WordsFromNumber(did))
					}
				}
			}
		}
	}

	if s == "" {
		s = "Device(s) not found"
	}
	sendToSafecastOps(s, SlackMsgReply)

}

// Get the number of minutes after which to expire a device
func deviceWarningAfterMinutes(deviceID uint32) int64 {

	// On 2017-08-14 Ray changed to only warn very rarely, because it was getting
	// far, far too noisy in the ops channel with lots of devices.
	return 24*60

	// This is what the behavior was for months while we were debugging
	deviceType, _ := SafecastDeviceType(deviceID)
	switch deviceType {
	case "pointcast":
		fallthrough
	case "safecast-air":
		return 20
	}

	return 90

}

// Get a summary of devices that are older than this many minutes ago
func sendSafecastDeviceSummaryToSlack(user string, header string, devicelist string, fOffline bool) {

	// Update the in-memory list of seen devices
	trackAllDevices()

	// Force a re-read of the sheet, just to ensure that it reflects the lastest changes
	sheetInvalidateCache()

	// Get the device list if one was specified
	valid, _, devices, ranges, _ := DeviceList(user, devicelist)
	if !valid {
		devices = nil
	}

	// First, age out the expired devices and recompute when last seen
	sendExpiredSafecastDevicesToSlack()

	// Next sort the device list
	sortedDevices := seenDevices
	sort.Sort(byDeviceKey(sortedDevices))

	// Finally, sweep over all these devices in sorted order,
	// generating a single large text string to be sent as a Slack message
	s := header
	for i := 0; i < len(sortedDevices); i++ {

		// Skip if the online state doesn't match
		isOffline := sortedDevices[i].minutesAgo > (12 * 60)
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
		label, gps, summary = GetDeviceStatusSummary(id, sortedDevices[i].normalizedSN)
		// Refresh cached label
		sortedDevices[i].label = label

		words := WordsFromNumber(id)

		s += fmt.Sprintf("<http://%s%s%d|%010d> ", TTServerHTTPAddress, TTServerTopicDeviceStatus, id, id)

		s += fmt.Sprintf("<http://%s%s%d|chk> ", TTServerHTTPAddress, TTServerTopicDeviceCheck, id)
		info, _ := sheetDeviceInfo(id, sortedDevices[i].normalizedSN)
		sn := info.NormalizedSN
		if sn != "" {
			sn = "-"+sn
		}
		s += fmt.Sprintf("<http://%s%s%s%d%s.json|log> ", TTServerHTTPAddress, TTServerTopicDeviceLog, time.Now().UTC().Format("2006-01-"), id, sn)
		if gps != "" {
			s += " " + gps
		} else {
			s += " gps"
		}

		if sortedDevices[i].minutesAgo == 0 {
			s = fmt.Sprintf("%s just now", s)
		} else {
			s = fmt.Sprintf("%s %s ago", s, AgoMinutes(uint32(sortedDevices[i].minutesAgo)))
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

	// None
	if s == header {
		if fOffline {
			s = "All devices are currently online."
		} else {
			s = "All devices are currently offline."
		}
	}

	// Send it to Slack
	sendToSafecastOps(s, SlackMsgReply)

}
