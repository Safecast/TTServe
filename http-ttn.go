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
    var ReplyToDeviceId uint32 = 0

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
		alt := float32(ttn.Metadata.Altitude)
	    AppReq.GwAltitude = &alt
	}
    if (len(ttn.Metadata.Gateways) >= 1) {
        AppReq.GwSnr = &ttn.Metadata.Gateways[0].SNR
        AppReq.GwLocation = &ttn.Metadata.Gateways[0].GtwID
    }

    ReplyToDeviceId = processBuffer(AppReq, "TTN", "ttn-http:"+ttn.DevID, ttn.PayloadRaw)
    stats.Count.HTTPTTN++

    // Outbound message processing
    if (ReplyToDeviceId != 0) {

        // Delay just in case there's a chance that request processing may generate a reply
        // to this request.  It's no big deal if we miss it, though, because it will just be
        // picked up on the next call.
        time.Sleep(1 * time.Second)

        // See if there's an outbound message waiting for this device.
        isAvailable, payload := TelecastOutboundPayload(ReplyToDeviceId)
        if (isAvailable) {
            jmsg := &DownlinkMessage{}
            jmsg.DevID = ttn.DevID
            jmsg.FPort = 1
            jmsg.Confirmed = false
            jmsg.PayloadRaw = payload
            jdata, jerr := json.Marshal(jmsg)
            if jerr != nil {
                fmt.Printf("dl j marshaling error: ", jerr)
            } else {

                url := fmt.Sprintf(ttnDownlinkURL, ttnAppId, ttnProcessId, ttnAppAccessKey)

                fmt.Printf("\nHTTP POST to %s\n%s\n\n", url, jdata)

                req, err := http.NewRequest("POST", url, bytes.NewBuffer(jdata))
                req.Header.Set("User-Agent", "TTSERVE")
                req.Header.Set("Content-Type", "text/plain")
                httpclient := &http.Client{
                    Timeout: time.Second * 15,
                }
                resp, err := httpclient.Do(req)
                if err != nil {
                    fmt.Printf("\n*** HTTPS POST error: %v\n\n", err);
                    sendToSafecastOps(fmt.Sprintf("Error transmitting command to device %d: %v\n", ReplyToDeviceId, err), SLACK_MSG_UNSOLICITED)
                } else {
                    resp.Body.Close()
                    sendToSafecastOps(fmt.Sprintf("Device %d picked up its pending command\n", ReplyToDeviceId), SLACK_MSG_UNSOLICITED)
                }

            }
        }

    }

}
