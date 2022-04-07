// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Handling of "gateway files", which contain information
// observed as gateway status update messages are sent inbound.
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// GatewayStatus is The data structure for the "Gateway Status" files
type GatewayStatus struct {
	UpdatedAt string    `json:"when_updated,omitempty"`
	Ttg       TTGateReq `json:"current_values,omitempty"`
	// for backward compatibility - you can remove after 2017-04
	UploadedAt string `json:"when_uploaded,omitempty"`
	// Our view of the IP info
	IPInfo IPInfoData `json:"gateway_location,omitempty"`
}

// ReadGatewayStatus gets the current value
func ReadGatewayStatus(gatewayID string) (isAvail bool, isReset bool, sv GatewayStatus) {
	valueEmpty := GatewayStatus{}
	valueEmpty.UpdatedAt = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	valueEmpty.Ttg.GatewayID = gatewayID

	// Generate the filename, which we'll use twice
	filename := SafecastDirectory() + TTGatewayStatusPath + "/" + gatewayID + ".json"

	// If the file doesn't exist, don't even try
	_, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// We did not reinitialize it - it's truly empty.
			return true, false, valueEmpty
		}
		return false, true, valueEmpty
	}

	// Try reading the file several times, now that we know it exists.
	// We retry just in case of file system errors on contention.
	for i := 0; i < 5; i++ {

		// Read the file and unmarshall if no error
		contents, errRead := ioutil.ReadFile(filename)
		if errRead == nil {
			valueToRead := GatewayStatus{}
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
				fmt.Printf("*** %s appears to be corrupt - erasing ***\n", filename)
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

// WriteGatewayStatus saves the last value in a file
func WriteGatewayStatus(ttg TTGateReq, IP string) {
	var value GatewayStatus

	// Read the current value, or a blank value structure if it's blank.
	// If the value isn't available it's because of a nonrecoverable  error.
	// If it was reset, try waiting around a bit until it is fixed.
	for i := 0; i < 5; i++ {
		isAvail, isReset, rvalue := ReadGatewayStatus(ttg.GatewayID)
		value = rvalue
		if !isAvail {
			return
		}
		if !isReset {
			break
		}
		time.Sleep(time.Duration(Random(1, 6)) * time.Second)
	}

	// Copy over all the values directly.  If someday we need to aggregate
	// values rather than replace them, this is the place to do it
	value.Ttg = ttg

	// Update the uploaded at
	value.UpdatedAt = time.Now().UTC().Format("2006-01-02T15:04:05Z")

	// If the new one doesn't have a successful IPInfo, we'd like to fetch it

	// If the IP info isn't filled in, fill it in.  This will only happen once.
	needUpdate := false
	if value.IPInfo.Status == "" {
		fmt.Printf("*** First-time IPInfo update for gateway %s\n", IP)
		needUpdate = true
	}
	if value.IPInfo.IP.String() != IP {
		fmt.Printf("*** Updating gateway IPInfo because of IP change from %s to %s\n", value.IPInfo.IP.String(), IP)
		needUpdate = true
	}
	if needUpdate {
		response, err := http.Get("http://ip-api.com/json/" + IP)
		if err == nil {
			defer response.Body.Close()
			contents, err := ioutil.ReadAll(response.Body)
			if err == nil {
				var info IPInfoData
				err = json.Unmarshal(contents, &info)
				if err == nil {
					value.IPInfo = info
				}
			}
		}
	}

	// Write it to the file
	filename := SafecastDirectory() + TTGatewayStatusPath + "/" + ttg.GatewayID + ".json"
	valueJSON, _ := json.MarshalIndent(value, "", "    ")

	for {

		// Write the value
		fd, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
		if err != nil {
			fmt.Printf("*** Unable to write %s: %v\n", filename, err)
			break
		}
		fd.WriteString(string(valueJSON))
		fd.Close()

		// Delay, to increase the chance that we will catch a concurrent update/overwrite
		time.Sleep(time.Duration(Random(1, 6)) * time.Second)

		// Do an integrity check, and re-write the value if necessary
		_, isEmpty, _ := ReadGatewayStatus(ttg.GatewayID)
		if !isEmpty {
			break
		}
	}

}

// GetGatewaySummary gets summary of a device
func GetGatewaySummary(GatewayID string, bol string) (Summary string, Label string) {

	// Read the file
	isAvail, _, value := ReadGatewayStatus(GatewayID)
	if !isAvail {
		return "", ""
	}

	// Get the label
	label := value.Ttg.GatewayName

	// Get a summary of the location
	loc := fmt.Sprintf("%s, %s", value.IPInfo.City, value.IPInfo.Country)
	if value.IPInfo.City == "" {
		loc = value.IPInfo.Country
	}

	// Build the summary
	s := ""

	// How long ago
	whenSeen, err := time.Parse("2006-01-02T15:04:05Z", value.UpdatedAt)
	if err == nil {
		s += fmt.Sprintf("%s ago", Ago(whenSeen))
	}

	// Label
	if label != "" {
		s += fmt.Sprintf(" \"%s\"", label)
	}

	// Location
	if loc != "" {
		s += "\n" + bol + loc
	}

	// Received
	if value.Ttg.MessagesReceived != 0 {
		s += "\n" + bol

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
	return s, label

}
