// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Handling of "gateway files", which contain information
// observed as gateway status update messages are sent inbound.
package main

import (
    "os"
    "time"
    "fmt"
    "strings"
    "strconv"
    "io/ioutil"
    "encoding/json"
)

// The data structure for the "Gateway Status" files
type SafecastGatewayStatus struct {
    UpdatedAt   string      `json:"when_updated,omitempty"`
    Ttg         TTGateReq   `json:"current_values,omitempty"`
	// for backward compatibility - you can remove after 2017-04
    UploadedAt  string      `json:"when_uploaded,omitempty"`
}

// Get the current value
func SafecastReadGatewayStatus(gatewayId string) (isAvail bool, isReset bool, sv SafecastGatewayStatus) {
    valueEmpty := SafecastGatewayStatus{}
    valueEmpty.UpdatedAt = time.Now().UTC().Format("2006-01-02T15:04:05Z")
    valueEmpty.Ttg.GatewayId = gatewayId

    // Generate the filename, which we'll use twice
    filename := SafecastDirectory() + TTGatewayStatusPath + "/" + gatewayId + ".json"

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
            valueToRead := SafecastGatewayStatus{}
            errRead = json.Unmarshal(contents, &valueToRead)
            if errRead == nil {
				// Backward compatbility with old field names
				if valueToRead.UploadedAt != "" {
					valueToRead.UpdatedAt = valueToRead.UploadedAt
					valueToRead.UploadedAt = ""
				}
                return true, false, valueToRead
            }
            // Malformed JSON can easily occur because of multiple concurrent
            // writers, and so this self-corrects the situation.
            if false {
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

// Save the last value in a file
func SafecastWriteGatewayStatus(ttg TTGateReq) {
    var value SafecastGatewayStatus

    // Read the current value, or a blank value structure if it's blank.
    // If the value isn't available it's because of a nonrecoverable  error.
    // If it was reset, try waiting around a bit until it is fixed.
    for i:=0; i<5; i++ {
        isAvail, isReset, rvalue := SafecastReadGatewayStatus(ttg.GatewayId)
        value = rvalue
        if !isAvail {
            return
        }
        if !isReset {
            break
        }
        time.Sleep(time.Duration(random(1, 6)) * time.Second)
    }

    // Copy over all the values directly.  If someday we need to aggregate
    // values rather than replace them, this is the place to do it
    value.Ttg = ttg

    // Update the uploaded at
    value.UpdatedAt = time.Now().UTC().Format("2006-01-02T15:04:05Z")

    // Write it to the file
    filename := SafecastDirectory() + TTGatewayStatusPath + "/" + ttg.GatewayId + ".json"
    valueJSON, _ := json.MarshalIndent(value, "", "    ")


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
        _, isEmpty, _ := SafecastReadGatewayStatus(ttg.GatewayId)
        if !isEmpty {
            break
        }
    }

}

// Get summary of a device
func SafecastGetGatewaySummary(GatewayId string, bol string) (Label string, Loc string, Summary string) {

    // Read the file
    isAvail, _, value := SafecastReadGatewayStatus(GatewayId)
    if !isAvail {
        return "", "", ""
    }

    // Get the label
    label := value.Ttg.GatewayName;

    // Get a summary of the location
    loc := fmt.Sprintf("%s, %s", value.Ttg.IPInfo.City, value.Ttg.IPInfo.Country)
    if value.Ttg.IPInfo.City == "" {
        loc = value.Ttg.IPInfo.Country
    }

    // Build the summary
    s := ""

    // When active
    whenSeen, err := time.Parse("2006-01-02T15:04:05Z", value.UpdatedAt)
    if err == nil {
        minutesAgo := int64(time.Now().Sub(whenSeen) / time.Minute)
        if minutesAgo > 60 {
            s += bol
            s += fmt.Sprintf("Last seen %s ago", Ago(whenSeen))
        }
    }

    // Messages Received
    if value.Ttg.MessagesReceived != 0 {

        if s != "" {
            s += "\n"
        }

        s += bol

        if value.Ttg.DevicesSeen == "" {
            s += fmt.Sprintf("%d messages received", value.Ttg.MessagesReceived)
        } else {
            s += fmt.Sprintf("%d received from ", value.Ttg.MessagesReceived)

            // Iterate over devices
            devicelist := value.Ttg.DevicesSeen
            devices := strings.Split(devicelist, ",")
            for _, d := range devices {
                i64, _ := strconv.ParseUint(d, 10, 32)
                deviceID := uint32(i64)
                s += fmt.Sprintf("<http://%s%s%d|%010d> ", TTServerHTTPAddress, TTServerTopicDeviceStatus, deviceID, deviceID)
            }
        }
    }

    // Done
    return label, loc, s

}
