// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the HTTP V1 safecast topic
package main

import (
	"bytes"
	"strings"
    "io/ioutil"
    "net/http"
    "fmt"
	"time"
    "io"
    "encoding/json"
)

// Debugging
const redirectDebug bool = true

// Handle inbound HTTP requests from the Teletype Gateway
func inboundWebRedirectHandler(rw http.ResponseWriter, req *http.Request) {
    var sdV1 *SafecastDataV1

	// Remember when it was uploaded to us
    UploadedAt := nowInUTC()
	
	// Get the remote address, and only add this to the count if it's likely from
	// the internal HTTP load balancer.
	remoteAddr, isReal := getRequestorIPv4(req)
	if !isReal {
		remoteAddr = "internal address"
	}

    // Read the body as a byte array
    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
	    stats.Count.HTTP++
        fmt.Printf("Error reading HTTP request body: \n%v\n", req)
        return
    }

    // Decode the request with custom marshaling
    sdV1, err = SafecastV1Decode(bytes.NewReader(body))
    if err != nil {
	    stats.Count.HTTP++
		// Eliminate a bit of the noise caused by load balancer health checks
	    if (isReal && req.RequestURI != "/" && req.RequestURI != "/favicon.ico") {
            if err == io.EOF {
                fmt.Printf("\n%s HTTP request '%s' from %s ignored\n", time.Now().Format(logDateFormat), req.RequestURI, remoteAddr);
            } else {
                fmt.Printf("\n%s HTTP request '%s' from %s ignored: %v\n", time.Now().Format(logDateFormat), req.RequestURI, remoteAddr, err);
            }
            if len(body) != 0 {
                fmt.Printf("%s\n", string(body));
            }
        }
        io.WriteString(rw, fmt.Sprintf("Live Free or Die.\n"))
        return
    }

	// A real request
    stats.Count.HTTP++

	// Process the request URI, looking for things that will indicate "dev"
	method := req.Method
	if method == "" {
		method = "GET"
	}
	isTestMeasurement := strings.Contains(req.RequestURI, "test")

	// If debugging, display it
	if redirectDebug {
		fmt.Printf("*** Redirect %s test:%v: %s\n", method, isTestMeasurement, req.RequestURI)
		fmt.Printf("*** Redirect received:\n%s\n", string(body))
		fmt.Printf("*** Redirect decoded to V1:\n%v\n", sdV1)
	}

    // Convert to current data format
    deviceID, deviceType, sd := SafecastReformat(sdV1, isTestMeasurement)
    if (deviceID == 0) {
        fmt.Printf("%s\n%v\n", string(body), sdV1);
        return
    }

	// If debugging, display it
	if redirectDebug {
	    scJSON, _ := json.Marshal(sd)
		fmt.Printf("*** Redirect reformatted to V2:\n%s\n", scJSON)
	}

	// Report where we got it from, and when we got it
    var svc Service
	svc.UploadedAt = &UploadedAt
	requestor, _ := getRequestorIPv4(req)
    transportStr := deviceType+":" + requestor
    svc.Transport = &transportStr
    sd.Service = &svc

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

	// Generate the CRC of the original device data
	hash := HashSafecastData(sd)
	sd.Service.HashMd5 = &hash

	// Add info about the server instance that actually did the upload
	sd.Service.Handler = &TTServeInstanceID

    // For backward compatibility,post it to V1 with an URL that is preserved.  Also do normal post
    SafecastV1Upload(body, req.RequestURI, method, isTestMeasurement, *sdV1.Unit, fmt.Sprintf("%.3f", *sdV1.Value))
    SafecastUpload(sd)
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
