// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Global configuration Parameters
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// TTNMQTTMode defines the MQTT vs mq-over-HTTP operating mode.
// As of 2017-02 we're now operating in "HTTP Integration" TTN mode, largely
// so that we can serve incoming requests through our load balancer rather than
// having a single server that pulls MQTT requests.
const TTNMQTTMode = false

// TTN service info
const ttnAppID string = "ttserve"
const ttnServer string = "tcp://eu.thethings.network:1883"
const ttnTopic string = "+/devices/+/up"

// Google Sheets ID of published (File/Publish to Web) doc, as CSV
const sheetsSolarcastTracker = "https://docs.google.com/spreadsheets/d/1lvB_0XFFSwON4PQFoC8NdDv6INJTCw2f_KBZuMTZhZA/export?format=csv"

// Safecast service info

// SafecastV1UploadDomainDev is developer API server
const SafecastV1UploadDomainDev = "dev.safecast.cc"

// SafecastV1UploadDomain is production API server
const SafecastV1UploadDomain = "api.safecast.cc"

// SafecastV1UploadPattern is the pattern of the URL for both
const SafecastV1UploadPattern = "https://%s/measurements.json?%s"

// SafecastV1SolarcastUploadURL is the url where to upload solarcast data
const SafecastV1SolarcastUploadURL = "https://api.safecast.cc/measurements.json?api_key=z3sHhgousVDDrCVXhzMT"

// SafecastUploadURLs are the places we should upload V2 measurements
var SafecastUploadURLs = [...]string{
	"http://ingest.safecast.cc/v1/measurements",
}

// Slack service info

// SlackOpsNone means "no ops channel specified"
const SlackOpsNone = -1

// SlackOpsSafecast means the #ops channel in the safecast slack
const SlackOpsSafecast = 0

// SlackCommandTime is the time a slack command was issued
var SlackCommandTime time.Time

// SlackCommandSource means the ID of the channel that originated a command
var SlackCommandSource = SlackOpsNone

// SlackMsgUnsolicitedAll means to send a message to all safecast channels
const SlackMsgUnsolicitedAll = 0

// SlackMsgUnsolicitedOps means to reply just to the safecast ops channel
const SlackMsgUnsolicitedOps = 1

// SlackMsgReply means to reply to the SlackCommandSource
const SlackMsgReply = 2

// ServiceConfig is our configuration, read out of a file for security reasons
var ServiceConfig TTServeConfig

// Paths for the file system shared among all TTSERVE instances

// TTConfigPath (here for golint)
const TTConfigPath = "/config/config.json"

// TTDeviceLogPath (here for golint)
const TTDeviceLogPath = "/device-log"

// TTFilePath (here for golint)
const TTFilePath = "/files"

// TTDeviceStampPath (here for golint)
const TTDeviceStampPath = "/device-stamp"

// TTDeviceStatusPath (here for golint)
const TTDeviceStatusPath = "/device-status"

// TTQueryPath (here for golint)
const TTQueryPath = "/query"

// TTServerLogPath (here for golint)
const TTServerLogPath = "/server-log"

// TTServerStatusPath (here for golint)
const TTServerStatusPath = "/server-status"

// TTGatewayStatusPath (here for golint)
const TTGatewayStatusPath = "/gateway-status"

// TTServerControlPath (here for golint)
const TTServerControlPath = "/control"

// TTServerBuildPath (here for golint)
const TTServerBuildPath = "/build"

// TTServerSlackCommandControlFile (here for golint)
const TTServerSlackCommandControlFile = "slack_command.txt"

// TTServerRestartAllControlFile (here for golint)
const TTServerRestartAllControlFile = "restart_all.txt"

// TTServeInstanceID is the AWS instance ID for the current instance
var TTServeInstanceID = ""

// TTSERVE's address and ports

// TTServerHTTPAddress (here for golint)
const TTServerHTTPAddress = "tt.safecast.cc"

// TTServerUDPAddress (here for golint)
const TTServerUDPAddress = "tt-udp.safecast.cc"

// TTServerUDPAddressIPv4 (here for golint)
var TTServerUDPAddressIPv4 = "" // Looked up dynamically

// TTServerHTTPPort (here for golint)
const TTServerHTTPPort string = ":80"

// TTServerHTTPPortAlternate (here for golint)
const TTServerHTTPPortAlternate string = ":8080"

// TTServerUDPPort (here for golint)
const TTServerUDPPort string = ":8081"

// TTServerTCPPort (here for golint)
const TTServerTCPPort string = ":8082"

// TTServerTopicID (here for golint) [DEPRECATED]
const TTServerTopicID string = "/id/"

// TTServerTopicDashboard (here for golint)
const TTServerTopicDashboard string = "/dashboard/"

// TTServerTopicProfile (here for golint)
const TTServerTopicProfile string = "/profile/"

// TTServerTopicMap (here for golint)
const TTServerTopicMap string = "/map/"

