// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the HTTP V1 safecast topic
package main

import (
	"bytes"
    "io/ioutil"
    "net/http"
    "fmt"
	"time"
    "io"
)

// Handle inbound HTTP requests from the Teletype Gateway
func inboundWebRedirectHandler(rw http.ResponseWriter, req *http.Request) {
    var sdV1 *SafecastDataV1
    stats.Count.HTTP++

    // Read the body as a byte array
    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        fmt.Printf("Error reading HTTP request body: \n%v\n", req)
        return
    }

    // Decode the request with custom marshaling
    sdV1, err = SafecastV1Decode(bytes.NewReader(body))
    if err != nil {
		remoteAddr := getRequestorIPv4(req)
//		This check just makes it a bit less noisy at the console
//      if (req.RequestURI != "/" && req.RequestURI != "/favicon.ico") {
		if true {
            if err == io.EOF {
                fmt.Printf("\n%s HTTP request '%s' from %s ignored\n", time.Now().Format(logDateFormat), req.RequestURI, remoteAddr);
            } else {
                fmt.Printf("\n%s HTTP request '%s' from %s ignored: %v\n", time.Now().Format(logDateFormat), req.RequestURI, remoteAddr, err);
            }
            if len(body) != 0 {
                fmt.Printf("%s\n", string(body));
            }
        }
        if (req.RequestURI == "/") {
            io.WriteString(rw, fmt.Sprintf("Live Free or Die. (%s)\n", ThisServerAddressIPv4))
        }
        return
    }

    // Convert to current data format
    deviceID, deviceType, sd := SafecastReformat(sdV1)
    if (deviceID == 0) {
        fmt.Printf("%s\n%v\n", string(body), sdV1);
        return
    }

	// Report where we got it from
    var net Net
    transportStr := deviceType+":" + getRequestorIPv4(req)
    net.Transport = &transportStr
    sd.Net = &net

    fmt.Printf("\n%s Received payload for %d from %s\n", time.Now().Format(logDateFormat), sd.DeviceId, transportStr)
    fmt.Printf("%s\n", body)

	// If the data doesn't have anything useful in it, optimize it completely away.  This is
	// observed to happen for Safecast Air from time to time
	if sd.Opc == nil && sd.Pms == nil && sd.Env == nil && sd.Lnd == nil && sd.Bat == nil && sd.Dev == nil {
	    fmt.Printf("%s *** Ignoring because message contains no data\n", time.Now().Format(logDateFormat))
		return
	}

    // Fill in the minimums so as to prevent faults in V1 processing
    if sdV1.Unit == nil {
        s := ""
        sdV1.Unit = &s
    }
    if sdV1.Value == nil {
        v := float32(0)
        sdV1.Value = &v
    }

    // For backward compatibility,post it to V1 with an URL that is preserved.  Also do normal post
    UploadedAt := nowInUTC()
    SafecastV1Upload(body, SafecastV1UploadURL+req.RequestURI, *sdV1.Unit, fmt.Sprintf("%.3f", *sdV1.Value))
    SafecastUpload(UploadedAt, sd)
    SafecastWriteToLogs(UploadedAt, sd)
    stats.Count.HTTPRedirect++

    // It is an error if there is a pending outbound payload for this device, so remove it and report it
    isAvailable, _ := TelecastOutboundPayload(deviceID)
    if (isAvailable) {
        sendToSafecastOps(fmt.Sprintf("%d is not capable of processing commands (cancelled)\n", deviceID), SLACK_MSG_UNSOLICITED)
    }

    // Send a reply to Pointcast saying that the request was processed acceptably.
    // If we fail to do this, Pointcast goes into an infinite reboot loop with comms errors
    // due to GetMeasurementReply() returning 0.
    io.WriteString(rw, "{\"id\":00000001}\r\n")


}
