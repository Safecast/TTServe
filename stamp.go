// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Support for "stamping" of messages - a method wherein the
// GPS satellite-detected date/time and location are uploaded
// very infrequently and associated with a "stamp ID".  By
// including this stamp on each uploaded message (along with
// an offset), we save significant network bandwidth.
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	ttproto "github.com/Safecast/ttproto/golang"
)

// Debugging
const debugStamp = false

// StampVersion1 is the first stamp version we introduced.  Unlike the client, the support
// for downlevel stamp version must be kept here forever.
const StampVersion1 = 1

// Cache file format
type stampFile struct {
	Version         uint32  `json:"Version,omitempty"`
	Stamp           uint32  `json:"Stamp,omitempty"`
	Latitude        float64 `json:"Latitude,omitempty"`
	Longitude       float64 `json:"Longitude,omitempty"`
	Altitude        int32   `json:"Altitude,omitempty"`
	CapturedAtDate  uint32  `json:"CapturedAtDate,omitempty"`
	CapturedAtTime  uint32  `json:"CapturedAtTime,omitempty"`
	HasTestMode     bool    `json:"HasTestMode,omitempty"`
	HasMotionOffset bool    `json:"HasMotionOffset,omitempty"`
	TestMode        bool    `json:"TestMode,omitempty"`
	MotionOffset    uint32  `json:"MotionOffset,omitempty"`
}

// Describes every device that has sent us a message
type cachedDevice struct {
	deviceid uint32
	valid    bool
	cache    stampFile
}

var cacheLock sync.RWMutex
var cachedDevices []cachedDevice

// Statics
var substituteCapturedAt string

// Construct the path of a command file
func stpFilename(DeviceID uint32) string {
	directory := SafecastDirectory()
	file := directory + TTDeviceStampPath + "/" + fmt.Sprintf("%d", DeviceID) + ".json"
	return file
}

// Set or apply the stamp
func stampSetOrApply(message *ttproto.Telecast) (isValidMessage bool) {
	var CacheEntry int

	// Device ID is required here, but that doesn't mean it's not a valid message
	if message.DeviceId == nil {
		return true
	}
	DeviceID := message.GetDeviceId()

	// Protect the cache data structure
	cacheLock.Lock()

	// Find or create the cache entry for this device
	found := false
	for CacheEntry = 0; CacheEntry < len(cachedDevices); CacheEntry++ {
		if DeviceID == cachedDevices[CacheEntry].deviceid {
			found = true
			break
		}
	}
	if !found {
		var entry cachedDevice
		entry.deviceid = DeviceID
		entry.valid = false
		cachedDevices = append(cachedDevices, entry)
		CacheEntry = len(cachedDevices) - 1
		if debugStamp {
			fmt.Printf("Added new device cache entry for never-before seen %d: %d\n", DeviceID, CacheEntry)
		}
	}

	// If this is a "set stamp" operation, do it
	if message.StampVersion != nil {
		isValidMessage = stpSet(message, DeviceID, CacheEntry)
		cacheLock.Unlock()
		return
	}

	// If this isn't a "stamp this message" operation, exit
	if message.Stamp != nil {
		isValidMessage = stpApply(message, DeviceID, CacheEntry)
		cacheLock.Unlock()
		return
	}

	// Neither a stamper or a stampee
	cacheLock.Unlock()
	return true

}

