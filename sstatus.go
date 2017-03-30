// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Handling of "server status files"
package main

import (
    "os"
    "time"
    "fmt"
    "io/ioutil"
    "encoding/json"
)

// The data structure for the "Server Status" files
type SafecastServerStatus struct {
    UpdatedAt  string           `json:"when_updated,omitempty"`
    Tts         TTServeStatus   `json:"current_values,omitempty"`
}

// Get the current value
func SafecastReadServerStatus(serverId string) (isAvail bool, isReset bool, sv SafecastServerStatus) {
    valueEmpty := SafecastServerStatus{}
    valueEmpty.UpdatedAt = time.Now().UTC().Format("2006-01-02T15:04:05Z")
    valueEmpty.Tts = stats

    // Generate the filename, which we'll use twice
    filename := SafecastDirectory() + TTServerStatusPath + "/" + serverId + ".json"

    // If the file doesn't exist, don't even try
    _, err := os.Stat(filename)
    if err != nil {
        if os.IsNotExist(err) {
            // We did not reinitialize it - it's truly empty.
            return true, false, valueEmpty
        }
        return false, true, valueEmpty
    }

    // Read the file
    contents, errRead := ioutil.ReadFile(filename)
    if errRead != nil {
        return false, true, valueEmpty
    }

    // Unmarshal
    valueToRead := SafecastServerStatus{}
    err = json.Unmarshal(contents, &valueToRead)
    if err != nil {
        // Malformed JSON
        fmt.Printf("*** %s appears to be corrupt - erasing ***\n", filename)
        return true, true, valueEmpty
    }

    // Done
    return true, false, valueToRead
}


// Save the current value into the file
func SafecastWriteServerStatus() {

    // Read the current value
    isAvail, _, value := SafecastReadServerStatus(TTServeInstanceID)
    if !isAvail {
        return
    }

    // Update the modification date
    value.UpdatedAt = time.Now().UTC().Format("2006-01-02T15:04:05Z")

    // By default, copy all Tts fields
    prevCount := value.Tts.Count
    value.Tts = stats

    // For certain fields, be additive to the prior values
    value.Tts.Count.Restarts += prevCount.Restarts
    stats.Count.Restarts = 0
    value.Tts.Count.UDP += prevCount.UDP
    stats.Count.UDP = 0
    value.Tts.Count.HTTP += prevCount.HTTP
    stats.Count.HTTP = 0
    value.Tts.Count.HTTPSlack += prevCount.HTTPSlack
    stats.Count.HTTPSlack = 0
    value.Tts.Count.HTTPGithub += prevCount.HTTPGithub
    stats.Count.HTTPGithub = 0
    value.Tts.Count.HTTPGUpdate += prevCount.HTTPGUpdate
    stats.Count.HTTPGUpdate = 0
    value.Tts.Count.HTTPDevice += prevCount.HTTPDevice
    stats.Count.HTTPDevice = 0
    value.Tts.Count.HTTPGateway += prevCount.HTTPGateway
    stats.Count.HTTPGateway = 0
    value.Tts.Count.HTTPRelay += prevCount.HTTPRelay
    stats.Count.HTTPRelay = 0
    value.Tts.Count.HTTPRedirect += prevCount.HTTPRedirect
    stats.Count.HTTPRedirect = 0
    value.Tts.Count.HTTPTTN += prevCount.HTTPTTN
    stats.Count.HTTPTTN = 0
    value.Tts.Count.MQQTTTN += prevCount.MQQTTTN
    stats.Count.MQQTTTN = 0

    // Write it to the file
    filename := SafecastDirectory() + TTServerStatusPath + "/" + TTServeInstanceID + ".json"
    valueJSON, err := json.MarshalIndent(value, "", "    ")
    if err != nil {
        fmt.Printf("*** Error marshaling for '%s': \n%v\n", filename, value)
        return
    }

    // Write the value
    fd, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
    if err != nil {
        fmt.Printf("*** Unable to write %s: %v\n", filename, err)
		return
    }
    fd.WriteString(string(valueJSON))
    fd.Close()

}

// Get a running total of server stats
var lastCount TTServeCounts
var firstSummary = true
func SafecastSummarizeStatsDelta() string {

    // First, make sure that they're up to date on the service
    SafecastWriteServerStatus()

    // Read them
    isAvail, isReset, value := SafecastReadServerStatus(TTServeInstanceID)
    if !isAvail || isReset {
        return ""
    }

    // Extract the current counts, and update them for next iteration
    prevCount := lastCount
    thisCount := value.Tts.Count
    lastCount = thisCount

    // If this is the first time through, all we're doing is initializing lastCount
    if firstSummary {
        firstSummary = false
        return ""
    }

    // Compute the difference
    var diff = TTServeCounts{}
    diff.Restarts = thisCount.Restarts - prevCount.Restarts
    diff.UDP = thisCount.UDP - prevCount.UDP
    diff.HTTP = thisCount.HTTP - prevCount.HTTP
    diff.HTTPSlack = thisCount.HTTPSlack - prevCount.HTTPSlack
    diff.HTTPGithub = thisCount.HTTPGithub - prevCount.HTTPGithub
    diff.HTTPGUpdate = thisCount.HTTPGUpdate - prevCount.HTTPGUpdate
    diff.HTTPDevice = thisCount.HTTPDevice - prevCount.HTTPDevice
    diff.HTTPGateway = thisCount.HTTPGateway - prevCount.HTTPGateway
    diff.HTTPRelay = thisCount.HTTPRelay - prevCount.HTTPRelay
    diff.HTTPRedirect = thisCount.HTTPRedirect - prevCount.HTTPRedirect
    diff.HTTPTTN = thisCount.HTTPTTN - prevCount.HTTPTTN
    diff.MQQTTTN = thisCount.MQQTTTN - prevCount.MQQTTTN

    // Return the jsonified summary
    statsdata, err := json.Marshal(&diff)
    if err != nil {
        return ""
    }

    return string(statsdata)

}

// Get summary of a server
func SafecastGetServerSummary(ServerId string, bol string) string {

    // Read the file
    isAvail, _, value := SafecastReadServerStatus(ServerId)
    if !isAvail {
        return ""
    }

    // Build the summary
    s := ""

    // When active
    s += fmt.Sprintf("alive for %s", Ago(value.Tts.Started))

	// If this is the current server, point that out
	if ServerId == TTServeInstanceID {
		s += " *"
	}

    // Done
    return s

}
