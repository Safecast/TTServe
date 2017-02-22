// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Handling of "gateway files", which contain information
// observed as gateway status update messages are sent inbound.
package main

import (
    "os"
	"sort"
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

// Warning behavior
const gatewayWarningAfterMinutes = 90

// Describes every device that has sent us a message
type seenGateway struct {
    gatewayid           string
    seen                time.Time
    everRecentlySeen    bool
    notifiedAsUnseen    bool
    minutesAgo          int64
}
var seenGateways []seenGateway

// Class used to sort seen devices
type ByGatewayKey []seenGateway
func (a ByGatewayKey) Len() int      { return len(a) }
func (a ByGatewayKey) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByGatewayKey) Less(i, j int) bool {
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
func trackGateway(GatewayId string, whenSeen time.Time) {
    var dev seenGateway
    dev.gatewayid = GatewayId

    // Attempt to update the existing entry if we can find it
    found := false
    for i := 0; i < len(seenGateways); i++ {
        if dev.gatewayid == seenGateways[i].gatewayid {
            // Only pay attention to things that have truly recently come or gone
            minutesAgo := int64(time.Now().Sub(whenSeen) / time.Minute)
            if (minutesAgo < deviceWarningAfterMinutes) {
                seenGateways[i].everRecentlySeen = true
                // Notify when the device comes back
                if seenGateways[i].notifiedAsUnseen {
                    minutesAgo := int64(time.Now().Sub(seenGateways[i].seen) / time.Minute)
                    hoursAgo := minutesAgo / 60
                    daysAgo := hoursAgo / 24
                    message := fmt.Sprintf("%d minutes", minutesAgo)
                    switch {
                    case daysAgo >= 2:
                        message = fmt.Sprintf("~%d days", daysAgo)
                    case minutesAgo >= 120:
                        message = fmt.Sprintf("~%d hours", hoursAgo)
                    }
                    sendToSafecastOps(fmt.Sprintf("** NOTE ** Gateway %s has returned after %s away", seenGateways[i].gatewayid, message), SLACK_MSG_UNSOLICITED)
                }
                // Mark as having been seen on the latest date of any file having that time
                seenGateways[i].notifiedAsUnseen = false;
            }
            // Always track the most recent seen date
            if (seenGateways[i].seen.Before(whenSeen)) {
                seenGateways[i].seen = whenSeen
            }
            found = true
            break
        }
    }

    // Add a new array entry if necessary
    if !found {
        dev.seen = whenSeen
        minutesAgo := int64(time.Now().Sub(dev.seen) / time.Minute)
        dev.everRecentlySeen = minutesAgo < deviceWarningAfterMinutes
        dev.notifiedAsUnseen = false
        seenGateways = append(seenGateways, dev)
    }

}

// Update the list of seen devices
func trackAllGateways() {

    // Loop over the file system, tracking all devices
    files, err := ioutil.ReadDir(SafecastDirectory() + TTServerGatewayPath)
    if err == nil {

        // Iterate over each of the values
        for _, file := range files {

            if !file.IsDir() {

                // Extract device ID from filename
                Str0 := file.Name()
                gatewayID := strings.Split(Str0, ".")[0]

                // Track the device
                if gatewayID != "" {
                    trackGateway(gatewayID, file.ModTime())
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
    expiration := time.Now().Add(-(time.Duration(deviceWarningAfterMinutes) * time.Minute))

    // Sweep through all gateways that we've seen
    for i := 0; i < len(seenGateways); i++ {

        // Update when we've last seen the device
        seenGateways[i].minutesAgo = int64(time.Now().Sub(seenGateways[i].seen) / time.Minute)

        // Notify Slack once and only once when a device has expired
        if !seenGateways[i].notifiedAsUnseen && seenGateways[i].everRecentlySeen {
            if seenGateways[i].seen.Before(expiration) {
                seenGateways[i].notifiedAsUnseen = true
                sendToSafecastOps(fmt.Sprintf("** Warning **  Gateway %s hasn't been seen for %d minutes",
                    seenGateways[i].gatewayid,
                    seenGateways[i].minutesAgo), SLACK_MSG_UNSOLICITED)
            }
        }
    }
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

    // Try reading the file several times, now that we know it exists.
    // We retry just in case of file system errors on contention.
    for i:=0; i<5; i++ {

        // Read the file and unmarshall if no error
        contents, errRead := ioutil.ReadFile(filename)
        if errRead == nil {
            valueToRead := SafecastGateway{}
            errRead = json.Unmarshal(contents, &valueToRead)
            if errRead == nil {
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

    // First, age out the expired devices and recompute when last seen
    sendExpiredSafecastGatewaysToSlack()

    // Next sort the device list
    sortedGateways := seenGateways
    sort.Sort(ByGatewayKey(sortedGateways))

    // Build the summary string
    s := ""

    // Finally, sweep over all these devices in sorted order,
    // generating a single large text string to be sent as a Slack message
    for i := 0; i < len(sortedGateways); i++ {
        gatewayID := sortedGateways[i].gatewayid

        // Emit info about the device
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

    // Send it to Slack
    if s == "" {
        s = "No gateways have recently reported"
    }
    sendToSafecastOps(s, SLACK_MSG_REPLY)

}
