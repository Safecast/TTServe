// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/ttn" HTTP topic
package main

import (
    "net/http"
    "fmt"
	"time"
    "bytes"
    "io/ioutil"
    "encoding/json"
)

// Handle inbound HTTP requests from TTN
func inboundWebTTNHandler(rw http.ResponseWriter, req *http.Request) {
    var AppReq IncomingAppReq
    var ttn UplinkMessage
    var ReplyToDeviceID uint32

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

	// Get the reply device ID
    ReplyToDeviceID = getReplyDeviceIDFromPayload(ttn.PayloadRaw)
		
    // Push it to be processed
    go AppReqPushPayload(AppReq, ttn.PayloadRaw, "TTN")
    stats.Count.HTTPTTN++

    // Outbound message processing
    if ReplyToDeviceID != 0 {

        // See if there's an outbound message waiting for this device.
        isAvailable, payload := TelecastOutboundPayload(ReplyToDeviceID)
        if isAvailable {
            jmsg := &DownlinkMessage{}
            jmsg.DevID = ttn.DevID
            jmsg.FPort = 1
            jmsg.Confirmed = false
            jmsg.PayloadRaw = payload
            jdata, jerr := json.Marshal(jmsg)
            if jerr != nil {
                fmt.Printf("dl j marshaling error: ", jerr)
            } else {
				var err error
				var resp *http.Response
				
                url := fmt.Sprintf(ttnDownlinkURL, ttnAppID, ttnProcessID, ServiceConfig.TtnAppAccessKey)

				// Retry several times in case of failure
				for i:=0; i<3; i++ {

	                fmt.Printf("\nHTTP POST to %s\n%s\n\n", url, jdata)
	                req, err := http.NewRequest("POST", url, bytes.NewBuffer(jdata))
	                req.Header.Set("User-Agent", "TTSERVE")
	                req.Header.Set("Content-Type", "text/plain")
	                httpclient := &http.Client{
	                    Timeout: time.Second * 15,
	                }
	                resp, err = httpclient.Do(req)
					if err == nil {
						break
					}
				}

                if err != nil {
                    fmt.Printf("\n*** HTTPS POST error: %v\n\n", err)
                    sendToSafecastOps(fmt.Sprintf("Error transmitting command to device %d: %s\n", ReplyToDeviceID, ErrorString(err)), SlackMsgUnsolicitedOps)
                } else {
                    resp.Body.Close()
                    sendToSafecastOps(fmt.Sprintf("Device %d picked up its pending command\n", ReplyToDeviceID), SlackMsgUnsolicitedOps)
                }

            }
        }

    }

}