// TTServerTopicDevices (here for golint)
const TTServerTopicDevices string = "/devices"

// TTServerTopicSend (here for golint)
const TTServerTopicSend string = "/send"

// TTServerTopicRoot1 (here for golint)
const TTServerTopicRoot1 string = "/index.html"

// TTServerTopicRoot2 (here for golint)
const TTServerTopicRoot2 string = "/index.htm"

// TTServerTopicDeviceLog (here for golint)
const TTServerTopicDeviceLog string = "/device-log/"

// TTServerTopicFile (here for golint)
const TTServerTopicFile string = "/file/"

// TTServerTopicDeviceCheck (here for golint)
const TTServerTopicDeviceCheck string = "/check/"

// TTServerTopicDeviceStatus (here for golint)
const TTServerTopicDeviceStatus string = "/device/"

// TTServerTopicServerLog (here for golint)
const TTServerTopicServerLog string = "/server-log/"

// TTServerTopicServerStatus (here for golint)
const TTServerTopicServerStatus string = "/server/"

// TTServerTopicGatewayUpdate (here for golint)
const TTServerTopicGatewayUpdate string = "/gateway"

// TTServerTopicGatewayStatus (here for golint)
const TTServerTopicGatewayStatus string = "/gateway/"

// TTServerTopicGithub (here for golint)
const TTServerTopicGithub string = "/github"

// TTServerTopicSlack (here for golint)
const TTServerTopicSlack string = "/slack"

// TTServerTopicTTN (here for golint)
const TTServerTopicTTN string = "/ttn"

// TTServerTopicRedirect1 (here for golint)
const TTServerTopicRedirect1 string = "/scripts/"

// TTServerTopicRedirect2 (here for golint)
const TTServerTopicRedirect2 string = "/"

// TTServerTopicNote (here for golint)
const TTServerTopicNote string = "/note"

// TTServerTopicNoteTest (here for golint)
const TTServerTopicNoteTest string = "/notetest"

// ThisServerAddressIPv4 is looked up dynamically
var ThisServerAddressIPv4 = ""

// Dynamically computed state about this particular server

// ThisServerServesUDP (here for golint)
var ThisServerServesUDP = false

// ThisServerServesMQTT (here for golint)
var ThisServerServesMQTT = false

// ThisServerIsMonitor (here for golint)
var ThisServerIsMonitor = false

// ThisServerBootTime (here for golint)
var ThisServerBootTime time.Time

// AllServersSlackRestartRequestTime (here for golint)
var AllServersSlackRestartRequestTime time.Time

// AllServersGithubRestartRequestTime (here for golint)
var AllServersGithubRestartRequestTime time.Time

// BuffFormatPBArray is the payload buffer format
const BuffFormatPBArray byte = 0

// Log-related
const logDateFormat string = "2006-01-02 15:04:05"

// TTServeCounts is our global statistics structure
type TTServeCounts struct {
	Restarts     uint32 `json:"restarts,omitempty"`
	UDP          uint32 `json:"received_device_udp,omitempty"`
	TCP          uint32 `json:"received_device_tcp,omitempty"`
	HTTP         uint32 `json:"received_all_http,omitempty"`
	HTTPSlack    uint32 `json:"received_slack_http,omitempty"`
	HTTPGithub   uint32 `json:"received_github_http,omitempty"`
	HTTPGUpdate  uint32 `json:"received_gateway_update_http,omitempty"`
	HTTPDevice   uint32 `json:"received_device_msg_http,omitempty"`
	HTTPGateway  uint32 `json:"received_gateway_msg_http,omitempty"`
	HTTPRelay    uint32 `json:"received_udp_to_http,omitempty"`
	HTTPRedirect uint32 `json:"received_redirect_http,omitempty"`
	HTTPTTN      uint32 `json:"received_ttn_http,omitempty"`
	MQTTTTN      uint32 `json:"received_ttn_mqtt,omitempty"`
}

// TTServeStatus is our global status
type TTServeStatus struct {
	Started     time.Time           `json:"started,omitempty"`
	AddressIPv4 string              `json:"publicIp,omitempty"`
	Services    string              `json:"services,omitempty"`
	AWSInstance AWSInstanceIdentity `json:"aws,omitempty"`
	Count       TTServeCounts       `json:"counts,omitempty"`
}

var stats TTServeStatus

// ServiceReadConfig gets the current value of the service config
func ServiceReadConfig() TTServeConfig {

	// Read the file and unmarshall if no error
	contents, err := os.ReadFile(SafecastDirectory() + TTConfigPath)
	if err != nil {
		fmt.Printf("Can't start service: %s %v\n", TTConfigPath, err)
		os.Exit(0)
	}

	value := TTServeConfig{}
	err = json.Unmarshal(contents, &value)
	if err != nil {
		fmt.Printf("Can't parse JSON: %s %v\n", TTConfigPath, err)
		os.Exit(0)
	}

	return value

}
