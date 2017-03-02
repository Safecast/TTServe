// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/send" HTTP topic
package main

import (
    "net/http"
    "fmt"
    "time"
    "io"
    "io/ioutil"
    "encoding/hex"
    "encoding/json"
)

// Handle inbound HTTP requests from the gateway or directly from the device
func inboundWebSendHandler(rw http.ResponseWriter, req *http.Request) {
    var AppReq IncomingAppReq
    var ReplyToDeviceId uint32 = 0
    stats.Count.HTTP++

    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        fmt.Printf("Error reading HTTP request body: \n%v\n", req)
        return
    }

    switch (req.UserAgent()) {

        // UDP messages that were relayed to the TTSERVE HTTP load balancer, JSON-formatted
    case "TTSERVE": {
        var ttg TTGateReq

        err = json.Unmarshal(body, &ttg)
        if err != nil {
            fmt.Printf("*** Received badly formatted HTTP request from %s: \n%v\n", req.UserAgent(), body)
            return
        }

        // Process it.  Note there is no possibility of a reply.
        processBuffer(AppReq, "device on cellular", ttg.Transport, ttg.Payload)
        stats.Count.HTTPRelay++;

    }

        // Messages that come from TTGATE are JSON-formatted
    case "TTGATE": {
        var ttg TTGateReq

        err = json.Unmarshal(body, &ttg)
        if err != nil {
            return
        }

        // Copy into the app req structure
        AppReq.Latitude = ttg.Latitude
        AppReq.Longitude = ttg.Longitude
        AppReq.Altitude = float32(ttg.Altitude)
        AppReq.Snr = ttg.Snr
        AppReq.Location = ttg.Location

		// Figure out the transport based upon whether or not a gateway ID was included
		Transport := "lora-http:" + getRequestorIPv4(req)
		if ttg.GatewayId != "" {
			Transport = "lora:"+ttg.GatewayId
		}

        // Process it
        ReplyToDeviceId = processBuffer(AppReq, "Lora gateway", Transport, ttg.Payload)
        stats.Count.HTTPGateway++;

    }

        // Messages directly from devices are hexified
    case "TTNODE": {

        // After the single solarproto unit is upgraded, we can remove this.
        buf, err := hex.DecodeString(string(body))
        if err != nil {
            fmt.Printf("Hex decoding error: %v\n%v\n", err, string(body))
            return
        }

        // Process it
        ReplyToDeviceId = processBuffer(AppReq, "device on cellular", "device-http:"+getRequestorIPv4(req), buf)
        stats.Count.HTTPDevice++;

    }

    default: {

        // A web crawler, etc.
        return

    }

    }

    // Outbound message processing
    if (ReplyToDeviceId != 0) {

        // Wait for up to five seconds for a reply to appear.  It's no big deal if we miss it,
        // though, because it'll just be picked up on the next call

        for i:=0; i<5; i++ {

            // See if there's an outbound message waiting for this device.
            isAvailable, payload := TelecastOutboundPayload(ReplyToDeviceId)
            if (isAvailable) {

                // Responses for now are always hex-encoded for easy device processing
                hexPayload := hex.EncodeToString(payload)
                io.WriteString(rw, hexPayload)
                sendToSafecastOps(fmt.Sprintf("Device %d picked up its pending command\n", ReplyToDeviceId), SLACK_MSG_UNSOLICITED)
                break
            }

            time.Sleep(1 * time.Second)
        }

    }

}
