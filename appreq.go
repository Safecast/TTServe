// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Processing of requests enqueued by many protocol front-ends
package main

import (
	"fmt"

	ttproto "github.com/Safecast/ttproto/golang"
	"github.com/golang/protobuf/proto"
)

// IncomingAppReq is the common request format that we process as a goroutine
type IncomingAppReq struct {
	Payload      []byte
	GwLongitude  *float64
	GwLatitude   *float64
	GwAltitude   *float64
	GwSnr        *float64
	GwLocation   *string
	GwReceivedAt *string
	SvTransport  string
	SvUploadedAt string
	TTNDevID     string
	SeqNo        int
}

// AppReqProcess handles an app request synchronously, WITHOUT an inner goroutine.
// This is important for sequencing of certain incoming requests
func AppReqProcess(AppReq IncomingAppReq) {
	// Unmarshal the message
	msg := &ttproto.Telecast{}
	err := proto.Unmarshal(AppReq.Payload, msg)
	if err != nil {
		fmt.Printf("*** PB unmarshaling error: ", err)
		fmt.Printf("*** ")
		for i := 0; i < len(AppReq.Payload); i++ {
			fmt.Printf("%02x", AppReq.Payload[i])
		}
		fmt.Printf("\n")
		return
	}

	// Display the actual unmarshaled value received in the payload
	fmt.Printf("%v\n", msg)

	// Display info about the received message
	if msg.RelayDevice1 != nil {
		fmt.Printf("%s RELAYED thru hop #1 %d\n", LogTime(), msg.GetRelayDevice1())
	}
	if msg.RelayDevice2 != nil {
		fmt.Printf("%s RELAYED thru hop #2 %d\n", LogTime(), msg.GetRelayDevice2())
	}
	if msg.RelayDevice3 != nil {
		fmt.Printf("%s RELAYED thru hop #3 %d\n", LogTime(), msg.GetRelayDevice3())
	}
	if msg.RelayDevice4 != nil {
		fmt.Printf("%s RELAYED thru hop #4 %d\n", LogTime(), msg.GetRelayDevice4())
	}
	if msg.RelayDevice5 != nil {
		fmt.Printf("%s RELAYED thru hop #5 %d\n", LogTime(), msg.GetRelayDevice5())
	}

	// Do various things based upon the message type
	if msg.DeviceType == nil {
		SendSafecastMessage(AppReq, *msg)
	} else {
		switch msg.GetDeviceType() {

		// Is it something we recognize as being from safecast?
		case ttproto.Telecast_BGEIGIE_NANO:
			fallthrough
		case ttproto.Telecast_UNKNOWN_DEVICE_TYPE:
			fallthrough
		case ttproto.Telecast_SOLARCAST:
			SendSafecastMessage(AppReq, *msg)

		}
	}
}

// AppReqPushPayload handles a payload buffer by either placing it onto a queue, or in the case of
// a PB array by processing it directly.  As such, if there is any ambiguity about whether or not the
// payload is an array, it is best to invoke this as a goroutine.
func AppReqPushPayload(req IncomingAppReq, buf []byte, from string) {
	var AppReq = req

	bufFormat := buf[0]
	bufLength := len(buf)

	switch bufFormat {

	case BuffFormatPBArray:
		{

			if !validBulkPayload(buf, bufLength) {
				fmt.Printf("\n%s Received INVALID %d-byte payload from %s %s\n", LogTime(), bufLength, from, AppReq.SvTransport)
				return
			}

			// Loop over the various things in the buffer
			UploadedAt := NowInUTC()
			count := int(buf[1])
			lengthArrayOffset := 2
			payloadOffset := lengthArrayOffset + count

			for i := 0; i < count; i++ {

				// Extract the length
				length := int(buf[lengthArrayOffset+i])

				// Construct the app request
				AppReq.Payload = buf[payloadOffset : payloadOffset+length]

				if count == 1 {
					fmt.Printf("\n%s Received %d-byte payload from %s %s\n", LogTime(), len(AppReq.Payload), from, AppReq.SvTransport)
				} else {
					fmt.Printf("\n%s Received %d-byte (%d/%d) payload from %s %s\n", LogTime(), len(AppReq.Payload), i+1, count, from, AppReq.SvTransport)
				}

				// Process the AppReq synchronously, because they must be done in-order
				AppReq.SvUploadedAt = UploadedAt
				AppReqProcess(AppReq)

				// Bump the payload offset
				payloadOffset += length

			}
		}

	default:
		{
			isASCII := true
			for i := 0; i < len(buf); i++ {
				if buf[i] > 0x7f || (buf[i] < ' ' && buf[i] != '\r' && buf[i] != '\n' && buf[i] != '\t') {
					isASCII = false
					break
				}
			}
			if isASCII {
				fmt.Printf("\n%s Received unrecognized %d-byte payload from %s:\n%s\n", LogTime(), bufLength, AppReq.SvTransport, buf)
			} else {
				fmt.Printf("\n%s Received unrecognized %d-byte payload from %s:\n%v\n", LogTime(), bufLength, AppReq.SvTransport, buf)
			}
		}
	}

}

// Validate a bulk payload
func validBulkPayload(buf []byte, length int) bool {

	// Debug
	if false {
		fmt.Printf("%v\n", buf)
	}

	// Enough room for the count field in header?
	headerLength := 2
	if length < headerLength {
		fmt.Printf("%s *** Invalid header ***\n", LogTime())
		return false
	}

	// A count of at least 1?
	count := int(buf[1])
	if count == 0 {
		fmt.Printf("%s *** Invalid count ***\n", LogTime())
		return false
	}

	// Enough room for the length array?
	headerLength += count
	if length < headerLength {
		fmt.Printf("%s *** Invalid header ***\n", LogTime())
		return false
	}

	// Enough room for payloads?
	totalLength := headerLength
	lengthArrayOffset := 2
	for i := 0; i < count; i++ {
		totalLength += int(buf[lengthArrayOffset+i])
	}
	if length < totalLength {
		fmt.Printf("%s *** Invalid payload ***\n", LogTime())
		return false
	}

	// Safe
	return true
}
