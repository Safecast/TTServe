// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Processing of requests enqueued by many protocol front-ends
package main

import (
	"fmt"
	"time"
    "hash/crc32"
    "github.com/golang/protobuf/proto"
    "github.com/safecast/ttproto/golang"
)

// Common app request
type IncomingAppReq struct {
    Payload []byte
    GwLongitude   *float32
    GwLatitude    *float32
    GwAltitude    *float32
    GwSnr         *float32
    GwLocation    *string
    GwReceivedAt  *string
    SvTransport   string
    SvUploadedAt  string
    TTNDevID      string
    SeqNo         int
}

var MAX_REQQ_PENDING int = 100
var AppReqQ chan IncomingAppReq

// Make the queue
func AppReqInit() {
    AppReqQ = make(chan IncomingAppReq, MAX_REQQ_PENDING)
}

// Push a new entry on the request queue
func AppReqPush(req IncomingAppReq) {
    AppReqQ <- req
}

// Common handler for messages incoming either from TTN or HTTP
func AppReqHandler() {

    // Dequeue and process the messages as they're enqueued
    for AppReq := range AppReqQ {

        // Unmarshal the message
        msg := &ttproto.Telecast{}
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
        case ttproto.Telecast_BGEIGIE_NANO:
            fallthrough
        case ttproto.Telecast_SOLARCAST:
            go SendSafecastMessage(AppReq, *msg, checksum)

            // Handle messages from non-safecast devices
        default:
            go SendTelecastMessage(*msg, AppReq.TTNDevID)
        }
    }
}

// Process a payload buffer
func AppReqPushPayload(req IncomingAppReq, buf []byte, from string) (DeviceId uint32) {
    var ReplyToDeviceId uint32 = 0
    var AppReq IncomingAppReq = req

    buf_format := buf[0]
    buf_length := len(buf)

    switch (buf_format) {

    case BUFF_FORMAT_SINGLE_PB: {

        fmt.Printf("\n%s Received %d-byte payload from %s %s\n", time.Now().Format(logDateFormat), buf_length, from, AppReq.SvTransport)

        // Construct an app request
        AppReq.Payload = buf

        // Extract the device ID from the message, which we will need later
        _, ReplyToDeviceId = getReplyDeviceIdFromPayload(AppReq.Payload)

        // Enqueue the app request
        AppReq.SvUploadedAt = nowInUTC()
		AppReqPush(AppReq)
    }

    case BUFF_FORMAT_PB_ARRAY: {

        fmt.Printf("\n%s Received %d-byte BUFFERED payload from %s %s\n", time.Now().Format(logDateFormat), buf_length, from, AppReq.SvTransport)

        if !validBulkPayload(buf, buf_length) {
            return 0
        }

        // Loop over the various things in the buffer
        UploadedAt := nowInUTC()
        count := int(buf[1])
        lengthArrayOffset := 2
        payloadOffset := lengthArrayOffset + count

        for i:=0; i<count; i++ {

            // Extract the length
            length := int(buf[lengthArrayOffset+i])

            // Construct the app request
            AppReq.Payload = buf[payloadOffset:payloadOffset+length]

            // Extract the device ID from the message, which we will need later
            _, ReplyToDeviceId = getReplyDeviceIdFromPayload(AppReq.Payload)

			// If a reply is expected, pass a sequence number of 0 so we process it as quickly as possible.
			// Otherwise, insert a sequence number to attempt to impose a sequencing delay in SendSafecastMessage,
            // so that things are sequenced properly in the log.  This is not guaranteed of course, but it is helpful
            // for log readability.
			if (ReplyToDeviceId == 0) {
	            AppReq.SeqNo = i
			} else {
				AppReq.SeqNo = 0
			}

            fmt.Printf("\n%s Received %d-byte (%d/%d) payload from %s %s\n", time.Now().Format(logDateFormat), len(AppReq.Payload),
                i+1, count, from, AppReq.SvTransport)

            // Enqueue AppReq
            AppReq.SvUploadedAt = UploadedAt
			AppReqPush(AppReq)

            // Bump the payload offset
            payloadOffset += length;

        }
    }

    default: {
        fmt.Printf("\n%s Received INVALID %d-byte buffered payload from DEVICE\n", time.Now().Format(logDateFormat), buf_length)
    }
    }

    return ReplyToDeviceId

}

// Validate a bulk payload
func validBulkPayload(buf []byte, length int) (bool) {

    // Debug
    if (false) {
        fmt.Printf("%v\n", buf)
    }

    // Enough room for the count field in header?
    header_length := 2
    if length < header_length {
        fmt.Printf("*** Invalid header ***\n", time.Now().Format(logDateFormat))
        return false
    }

    // Enough room for the length array?
    count := int(buf[1])
    header_length += count
    if length < header_length {
        fmt.Printf("*** Invalid header ***\n", time.Now().Format(logDateFormat))
        return false
    }

    // Enough room for payloads?
    total_length := header_length
    lengthArrayOffset := 2
    for i:=0; i<count; i++ {
        total_length += int(buf[lengthArrayOffset+i])
    }
    if length < total_length {
        fmt.Printf("*** Invalid payload ***\n", time.Now().Format(logDateFormat))
        return false
    }

    // Safe
    return true
}
