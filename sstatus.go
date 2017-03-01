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
    UpdatedAt  string			`json:"when_updated,omitempty"`
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
            // We did not reinitialize it; it's truly empty.
            return true, false, valueEmpty
        }
        return false, true, valueEmpty
    }

    // Try reading the file several times, now that we know it exists.
    // We retry just in case of file system errors on contention.
    for i:=0; i<5; i++ {

        // Read the file and unmarshall if no error
        contents, errRead := ioutil.ReadFile(filename)
        if errRead == nil {
            valueToRead := SafecastServerStatus{}
            errRead = json.Unmarshal(contents, &valueToRead)
            if errRead == nil {
                return true, false, valueToRead
            }
            // Malformed JSON can easily occur because of multiple concurrent
            // writers, and so this self-corrects the situation.
            if true {
                fmt.Printf("*** %s appears to be corrupt - erasing ***\n", filename);
            }
            return true, true, valueEmpty
        }
        err = errRead

        // Delay before trying again
        time.Sleep(5 * time.Second)

    }

    // Error
    if os.IsNotExist(err) {
        return true, true, valueEmpty
    }
    return false, true, valueEmpty

}

// Save the current value into the file
func SafecastWriteServerStatus() {
    var value SafecastServerStatus

    // Read the current value, or a blank value structure if it's blank.
    // If the value isn't available it's because of a nonrecoverable  error.
    // If it was reset, try waiting around a bit until it is fixed.
    for i:=0; i<5; i++ {
        isAvail, isReset, rvalue := SafecastReadServerStatus(TTServeInstanceID)
        value = rvalue
        if !isAvail {
            return
        }
        if !isReset {
            break
        }
        time.Sleep(time.Duration(random(1, 6)) * time.Second)
    }

	// Update the modification date
    value.UpdatedAt = time.Now().UTC().Format("2006-01-02T15:04:05Z")

	// Copy certain fields directly
	value.Tts.AddressIPv4 = stats.AddressIPv4
	value.Tts.AWSInstance = stats.AWSInstance
	
	// Update the data that we accumulate across sessions
	value.Tts.CountUDP += stats.CountUDP
	stats.CountUDP = 0
	value.Tts.CountHTTPDevice += stats.CountHTTPDevice
	stats.CountHTTPDevice = 0
	value.Tts.CountHTTPGateway += stats.CountHTTPGateway
	stats.CountHTTPGateway = 0
	value.Tts.CountHTTPRelay += stats.CountHTTPRelay
	stats.CountHTTPRelay = 0
	value.Tts.CountHTTPRedirect += stats.CountHTTPRedirect
	stats.CountHTTPRedirect = 0
	value.Tts.CountHTTPTTN += stats.CountHTTPTTN
	stats.CountHTTPTTN = 0
	value.Tts.CountMQQTTTN += stats.CountMQQTTTN
	stats.CountMQQTTTN = 0

    // Write it to the file
    filename := SafecastDirectory() + TTServerStatusPath + "/" + TTServeInstanceID + ".json"
    valueJSON, err := json.MarshalIndent(value, "", "    ")
	if err != nil {
		fmt.Printf("Error writing to '%s': \n%v\n", filename, value)
		return
	}
	
    for {

        // Write the value
        fd, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
        if err != nil {
            fmt.Printf("*** Unable to write %s: %v\n", filename, err)
            break
        }
        fd.WriteString(string(valueJSON));
        fd.Close();

        // Delay, to increase the chance that we will catch a concurrent update/overwrite
        time.Sleep(time.Duration(random(1, 6)) * time.Second)

        // Do an integrity check, and re-write the value if necessary
        _, isEmpty, _ := SafecastReadServerStatus(TTServeInstanceID)
        if !isEmpty {
            break
        }
    }

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

    // Done
    return s

}
