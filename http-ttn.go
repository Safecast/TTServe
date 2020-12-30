// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/ttn" HTTP topic
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// Handle inbound HTTP requests from TTN
func inboundWebTTNHandler(rw http.ResponseWriter, req *http.Request) {
	var AppReq IncomingAppReq
	var ttn UplinkMessage

	stats.Count.HTTP++

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		fmt.Printf("Error reading HTTP request body: \n%v\n", req)
		return
	}

	// Unmarshal the payload and extract the base64 data
	err = json.Unmarshal(body, &ttn)
	if err != nil {
		fmt.Printf("\n*** Web TTN payload doesn't have TTN data *** %v\n%s\n\n", err, body)
		return
	}

	// Copy fields to the app request structure
	AppReq.TTNDevID = ttn.DevID
	tt := time.Time(ttn.Metadata.Time)
	ts := tt.UTC().Format("2006-01-02T15:04:05Z")
	AppReq.GwReceivedAt = &ts
	if ttn.Metadata.Longitude != 0 {
		AppReq.GwLongitude = &ttn.Metadata.Longitude
		AppReq.GwLatitude = &ttn.Metadata.Latitude
		alt := float64(ttn.Metadata.Altitude)
		AppReq.GwAltitude = &alt
	}
	if len(ttn.Metadata.Gateways) >= 1 {
		AppReq.GwSnr = &ttn.Metadata.Gateways[0].SNR
		AppReq.GwLocation = &ttn.Metadata.Gateways[0].GtwID
	}
	AppReq.SvTransport = "ttn-http:" + ttn.DevID

	// Push it to be processed
	go AppReqPushPayload(AppReq, ttn.PayloadRaw, "TTN")
	stats.Count.HTTPTTN++

}
