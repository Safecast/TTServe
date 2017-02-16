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
    var ReplyToDeviceID uint32 = 0

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
    AppReq.Longitude = ttn.Metadata.Longitude
    AppReq.Latitude = ttn.Metadata.Latitude
    AppReq.Altitude = float32(ttn.Metadata.Altitude)
    if (len(ttn.Metadata.Gateways) >= 1) {
        AppReq.Snr = ttn.Metadata.Gateways[0].SNR
        AppReq.Location = ttn.Metadata.Gateways[0].GtwID
    }

    ReplyToDeviceID = processBuffer(AppReq, "TTN", "ttn-http:"+ttn.DevID, ttn.PayloadRaw)
    CountTTN++

    // Outbound message processing
    if (ReplyToDeviceID != 0) {

        // Delay just in case there's a chance that request processing may generate a reply
        // to this request.  It's no big deal if we miss it, though, because it will just be
        // picked up on the next call.
        time.Sleep(1 * time.Second)

        // See if there's an outbound message waiting for this device.
        isAvailable, payload := TelecastOutboundPayload(ReplyToDeviceID)
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
                    sendToSafecastOps(fmt.Sprintf("Error transmitting command to device %d: %v\n", ReplyToDeviceID, err))
                } else {
                    resp.Body.Close()
                    sendToSafecastOps(fmt.Sprintf("Device %d picked up its pending command\n", ReplyToDeviceID))
                }

            }
        }

    }

}