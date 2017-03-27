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
const redirectDebug bool = false

// Handle inbound HTTP requests from the Teletype Gateway
func inboundWebRedirectHandler(rw http.ResponseWriter, req *http.Request) {
    var sdV1 *SafecastDataV1
    var sdV1Emit *SafecastDataV1ToEmit
    var body []byte
    var err error

    // Remember when it was uploaded to us
    UploadedAt := nowInUTC()

    // Process the request URI, looking for things that will indicate "dev"
    method := req.Method
    if method == "" {
        method = "GET"
    }

    // See if this is a test measurement
    isTestMeasurement := strings.Contains(req.RequestURI, "test")

    // Get the remote address, and only add this to the count if it's likely from
    // the internal HTTP load balancer.
    remoteAddr, isReal := getRequestorIPv4(req)
    if !isReal {
        remoteAddr = "internal address"
    }

    // If this is a GET (a V1 Pointcast 3G), convert RequestURI into valid json
    RequestURI := req.RequestURI
    if method == "GET" {
        // Before: /scripts/shorttest.php?api_key=q1LKu7RQ8s5pmyxunnDW&lat=34.4883&lon=136.165&cpm=0&id=100031&alt=535
        //  After: {"unit":"cpm","latitude":"34.4883","longitude":"136.165","value":"0","device_id":"100031","height":"535"}
        str1 := strings.SplitN(RequestURI, "&", 2)
        RequestURI = str1[0]
        if len(str1) == 1 {
            body = []byte("")
        } else {
	        str2 := str1[len(str1)-1]
            str3 := "unit=cpm&" + str2
            str4 := strings.Replace(str3, "lat=", "latitude=", 1)
            str5 := strings.Replace(str4, "lon=", "longitude=", 1)
            str6 := strings.Replace(str5, "alt=", "height=", 1)
            str7 := strings.Replace(str6, "cpm=", "value=", 1)
            str8 := strings.Replace(str7, "id=", "device_id=", 1)
            str9 := strings.Replace(str8, "=", "\":\"", -1)
            str10 := strings.Replace(str9, "&", "\",\"", -1)
            body = []byte("{\"" + str10 + "\"}")
        }

    } else {

        // Read the body as a byte array
        body, err = ioutil.ReadAll(req.Body)
        if err != nil {
            stats.Count.HTTP++
            fmt.Printf("Error reading HTTP request body: \n%v\n", req)
            return

        }
    }

    // Decode the request with custom marshaling
    sdV1, sdV1Emit, err = SafecastV1Decode(bytes.NewReader(body))
    if err != nil {
        stats.Count.HTTP++
        // Eliminate a bit of the noise caused by load balancer health checks
        if (isReal && req.RequestURI != "/" && req.RequestURI != "/favicon.ico") {
            if err == io.EOF {
                fmt.Printf("\n%s HTTP request '%s' from %s ignored\n", time.Now().Format(logDateFormat), RequestURI, remoteAddr);
            } else {
                fmt.Printf("\n%s HTTP request '%s' from %s ignored: %v\n", time.Now().Format(logDateFormat), RequestURI, remoteAddr, err);
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

    // Fill in the minimum defaults
    if sdV1.Unit == nil {
        s := "cpm"
        sdV1.Unit = &s
        sdV1Emit.Unit = &s
    }
    if sdV1.Value == nil {
        f32 := float32(0)
        sdV1.Value = &f32
        str := fmt.Sprintf("%f", f32)
        sdV1Emit.Value = &str
    }
    if sdV1.CapturedAt == nil {
        capturedAt := nowInUTC()
        sdV1.CapturedAt = &capturedAt
        sdV1Emit.CapturedAt = &capturedAt
    }

    // Convert it to text to emit
    sdV1EmitJSON, _ := json.Marshal(sdV1Emit)

    // If debugging, display it
    if redirectDebug {
        fmt.Printf("\n\n*** Redirect %s test:%v %s\n", method, isTestMeasurement, req.RequestURI)
        fmt.Printf("*** Redirect received:\n%s\n", string(body))
        fmt.Printf("*** Redirect decoded to V1:\n%s\n", sdV1EmitJSON)
    }

    // For backward compatibility,post it to V1 with an URL that is preserved.  Also do normal post
    _, result := SafecastV1Upload(sdV1EmitJSON, RequestURI, isTestMeasurement, *sdV1.Unit, fmt.Sprintf("%.3f", *sdV1.Value))

    // Send a reply to Pointcast saying that the request was processed acceptably.
    // If we fail to do this, Pointcast goes into an infinite reboot loop with comms errors
    // due to GetMeasurementReply() returning 0.
    io.WriteString(rw, result)

    // Convert to current data format
    deviceID, deviceType, sd := SafecastReformat(sdV1, isTestMeasurement)
    if (deviceID == 0) {
        fmt.Printf("%s\n%v\n", string(body), sdV1);
        return
    }

    // If debugging, display it
    if redirectDebug {
        scJSON, _ := json.Marshal(sd)
        fmt.Printf("*** Redirect reformatted to V2:\n%s\n\n\n", scJSON)
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

    // Generate the CRC of the original device data
    hash := HashSafecastData(sd)
    sd.Service.HashMd5 = &hash

    // Add info about the server instance that actually did the upload
    sd.Service.Handler = &TTServeInstanceID

    // Post to V2
    SafecastUpload(sd)
    SafecastWriteToLogs(UploadedAt, sd)
    stats.Count.HTTPRedirect++

    // It is an error if there is a pending outbound payload for this device, so remove it and report it
    isAvailable, _ := TelecastOutboundPayload(deviceID)
    if (isAvailable) {
        go sendToSafecastOps(fmt.Sprintf("%d is not capable of processing commands (cancelled)\n", deviceID), SLACK_MSG_UNSOLICITED)
    }

}
