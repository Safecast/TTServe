// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Slack channel handling, both inbound and outbound
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Slack webhook
func inboundWebSlackHandler(rw http.ResponseWriter, req *http.Request) {
	stats.Count.HTTP++
	stats.Count.HTTPSlack++

	// Unpack the request
	body, err := io.ReadAll(req.Body)
	if err != nil {
		fmt.Printf("Slack webhook: error reading body: %s\n", err)
		return
	}
	urlParams, err := url.ParseQuery(string(body))
	if err != nil {
		fmt.Printf("Slack webhook: error parsing body: %s\n", err)
		return
	}

	// Extract useful information
	t, present := urlParams["token"]
	if !present {
		fmt.Printf("Slack token not present\n")
		return
	}
	token := t[0]

	u, present := urlParams["user_name"]
	if !present {
		fmt.Printf("Slack user_name not present\n")
		return
	}
	user := u[0]

	m, present := urlParams["text"]
	if !present {
		fmt.Printf("Slack message not present\n")
		return
	}
	message := m[0]

	// If the message is surrounded in backticks to eliminate formatting, remove them
	message = strings.Replace(message, "`", "", -1)

	// Process the command arguments
	args := strings.Split(message, " ")
	argsLC := strings.Split(strings.ToLower(message), " ")

	messageAfterFirstWord := ""
	if len(args) > 1 {
		messageAfterFirstWord = strings.Join(args[1:], " ")
	}

	// If this is a recursive echoing of our own post, bail.
	if user == "slackbot" {
		return
	}

	// Remember when this command was received, and make sure
	// that all other instances also know that precise time
	SlackCommandTime = time.Now()
	ControlFileTime(TTServerSlackCommandControlFile, user)

	// Figure out who is sending this to us
	SlackCommandSource = SlackOpsNone
	str := strings.Split(ServiceConfig.SlackInboundTokens, ",")
	for source, tok := range str {
		if tok == token {
			SlackCommandSource = source
		}
	}
	if SlackCommandSource == SlackOpsNone {
		fmt.Printf("*** Slack command from uknown token: %s\n", token)
		return
	}

	// Process queries
	switch argsLC[0] {

	case "help":
		help := ""
		help += "Show bulk device status:\n"
		help += "     online\n"
		help += "     offline\n"
		help += "Show gateway and server status:\n"
		help += "     gateway\n"
		help += "     server\n"
		go sendToSafecastOps(help, SlackMsgReply)

	case "online":
		go sendSafecastDeviceSummaryToSlack(user, "", false)

	case "offline":
		go sendSafecastDeviceSummaryToSlack(user, "", true)

	case "gateway":
		fallthrough
	case "gateways":
		sendSafecastGatewaySummaryToSlack("")

	case "server":
		fallthrough
	case "servers":
		fallthrough
	case "ttserve":
		sendSafecastServerSummaryToSlack("")

	case "reboot-all":
	case "restart-all":
		sendToSafecastOps("Restarting all service instances.", SlackMsgReply)
		time.Sleep(2 * time.Second)
		ServerLog("*** RESTARTING because of Slack 'restart-all' command\n")
		ControlFileTime(TTServerRestartAllControlFile, user)
		sendToSafecastOps(fmt.Sprintf("** %s restarting **", TTServeInstanceID), SlackMsgUnsolicitedOps)
		time.Sleep(3 * time.Second)
		os.Exit(0)

	case "reboot":
	case "restart":
		sendToSafecastOps("Restarting non-monitor service instances.", SlackMsgReply)
		ControlFileTime(TTServerRestartAllControlFile, user)

	case "hello":
		if len(args) == 1 {
			sendToSafecastOps(fmt.Sprintf("Hello there. Nice day, isn't it, %s?", user), SlackMsgReply)
		} else {
			sendToSafecastOps(fmt.Sprintf("Back at you: %s", messageAfterFirstWord), SlackMsgReply)
		}

	default:
		// Default is to do nothing
	}

}

// Send a text string to the Safecast #ops channel.  Note that this MUST BE FAST
// because there are assumptions that this will return quickly because it's called
// within an HTTP request handler that must return so as to flush the response buffer
// back to the callers.
func sendToSafecastOps(msg string, destination int) {
	if destination == SlackMsgUnsolicitedAll {
		str := strings.Split(ServiceConfig.SlackOutboundUrls, ",")
		for _, url := range str {
			go sendToOpsViaSlack(msg, url)
		}
	} else if destination == SlackMsgUnsolicitedOps {
		str := strings.Split(ServiceConfig.SlackOutboundUrls, ",")
		go sendToOpsViaSlack(msg, str[SlackOpsSafecast])
	} else {
		if SlackCommandSource != SlackOpsNone {
			str := strings.Split(ServiceConfig.SlackOutboundUrls, ",")
			go sendToOpsViaSlack(msg, str[SlackCommandSource])
		}
	}
}

// Send a text string to the TTN  #ops channel
func sendToTTNOps(msg string) {
	// Do nothing for now
}

// Send a string as a slack post to the specified channel
func sendToOpsViaSlack(msg string, SlackOpsPostURL string) {

	type SlackData struct {
		Message string `json:"text"`
	}

	m := SlackData{}
	m.Message = msg

	mJSON, _ := json.Marshal(m)
	req, _ := http.NewRequest("POST", SlackOpsPostURL, bytes.NewBuffer(mJSON))
	req.Header.Set("User-Agent", "TTSERVE")
	req.Header.Set("Content-Type", "application/json")

	httpclient := &http.Client{}
	resp, err := httpclient.Do(req)
	if err != nil {
		fmt.Printf("*** Error uploading %s to Slack  %s\n\n", msg, err)
	} else {
		resp.Body.Close()
	}

	// Wait for it to complete, because we seem to lose it on os.Exit()
	time.Sleep(5 * time.Second)

}
