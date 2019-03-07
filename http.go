// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Common support for all HTTP topic handlers
package main

import (
	"io"
    "net/http"
    "fmt"
)

// HTTPInboundHandler kicks off inbound messages coming from all sources, then serve HTTP
func HTTPInboundHandler() {

    // Spin up functions only available on the monitor role, of which there is only one
    if ThisServerIsMonitor {
        http.HandleFunc(TTServerTopicGithub, inboundWebGithubHandler)
        http.HandleFunc(TTServerTopicSlack, inboundWebSlackHandler)
    }

    // Spin up TTN
    if !TTNMQTTMode {
        http.HandleFunc(TTServerTopicTTN, inboundWebTTNHandler)
    }

    // Spin up misc handlers
    http.HandleFunc(TTServerTopicRoot1, inboundWebRootHandler)
    http.HandleFunc(TTServerTopicRoot2, inboundWebRootHandler)
    http.HandleFunc(TTServerTopicDeviceLog, inboundWebDeviceLogHandler)
    http.HandleFunc(TTServerTopicQueryResults, inboundWebQueryResultsHandler)
    http.HandleFunc(TTServerTopicDeviceCheck, inboundWebDeviceCheckHandler)
    http.HandleFunc(TTServerTopicDeviceStatus, inboundWebDeviceStatusHandler)
    http.HandleFunc(TTServerTopicServerLog, inboundWebServerLogHandler)
    http.HandleFunc(TTServerTopicServerStatus, inboundWebServerStatusHandler)
    http.HandleFunc(TTServerTopicGatewayStatus, inboundWebGatewayStatusHandler)
    http.HandleFunc(TTServerTopicGatewayUpdate, inboundWebGatewayUpdateHandler)
    http.HandleFunc(TTServerTopicSend, inboundWebSendHandler)
    http.HandleFunc(TTServerTopicNote, inboundWebNoteHandler)
    http.HandleFunc(TTServerTopicRedirect1, inboundWebRedirectHandler)
    http.HandleFunc(TTServerTopicRedirect2, inboundWebRedirectHandler)

	// Listen on the alternate HTTP port
    go func() {
	    fmt.Printf("Now handling inbound HTTP on %s\n", TTServerHTTPPortAlternate)
        http.ListenAndServe(TTServerHTTPPortAlternate, nil)
    }()

	// Listen on the primary HTTP port
    fmt.Printf("Now handling inbound HTTP on %s\n", TTServerHTTPPort)
    http.ListenAndServe(TTServerHTTPPort, nil)

}

// Handle inbound HTTP requests for root
func inboundWebRootHandler(rw http.ResponseWriter, req *http.Request) {
    stats.Count.HTTP++
    io.WriteString(rw, fmt.Sprintf("Hello. (%s)\n", ThisServerAddressIPv4))
}
