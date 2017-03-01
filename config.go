// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Global configuration Parameters
package main

import (
	"time"
)

// As of 2017-02 we're now operating in "HTTP Integration" TTN mode, largely
// so that we can serve incoming requests through our load balancer rather than
// having a single server that pulls MQQT requests.
const TTNMQQTMode = false	

// TTN service info
const ttnAppId string = "ttserve"
const ttnProcessId string = "ttserve"
const ttnAppAccessKey string = "ttn-account-v2.OFAp-VRdr1vrHqXf-iijSFaNdJSgIy5oVdmX2O2160g"
const ttnServer string = "tcp://eu.thethings.network:1883"
const ttnTopic string = "+/devices/+/up"
const ttnDownlinkURL = "https://integrations.thethingsnetwork.org/ttn-eu/api/v2/down/%s/%s?key=%s"

// Safecast service info
const SafecastV1UploadURL = "http://gw01.safecast.org"
var SafecastUploadURLs = [...]string {
    "http://ingest.safecast.org/v1/measurements",
}

// Slack service info
const SLACK_OPS_NONE =	   -1
const SLACK_OPS_SAFECAST =	0
const SLACK_OPS_ROZZIE =	1
const SLACK_OPS_MUSTI =		2
var SlackCommandSource = SLACK_OPS_NONE

var SlackCommandTime time.Time

const SLACK_MSG_UNSOLICITED =		0
const SLACK_MSG_UNSOLICITED_OPS =	1
const SLACK_MSG_REPLY =				2

var SlackInboundTokens = [...]string {
	// Safecast
	"JzemotPucrCixAx2JPRZpgn9",
	// Rozzie
	"sfC2GfAnleQ3BFgsq2dGO7Yw",
	// Musti
	"XnOxJn2lD7SvECqOs56aSUUb",
}

var SlackOutboundURLs = [...]string {
	// Safecast #ops channel
	"https://hooks.slack.com/services/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",
	// Ray Ozzie's private team/channel, for testing
	"https://hooks.slack.com/services/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",
	// Musti's team
	"https://hooks.slack.com/services/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",
}

// Paths for the file system shared among all TTSERVE instances
const TTDeviceLogPath = "/device-log"
const TTDeviceStampPath = "/device-stamp"
const TTDeviceStatusPath = "/device-status"
const TTServerLogPath = "/server-log"
const TTServerStatusPath = "/server-status"
const TTGatewayStatusPath = "/gateway-status"
const TTServerCommandPath = "/command"
const TTServerControlPath = "/control"
const TTServerBuildPath = "/build"
const TTServerFTPCertPath = "/cert/ftp"
const TTServerSlackCommandControlFile = "slack_command.txt"
const TTServerRestartGithubControlFile = "restart_github.txt"
const TTServerRestartAllControlFile = "restart_all.txt"
var TTServeInstanceID = ""

// TTSERVE's address and ports
const TTServerHTTPAddress = "tt.safecast.org"
const TTServerUDPAddress = "tt-udp.safecast.org"
var   TTServerUDPAddressIPv4 = ""					// Looked up dynamically
const TTServerFTPAddress = "tt-ftp.safecast.org"
var   TTServerFTPAddressIPv4 = ""					// Looked up dynamically
const TTServerHTTPPort string = ":80"
const TTServerHTTPPortAlternate string = ":8080"
const TTServerUDPPort string = ":8081"
const TTServerFTPPort int = 8083					// plus 8084 plus the entire passive range
const TTServerTopicSend string = "/send"
const TTServerTopicRoot1 string = "/index.html"
const TTServerTopicRoot2 string = "/index.htm"
const TTServerTopicDeviceLog string = "/device-log/"
const TTServerTopicDeviceStatus string = "/device/"
const TTServerTopicServerLog string = "/server-log/"
const TTServerTopicServerStatus string = "/server/"
const TTServerTopicGatewayUpdate string = "/gateway"
const TTServerTopicGatewayStatus string = "/gateway/"
const TTServerTopicGithub string = "/github"
const TTServerTopicSlack string = "/slack"
const TTServerTopicTTN string = "/ttn"
const TTServerTopicRedirect1 string = "/scripts/"
const TTServerTopicRedirect2 string = "/"
var	  ThisServerAddressIPv4 = ""					// Looked up dynamically

// Dynamically computed state about this particular server
var   ThisServerServesUDP = false
var   ThisServerServesFTP = false
var   ThisServerServesMQQT = false
var   ThisServerIsMonitor = false
var   ThisServerBootTime time.Time
var   AllServersSlackRestartRequestTime time.Time
var   AllServersGithubRestartRequestTime time.Time

// Buffered I/O header formats coordinated with TTNODE.  Note that although we are now starting
// with version number 0, we special-case version number 8 because of the old style "single protocl buffer"
// message format that always begins with 0x08. (see ttnode/send.c)
const BUFF_FORMAT_PB_ARRAY byte  =  0
const BUFF_FORMAT_SINGLE_PB byte =  8

// Log-related
const logDateFormat string = "2006-01-02 15:04:05"

// Global Server Stats
type TTServeStatus struct {
	Started				time.Time		`json:"started,omitempty"`
	AddressIPv4			string			`json:"publicIp,omitempty"`
	AWSInstance			AWSInstanceIdentity	`json:"aws,omitempty"`
	CountUDP			uint32			`json:"received_device_udp,omitempty"`
	CountHTTPDevice		uint32			`json:"received_device_http,omitempty"`
	CountHTTPGateway	uint32			`json:"received_gateway_http,omitempty"`
	CountHTTPRelay		uint32			`json:"received_ttserve_http,omitempty"`
	CountHTTPRedirect	uint32			`json:"received_redirect_http,omitempty"`
	CountHTTPTTN		uint32			`json:"received_ttn_http,omitempty"`
	CountMQQTTTN		uint32			`json:"received_ttn_mqqt,omitempty"`
}
var stats TTServeStatus