// Set or apply the stamp
func stpSet(message *ttproto.Telecast, DeviceID uint32, CacheEntry int) (isValidMessage bool) {

	// Regardless of whatever else happens, we need to invalidate the cache
	cachedDevices[CacheEntry].valid = false

	// Generate the contents for the cache file
	sf := &stampFile{}
	sf.Version = message.GetStampVersion()

	// Pack the new structure based on version #
	switch sf.Version {

	default:
		{
			fmt.Printf("*** Unrecognized stamp version: %d ***\n", sf.Version)
		}

	case StampVersion1:
		{

			if message.Stamp == nil || message.CapturedAtDate == nil || message.CapturedAtTime == nil {
				fmt.Printf("*** Warning - badly formatted v%d stamp ***\n", sf.Version)
			} else {

				sf.Stamp = message.GetStamp()
				sf.CapturedAtDate = message.GetCapturedAtDate()
				sf.CapturedAtTime = message.GetCapturedAtTime()
				if message.Latitude != nil || message.Longitude != nil {
					sf.Latitude = float64(message.GetLatitude())
					sf.Longitude = float64(message.GetLongitude())
					if message.Altitude != nil {
						sf.Altitude = message.GetAltitude()
					} else {
						sf.Altitude = 0.0
					}
				}
				if message.MotionBeganOffset != nil {
					sf.HasMotionOffset = true
					sf.MotionOffset = message.GetMotionBeganOffset()
				}
				if message.Test != nil {
					sf.HasTestMode = true
					sf.TestMode = message.GetTest()
				}

				sfJSON, _ := json.Marshal(sf)

				file := stpFilename(DeviceID)
				fd, err := os.OpenFile(file, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
				if err != nil {
					fmt.Printf("error creating file %s: %s\n", file, err)
				} else {

					// Write and close the file
					fd.WriteString(string(sfJSON))
					fd.Close()

					// Write the cache entry
					cachedDevices[CacheEntry].cache = *sf
					cachedDevices[CacheEntry].valid = true

					// Done
					if debugStamp {
						fmt.Printf("Saved and cached new stamp for %d\n%s\n", DeviceID, string(sfJSON))
					}

				}
			}
		}
	}

	// Remove the stamp fields so they're no longer part of the message
	message.Stamp = nil
	message.StampVersion = nil

	// Done
	return true

}

// Appply the stamp
func stpApply(message *ttproto.Telecast, DeviceID uint32, CacheEntry int) (isValidMessage bool) {

	// If there's no valid cache entry, or if the cache entry is wrong, refresh the cache
	if !cachedDevices[CacheEntry].valid || (cachedDevices[CacheEntry].valid && cachedDevices[CacheEntry].cache.Stamp != message.GetStamp()) {

		// Read the file and delete it
		file, err := ioutil.ReadFile(stpFilename(DeviceID))
		if err != nil {
			cachedDevices[CacheEntry].valid = false
		} else {
			sf := stampFile{}

			// Read it as JSON
			err = json.Unmarshal(file, &sf)
			if err != nil {
				cachedDevices[CacheEntry].valid = false
			} else {

				// Cache it
				cachedDevices[CacheEntry].cache = sf
				cachedDevices[CacheEntry].valid = true

				// Done
				if debugStamp {
					fmt.Printf("Read stamp for %d from file\n", DeviceID)
				}

			}

		}

	}

	// If there's still no valid cache entry, we need to discard this reading
	if !cachedDevices[CacheEntry].valid {
		fmt.Printf("*** No cached stamp for %d when one is needed ***\n", DeviceID)
		return false
	}

	// If there's a valid cache but it is incorrect, do the best we can by using cache as Last Known Good
	if cachedDevices[CacheEntry].cache.Stamp != message.GetStamp() {

		switch cachedDevices[CacheEntry].cache.Version {

		default:
			{
				fmt.Printf("*** Unrecognized stamp version in cache: %d ***\n", cachedDevices[CacheEntry].cache.Version)
				return false
			}

		case StampVersion1:
			{

				// Location is best set to last known good rather than nothing at all
				if message.Latitude == nil || message.Longitude == nil {
					if cachedDevices[CacheEntry].cache.Latitude != 0.0 || cachedDevices[CacheEntry].cache.Longitude != 0.0 {
						lat := float32(cachedDevices[CacheEntry].cache.Latitude)
						message.Latitude = &lat
						lon := float32(cachedDevices[CacheEntry].cache.Longitude)
						message.Longitude = &lon
						if cachedDevices[CacheEntry].cache.Altitude != 0.0 {
							message.Altitude = &cachedDevices[CacheEntry].cache.Altitude
						}
					}
				}

				// Modes are best set to last known good rather than making a mistake
				if message.Test == nil {
					if cachedDevices[CacheEntry].cache.HasTestMode {
						message.Test = &cachedDevices[CacheEntry].cache.TestMode
					}
				}

				// Motion is best set to last known good rather than faking it
				if message.MotionBeganOffset == nil {
					if cachedDevices[CacheEntry].cache.HasMotionOffset {
						message.MotionBeganOffset = &cachedDevices[CacheEntry].cache.MotionOffset
					}
				}

				// Time is best set to current time rather than nothing at all
				substituteCapturedAt := NowInUTC()
				message.CapturedAt = &substituteCapturedAt
				message.CapturedAtDate = nil
				message.CapturedAtTime = nil
				message.CapturedAtOffset = nil

				// Remove the stamp field so that it's no longer part of the message
				message.Stamp = nil

				// Done
				if debugStamp {
					fmt.Printf("Stamp message required by this message must've been lost, so faking it:\n%v\n", message)
				}
				return true

			}

		}

	}

	// We have a valid cache entry for the correct stamp, so use it
	switch cachedDevices[CacheEntry].cache.Version {

	default:
		{
			fmt.Printf("*** Unrecognized stamp version in cache: %d ***\n", cachedDevices[CacheEntry].cache.Version)
			return false
		}

	case StampVersion1:
		{

			// Set Location
			if message.Latitude == nil || message.Longitude == nil {
				if cachedDevices[CacheEntry].cache.Latitude != 0.0 || cachedDevices[CacheEntry].cache.Longitude != 0.0 {
					lat := float32(cachedDevices[CacheEntry].cache.Latitude)
					message.Latitude = &lat
					lon := float32(cachedDevices[CacheEntry].cache.Longitude)
					message.Longitude = &lon
					if cachedDevices[CacheEntry].cache.Altitude != 0.0 {
						message.Altitude = &cachedDevices[CacheEntry].cache.Altitude
					}
				}
			}

			// Set Modes
			if message.Test == nil {
				if cachedDevices[CacheEntry].cache.TestMode {
					message.Test = &cachedDevices[CacheEntry].cache.TestMode
				}
			}

			// Set Motion
			if message.MotionBeganOffset == nil {
				if cachedDevices[CacheEntry].cache.HasMotionOffset {
					message.MotionBeganOffset = &cachedDevices[CacheEntry].cache.MotionOffset
				}
			}

			// Set Time
			if message.CapturedAtOffset != nil {
				message.CapturedAtDate = &cachedDevices[CacheEntry].cache.CapturedAtDate
				message.CapturedAtTime = &cachedDevices[CacheEntry].cache.CapturedAtTime
			}

			// Done
			if debugStamp {
				fmt.Printf("Stamped: %v\n", message)
			}

		}

	}

	// Remove the stamp field so that it's no longer part of the message
	message.Stamp = nil

	return true

}
