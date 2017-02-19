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
    "os"
    "fmt"
    "io/ioutil"
    "encoding/json"
    "github.com/rayozzie/teletype-proto/golang"
)

// Debugging
const debugStamp = false

// Stamp versions.  Unlike the client, the support
// for downlevel stamp version must be kept here forever.
const STAMP_VERSION_1 = 1

// Cache file format
type stampFile struct {
    Version         uint32  `json:"Version,omitempty"`
    Stamp           uint32  `json:"Stamp,omitempty"`
    Latitude        float32 `json:"Latitude,omitempty"`
    Longitude       float32 `json:"Longitude,omitempty"`
    Altitude        int32   `json:"Altitude,omitempty"`
    CapturedAtDate  uint32  `json:"CapturedAtDate,omitempty"`
    CapturedAtTime  uint32  `json:"CapturedAtTime,omitempty"`
}

// Describes every device that has sent us a message
type cachedDevice struct {
    deviceid            uint32
    valid               bool
    cache               stampFile
}
var cachedDevices []cachedDevice

// Statics
var substituteCapturedAt string

// Construct the path of a command file
func stampFilename(DeviceID uint32) string {
    directory := SafecastDirectory()
    file := directory + TTServerStampPath + "/" + fmt.Sprintf("%d", DeviceID) + ".json"
    return file
}

// Set or apply the stamp
func stampSetOrApply(message *teletype.Telecast) (isValidMessage bool) {
    var CacheEntry int = 0

    // Device ID is required here, but that doesn't mean it's not a valid message
    if message.DeviceID == nil {
        return true;
    }
    DeviceID := message.GetDeviceID()

    // Find or create the cache entry for this device
    found := false
    for CacheEntry = 0; CacheEntry < len(cachedDevices); CacheEntry++ {
        if DeviceID == cachedDevices[CacheEntry].deviceid {
			found = true
            break;
        }
    }
    if (!found) {
        var entry cachedDevice
        entry.deviceid = DeviceID
        entry.valid = false
        cachedDevices = append(cachedDevices, entry)
        CacheEntry = len(cachedDevices)-1
        if debugStamp {
            fmt.Printf("Added new device cache entry for never-before seen %d: %d\n", DeviceID, CacheEntry)
        }
    }

    // If this is a "set stamp" operation, do it
    if message.StampVersion != nil {
        return(stampSet(message, DeviceID, CacheEntry))
    }


    // If this isn't a "stamp this message" operation, exit
    if message.Stamp != nil {
        return(stampApply(message, DeviceID, CacheEntry))
    }

    // Neither a stamper or a stampee
    return true

}

// Set or apply the stamp
func stampSet(message *teletype.Telecast, DeviceID uint32, CacheEntry int) (isValidMessage bool) {

    // Regardless of whatever else happens, we need to invalidate the cache
    cachedDevices[CacheEntry].valid = false

    // Generate the contents for the cache file
    sf := &stampFile{}
    sf.Version = message.GetStampVersion()

    // Pack the new structure based on version #
    switch sf.Version {

    default: {
        fmt.Printf("*** Unrecognized stamp version: %d ***\n", sf.Version)
    }

    case STAMP_VERSION_1: {

        if (message.Stamp == nil || message.Latitude == nil || message.Longitude == nil || message.CapturedAtDate == nil || message.CapturedAtTime == nil) {
            fmt.Printf("*** Warning - badly formatted v%d stamp ***\n", sf.Version)
        } else {

            sf.Stamp = message.GetStamp()
            sf.CapturedAtDate = message.GetCapturedAtDate()
            sf.CapturedAtTime = message.GetCapturedAtTime()
            sf.Latitude = message.GetLatitude()
            sf.Longitude = message.GetLongitude()
            if message.Altitude != nil {
                sf.Altitude = message.GetAltitude()
            } else {
                sf.Altitude = 0.0
            }
            sfJSON, _ := json.Marshal(sf)

            file := stampFilename(DeviceID)
            fd, err := os.OpenFile(file, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0666)
            if (err != nil) {
                fmt.Printf("error creating file %s: %s\n", file, err);
            } else {

                // Write and close the file
                fd.WriteString(string(sfJSON));
                fd.Close();

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

// Set or apply the stamp
func stampApply(message *teletype.Telecast, DeviceID uint32, CacheEntry int) (isValidMessage bool) {

    // If there's no valid cache entry, or if the cache entry is wrong, refresh the cache
    if !cachedDevices[CacheEntry].valid || (cachedDevices[CacheEntry].valid && cachedDevices[CacheEntry].cache.Stamp != message.GetStamp()) {

        // Read the file and delete it
        file, err := ioutil.ReadFile(stampFilename(DeviceID))
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
    if (cachedDevices[CacheEntry].cache.Stamp != message.GetStamp()) {

        switch cachedDevices[CacheEntry].cache.Version {

        default: {
            fmt.Printf("*** Unrecognized stamp version in cache: %d ***\n", cachedDevices[CacheEntry].cache.Version)
            return false
        }

        case STAMP_VERSION_1: {

            // Location is best set to last known good rather than nothing at all
            message.Latitude = &cachedDevices[CacheEntry].cache.Latitude
            message.Longitude = &cachedDevices[CacheEntry].cache.Longitude
            message.Altitude = &cachedDevices[CacheEntry].cache.Altitude

            // Time is best set to current time rather than nothing at all
            substituteCapturedAt := nowInUTC()
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

    default: {
        fmt.Printf("*** Unrecognized stamp version in cache: %d ***\n", cachedDevices[CacheEntry].cache.Version)
        return false
    }

    case STAMP_VERSION_1: {

        // Location is best set to last known good rather than nothing at all
        message.Latitude = &cachedDevices[CacheEntry].cache.Latitude
        message.Longitude = &cachedDevices[CacheEntry].cache.Longitude
        message.Altitude = &cachedDevices[CacheEntry].cache.Altitude

        // Time is best set to current time rather than nothing at all
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
