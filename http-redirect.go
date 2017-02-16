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

    // Read the body as a byte array
    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        fmt.Printf("Error reading HTTP request body: \n%v\n", req)
        return
    }

    // Decode the request with custom marshaling
    sdV1, err = SafecastV1Decode(bytes.NewReader(body))
    if err != nil {
        if (req.RequestURI != "/" && req.RequestURI != "/favicon.ico") {
            if err == io.EOF {
                fmt.Printf("\n%s HTTP request '%s' from %s ignored\n", time.Now().Format(logDateFormat), req.RequestURI, ipv4(req.RemoteAddr));
            } else {
                fmt.Printf("\n%s HTTP request '%s' from %s ignored: %v\n", time.Now().Format(logDateFormat), req.RequestURI, ipv4(req.RemoteAddr), err);
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

    var net Net
    transportStr := deviceType+":"+ipv4(req.RemoteAddr)
    net.Transport = &transportStr
    sd.Net = &net

    fmt.Printf("\n%s Received payload for %d from %s\n", time.Now().Format(logDateFormat), sd.DeviceID, transportStr)
    fmt.Printf("%s\n", body)

    // Fill in the minimums so as to prevent faults
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
    CountHTTPRedirect++

    // It is an error if there is a pending outbound payload for this device, so remove it and report it
    isAvailable, _ := TelecastOutboundPayload(deviceID)
    if (isAvailable) {
        sendToSafecastOps(fmt.Sprintf("%d is not capable of processing commands (cancelled)\n", deviceID))
    }

    // Send a reply to Pointcast saying that the request was processed acceptably.
    // If we fail to do this, Pointcast goes into an infinite reboot loop with comms errors
    // due to GetMeasurementReply() returning 0.
    io.WriteString(rw, "{\"id\":00000001}\r\n")


}