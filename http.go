// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Common support for all HTTP topic handlers
package main

import (
	"io"
    "net/http"
	"time"
    "fmt"
)

// Kick off inbound messages coming from all sources, then serve HTTP
func HttpInboundHandler() {

    // Spin up functions only available on the monitor role, of which there is only one
    if ThisServerIsMonitor {

        http.HandleFunc(TTServerTopicGithub, inboundWebGithubHandler)
        fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServerFTPAddress, TTServerHTTPPort, TTServerTopicGithub)

        http.HandleFunc(TTServerTopicSlack, inboundWebSlackHandler)
        fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServerFTPAddress, TTServerHTTPPort, TTServerTopicSlack)

    }

    // Spin up TTN
    if !TTNMQQTMode {

        http.HandleFunc(TTServerTopicTTN, inboundWebTTNHandler)
        fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServerHTTPAddress, TTServerHTTPPort, TTServerTopicTTN)

    }

    // Spin up handler to handle misc web ping requests
    http.HandleFunc(TTServerTopicRoot1, inboundWebRootHandler)
    http.HandleFunc(TTServerTopicRoot2, inboundWebRootHandler)

    // Spin up log handlers
    http.HandleFunc(TTServerTopicDeviceLog, inboundWebLogHandler)
    http.HandleFunc(TTServerTopicDeviceStatus, inboundWebDeviceStatusHandler)
    http.HandleFunc(TTServerTopicServerLog, inboundWebServerLogHandler)
    http.HandleFunc(TTServerTopicServerStatus, inboundWebServerStatusHandler)
    http.HandleFunc(TTServerTopicGatewayUpdate, inboundWebGatewayUpdateHandler)
    http.HandleFunc(TTServerTopicGatewayStatus, inboundWebGatewayStatusHandler)

    // Spin up functions available on all roles
    http.HandleFunc(TTServerTopicSend, inboundWebSendHandler)
    fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServerHTTPAddress, TTServerHTTPPort, TTServerTopicSend)

    http.HandleFunc(TTServerTopicRedirect1, inboundWebRedirectHandler)
    fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServerHTTPAddress, TTServerHTTPPort, TTServerTopicRedirect1)

    http.HandleFunc(TTServerTopicRedirect2, inboundWebRedirectHandler)
    fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServerHTTPAddress, TTServerHTTPPort, TTServerTopicRedirect2)

    go func() {
        http.ListenAndServe(TTServerHTTPPortAlternate, nil)
    }()

    http.ListenAndServe(TTServerHTTPPort, nil)

}

// Handle inbound HTTP requests for root
func inboundWebRootHandler(rw http.ResponseWriter, req *http.Request) {
    io.WriteString(rw, fmt.Sprintf("Hello. (%s)\n", ThisServerAddressIPv4))
}

// Process a payload buffer
func processBuffer(req IncomingAppReq, from string, transport string, buf []byte) (DeviceId uint32) {
    var ReplyToDeviceId uint32 = 0
    var AppReq IncomingAppReq = req

    AppReq.Transport = transport

    buf_format := buf[0]
    buf_length := len(buf)

    switch (buf_format) {

    case BUFF_FORMAT_SINGLE_PB: {

        fmt.Printf("\n%s Received %d-byte payload from %s %s\n", time.Now().Format(logDateFormat), buf_length, from, AppReq.Transport)

        // Construct an app request, with ServerTime in case the payload lacked CapturedAt
        AppReq.Payload = buf
        AppReq.ServerTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")

        // Extract the device ID from the message, which we will need later
        _, ReplyToDeviceId = getReplyDeviceIdFromPayload(AppReq.Payload)

        // Enqueue the app request
        AppReq.UploadedAt = nowInUTC()
		AppReqPush(AppReq)
    }

    case BUFF_FORMAT_PB_ARRAY: {

        fmt.Printf("\n%s Received %d-byte BUFFERED payload from %s %s\n", time.Now().Format(logDateFormat), buf_length, from, AppReq.Transport)

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

            // Add ServerTime in case the payload lacked CapturedAt
            AppReq.ServerTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")

            fmt.Printf("\n%s Received %d-byte (%d/%d) payload from %s %s\n", time.Now().Format(logDateFormat), len(AppReq.Payload),
                i+1, count, from, AppReq.Transport)

            // Enqueue AppReq
            AppReq.UploadedAt = UploadedAt
			AppReqPush(AppReq)

            // Bump the payload offset
            payloadOffset += length;

        }
    }

    default: {
        fmt.Printf("\n%s Received INVALID %d-byte HTTP buffered payload from DEVICE\n", time.Now().Format(logDateFormat), buf_length)
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
