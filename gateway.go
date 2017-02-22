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

// The data structure for the "Value" files
type SafecastGateway struct {
    UploadedAt  string      `json:"when_uploaded,omitempty"`
    Ttg         TTGateReq   `json:"current_values,omitempty"`
}

// Get the current value
func SafecastReadGateway(gatewayId string) (isAvail bool, isReset bool, sv SafecastGateway) {
    valueEmpty := SafecastGateway{}
    valueEmpty.UploadedAt = time.Now().UTC().Format("2006-01-02T15:04:05Z")
    valueEmpty.Ttg.GatewayId = gatewayId

    // Generate the filename, which we'll use twice
    filename := SafecastDirectory() + TTServerGatewayPath + "/" + gatewayId + ".json"

    // If the file doesn't exist, don't even try
    _, err := os.Stat(filename)
    if err != nil {
        if os.IsNotExist(err) {
			// We did not reinitialize it; it's truly empty.
            return true, false, valueEmpty
        }
        return false, true, valueEmpty
    }

    // Try reading the file several times, now that we know it exists,
    // just to deal with issues of contention
    for i:=0; i<5; i++ {

        // Read the file and unmarshall if no error
        contents, errRead := ioutil.ReadFile(filename)
        if errRead == nil {
            valueToRead := SafecastGateway{}
            errRead = json.Unmarshal(contents, &valueToRead)
            if errRead == nil {
                return true, false, valueToRead
            }
			fmt.Printf("*** %s appears to be corrupt - erasing ***\n", filename);
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
func SafecastWriteGateway(ttg TTGateReq) {
	var value SafecastGateway
	
    // Read the current value, or a blank value structure if it's blank.
	// If the value isn't available it's because of a nonrecoverable  error.
	// If it was reset, try waiting around a bit until it is fixed.
	for i:=0; i<5; i++ {
	    isAvail, isReset, rvalue := SafecastReadGateway(ttg.GatewayId)
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
    value.UploadedAt = time.Now().UTC().Format("2006-01-02T15:04:05Z")

    // Write it to the file
    filename := SafecastDirectory() + TTServerGatewayPath + "/" + ttg.GatewayId + ".json"
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
	    _, isEmpty, _ := SafecastReadGateway(ttg.GatewayId)
		if !isEmpty {
			break
		}
    }

}

// Get summary of a device
func SafecastGetGatewaySummary(GatewayId string, bol string) (Label string, Loc string, Summary string) {

    // Read the file
    isAvail, _, value := SafecastReadGateway(GatewayId)
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
    whenSeen, err := time.Parse("2006-01-02T15:04:05Z", value.UploadedAt)
	if err == nil {
	    minutesAgo := int64(time.Now().Sub(whenSeen) / time.Minute)
		if minutesAgo > 60 {
	        s += bol
			s += fmt.Sprintf("Last seen %d minutes ago", minutesAgo)
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
                s += fmt.Sprintf("<http://%s%s%d|%010d> ", TTServerHTTPAddress, TTServerTopicValue, deviceID, deviceID)
            }
        }
    }

    // Done
    return label, loc, s

}


// Get a summary of devices that are older than this many minutes ago
func sendSafecastGatewaySummaryToSlack() {

    // Build the summary string
    s := ""

    // Loop over the file system, tracking all devices
    files, err := ioutil.ReadDir(SafecastDirectory() + TTServerGatewayPath)
    if err == nil {

        // Iterate over each of the values
        for _, file := range files {

            if !file.IsDir() {

                // Extract gateway ID from filename
                Str0 := file.Name()
                gatewayID := strings.Split(Str0, ".")[0]

                // Track the device
                if gatewayID != "" {
                    label, loc, summary := SafecastGetGatewaySummary(gatewayID, "    ")
                    if summary != "" {
                        if s != "" {
                            s += fmt.Sprintf("\n");
                        }
                        s += fmt.Sprintf("<http://%s%s%s|%s>", TTServerHTTPAddress, TTServerTopicGateway2, gatewayID, gatewayID)
                        if loc != "" {
                            s += fmt.Sprintf(" %s", loc)
                        }
                        if label != "" {
                            s += fmt.Sprintf(" \"%s\"", label)
                        }
                        if summary != "" {
                            s += fmt.Sprintf("\n%s", summary)
                        }
                    }
                }

            }
        }
    }

    // Send it to Slack
    if s == "" {
        s = "No gateways have recently reported"
    }
    sendToSafecastOps(s, SLACK_MSG_REPLY)

}
