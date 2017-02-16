// Incoming request processing
package main

import (
	"os"
	"fmt"
	"time"
    "hash/crc32"
    "github.com/golang/protobuf/proto"
    "github.com/rayozzie/teletype-proto/golang"
)

// Common app request
type IncomingAppReq struct {
    Payload []byte
    Longitude   float32
    Latitude    float32
    Altitude    float32
    Snr         float32
    Location    string
    ServerTime  string
    Transport   string
    UploadedAt  string
    TTNDevID    string
    SeqNo       int
}
var MAX_REQQ_PENDING int = 100
var AppReqQ chan IncomingAppReq
var AppReqQMaxLength = 0

// Make the queue
func AppReqInit() {
    AppReqQ = make(chan IncomingAppReq, MAX_REQQ_PENDING)
}

// Common handler for messages incoming either from TTN or HTTP
func AppReqHandler() {

    // Dequeue and process the messages as they're enqueued
    for AppReq := range AppReqQ {

        // Unmarshal the message
        msg := &teletype.Telecast{}
        err := proto.Unmarshal(AppReq.Payload, msg)
        if err != nil {
            fmt.Printf("*** PB unmarshaling error: ", err)
            fmt.Printf("*** ");
            for i:=0; i<len(AppReq.Payload); i++ {
                fmt.Printf("%02x", AppReq.Payload[i]);
            }
            fmt.Printf("\n");
            continue
        }

        // Display the actual unmarshaled value received in the payload
        fmt.Printf("%v\n", msg);

        // Display info about the received message
        deviceID := TelecastDeviceID(msg)
        fmt.Printf("%s sent by %d\n", time.Now().Format(logDateFormat), deviceID)
        if (msg.RelayDevice1 != nil) {
            fmt.Printf("%s RELAYED thru hop #1 %d\n", time.Now().Format(logDateFormat), msg.GetRelayDevice1())
        }
        if (msg.RelayDevice2 != nil) {
            fmt.Printf("%s RELAYED thru hop #2 %d\n", time.Now().Format(logDateFormat), msg.GetRelayDevice2())
        }
        if (msg.RelayDevice3 != nil) {
            fmt.Printf("%s RELAYED thru hop #3 %d\n", time.Now().Format(logDateFormat), msg.GetRelayDevice3())
        }
        if (msg.RelayDevice4 != nil) {
            fmt.Printf("%s RELAYED thru hop #4 %d\n", time.Now().Format(logDateFormat), msg.GetRelayDevice4())
        }
        if (msg.RelayDevice5 != nil) {
            fmt.Printf("%s RELAYED thru hop #5 %d\n", time.Now().Format(logDateFormat), msg.GetRelayDevice5())
        }

        // Compute the checksum on a payload normalized by removing all the relay information
        msg.RelayDevice1 = nil
        msg.RelayDevice2 = nil
        msg.RelayDevice3 = nil
        msg.RelayDevice4 = nil
        msg.RelayDevice5 = nil
        normalizedPayload, err := proto.Marshal(msg)
        if err != nil {
            fmt.Printf("*** PB marshaling error: ", err)
            continue
        }
        checksum := crc32.ChecksumIEEE(normalizedPayload)

        // Do various things based upon the message type
        switch msg.GetDeviceType() {

            // Is it something we recognize as being from safecast?
        case teletype.Telecast_BGEIGIE_NANO:
            fallthrough
        case teletype.Telecast_SOLARCAST:
            go SendSafecastMessage(AppReq.SeqNo, *msg, checksum, AppReq.UploadedAt, AppReq.Transport)

            // Handle messages from non-safecast devices
        default:
            go SendTelecastMessage(*msg, AppReq.TTNDevID)
        }
    }
}

// Monitor the queue length
func AppReqPush(req IncomingAppReq) {

	// Enqueue the item
    AppReqQ <- req

	// Check the length of the queue
    elements := len(AppReqQ)
    if (elements > AppReqQMaxLength) {
        AppReqQMaxLength = elements
        if (AppReqQMaxLength > 1) {
            fmt.Printf("\n%s Requests pending reached new maximum of %d\n", time.Now().Format(logDateFormat), AppReqQMaxLength)
        }
    }

    // We have observed once that the HTTP stack got messed up to the point where the queue just grew forever
    // because nothing was getting serviced.  In this case, abort and restart the process.
    if (AppReqQMaxLength >= MAX_REQQ_PENDING) {
        fmt.Printf("\n***\n***\n*** RESTARTING defensively because of request queue overflow\n***\n***\n\n")
        os.Exit(0)
    }

}
