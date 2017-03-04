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

// Unpack common AppReq fields from an incoming TTGateReq
func newAppReq(ttg *TTGateReq, Transport string) IncomingAppReq {
    var AppReq IncomingAppReq

    if ttg.Latitude != 0 {
        AppReq.GwLatitude = &ttg.Latitude
        AppReq.GwLongitude = &ttg.Longitude
        alt := float32(ttg.Altitude)
        AppReq.GwAltitude = &alt
    }

    if ttg.Snr != 0 {
        AppReq.GwSnr = &ttg.Snr
    }

    if ttg.ReceivedAt != "" {
        AppReq.GwReceivedAt = &ttg.ReceivedAt
    }

    if ttg.Location != "" {
        AppReq.GwLocation = &ttg.Location
    }

    AppReq.SvTransport = Transport

	// debug
	if true {
		fmt.Printf("TTGateReq: \n%v\nAppReq: \n%v\n", ttg, AppReq)
	}

	return AppReq
	
}

// Handle inbound HTTP requests from the gateway or directly from the device
func inboundWebSendHandler(rw http.ResponseWriter, req *http.Request) {
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

        // Use the TTGateReq to initialize a new AppReq
        AppReq := newAppReq(&ttg, ttg.Transport)

        // Process it.  Note there is no possibility of a reply.
        processBuffer(AppReq, "device on cellular", ttg.Payload)
        stats.Count.HTTPRelay++;

    }

        // Messages that come from TTGATE are JSON-formatted
    case "TTGATE": {
        var ttg TTGateReq

        err = json.Unmarshal(body, &ttg)
        if err != nil {
            return
        }

	    // Figure out the transport based upon whether or not a gateway ID was included
	    requestor, _ := getRequestorIPv4(req)
		Transport := "lora-http:" + requestor
	    if ttg.GatewayId != "" {
	        Transport = "lora:"+ttg.GatewayId
	    }

        // Use the TTGateReq to initialize a new AppReq
        AppReq := newAppReq(&ttg, Transport)

        // Process it
        ReplyToDeviceId = processBuffer(AppReq, "Lora gateway", ttg.Payload)
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


        // Initialize a new AppReq
		AppReq := IncomingAppReq{}
        requestor, _ := getRequestorIPv4(req)
	    AppReq.SvTransport = "device-http:" + requestor
			
        ReplyToDeviceId = processBuffer(AppReq, "device on cellular", buf)
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
