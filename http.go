// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Common support for all HTTP topic handlers
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	http.HandleFunc(TTServerTopicID, inboundWebIDHandler)
	http.HandleFunc(TTServerTopicDevices, inboundWebDevicesHandler)
	http.HandleFunc(TTServerTopicDeviceLog, inboundWebDeviceLogHandler)
	http.HandleFunc(TTServerTopicDeviceCheck, inboundWebDeviceCheckHandler)
	http.HandleFunc(TTServerTopicDeviceStatus, inboundWebDeviceStatusHandler)
	http.HandleFunc(TTServerTopicServerLog, inboundWebServerLogHandler)
	http.HandleFunc(TTServerTopicServerStatus, inboundWebServerStatusHandler)
	http.HandleFunc(TTServerTopicGatewayStatus, inboundWebGatewayStatusHandler)
	http.HandleFunc(TTServerTopicGatewayUpdate, inboundWebGatewayUpdateHandler)
	http.HandleFunc(TTServerTopicSend, inboundWebSendHandler)
	http.HandleFunc(TTServerTopicNote, inboundWebNoteHandler)
	http.HandleFunc(TTServerTopicNoteTest, inboundWebNoteHandlerTest)
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

// HTTPArgs parses the request URI and returns interesting things
func HTTPArgs(req *http.Request, topic string) (target string, args map[string]string, err error) {
	args = map[string]string{}

	// Trim the request URI
	target = req.RequestURI[len(topic):]

	// If nothing left, there were no args
	if len(target) == 0 {
		return
	}

	// Make sure that the prefix is "/", else the pattern matcher is matching something we don't want
	if strings.HasPrefix(target, "/") {
		target = strings.TrimPrefix(target, "/")
	}

	// See if there is a query, and if so process it
	str := strings.SplitN(target, "?", 2)
	if len(str) == 1 {
		return
	}
	target = str[0]
	remainder := str[1]

	// See if there is an anchor on the target, and special-case it
	str2 := strings.Split(target, "#")
	if len(str2) > 1 {
		target = str2[0]
		args["anchor"] = str2[1]
	}

	// Now that we know we have args, parse them
	values, err2 := url.ParseQuery(remainder)
	if err2 != nil {
		err = err2
		fmt.Printf("can't parse query: %s\n%s\n", err, str[1])
		return
	}

	// Generate the return arg in the format we expect
	for k, v := range values {
		if len(v) == 1 {
			str := v[0]

			// Safely unquote the value.  This fails if there are NO quotes, so
			// only replace the str value if no error occurs
			s, err2 := strconv.Unquote(str)
			if err2 == nil {
				str = s
			}

			args[k], err = url.PathUnescape(str)
			if err != nil {
				fmt.Printf("can't unescape: %s\n%s\n", err, str)
				return
			}
		}
	}

	// Done
	return

}
