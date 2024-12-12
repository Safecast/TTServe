// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/send" HTTP topic
package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Unpack common AppReq fields from an incoming TTGateReq
func newAppReqFromGateway(ttg *TTGateReq, Transport string) IncomingAppReq {
	var AppReq IncomingAppReq

	if ttg.Latitude != 0 {
		AppReq.GwLatitude = &ttg.Latitude
		AppReq.GwLongitude = &ttg.Longitude
		alt := float64(ttg.Altitude)
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

	return AppReq

}

// Handle inbound HTTP requests from the gateway or directly from the device
func inboundWebSendHandler(rw http.ResponseWriter, req *http.Request) {
	stats.Count.HTTP++

	body, err := io.ReadAll(req.Body)
	if err != nil {
		fmt.Printf("Error reading HTTP request body: \n%v\n", req)
		return
	}

	switch req.UserAgent() {

	// UDP messages that were relayed to the TTSERVE HTTP load balancer, JSON-formatted
	case "TTSERVE":
		{
			var ttg TTGateReq

			err = json.Unmarshal(body, &ttg)
			if err != nil {
				fmt.Printf("*** Received badly formatted HTTP request from %s: \n%v\n", req.UserAgent(), body)
				return
			}

			// Use the TTGateReq to initialize a new AppReq
			AppReq := newAppReqFromGateway(&ttg, ttg.Transport)

			// Process it.  Note there is no possibility of a reply.
			go AppReqPushPayload(AppReq, ttg.Payload, "device directly")
			stats.Count.HTTPRelay++

		}

		// Messages that come from TTGATE are JSON-formatted
	case "TTGATE":
		{
			var ttg TTGateReq

			err = json.Unmarshal(body, &ttg)
			if err != nil {
				return
			}

			// Figure out the transport based upon whether or not a gateway ID was included
			requestor, _, abusive := getRequestorIPv4(req)
			if abusive {
				return
			}
			Transport := "lora-http:" + requestor
			if ttg.GatewayID != "" {
				Transport = "lora:" + ttg.GatewayID
			}

			// Use the TTGateReq to initialize a new AppReq
			AppReq := newAppReqFromGateway(&ttg, Transport)

			// Process it
			go AppReqPushPayload(AppReq, ttg.Payload, "Lora gateway")
			stats.Count.HTTPGateway++

		}

		// Messages directly from devices are hexified
	case "TTNODE":
		{

			// After the single solarproto unit is upgraded, we can remove this.
			buf, err := hex.DecodeString(string(body))
			if err != nil {
				fmt.Printf("Hex decoding error: %v\n%v\n", err, string(body))
				return
			}

			// Initialize a new AppReq
			AppReq := IncomingAppReq{}
			requestor, _, abusive := getRequestorIPv4(req)
			if abusive {
				return
			}
			AppReq.SvTransport = "device-http:" + requestor

			// Push it
			go AppReqPushPayload(AppReq, buf, "device directly")
			stats.Count.HTTPDevice++

		}

	default:
		{

			// A web crawler, etc.
			return

		}

	}

}
