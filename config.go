// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Global configuration Parameters
package main

import (
    "os"
    "time"
    "fmt"
    "io/ioutil"
    "encoding/json"
)

// As of 2017-02 we're now operating in "HTTP Integration" TTN mode, largely
// so that we can serve incoming requests through our load balancer rather than
// having a single server that pulls MQQT requests.
const TTNMQQTMode = false

// TTN service info
const ttnAppId string = "ttserve"
const ttnProcessId string = "ttserve"
const ttnServer string = "tcp://eu.thethings.network:1883"
const ttnTopic string = "+/devices/+/up"
const ttnDownlinkURL = "https://integrations.thethingsnetwork.org/ttn-eu/api/v2/down/%s/%s?key=%s"

// Google Sheets ID of published (File/Publish to Web) doc, as CSV
const sheetsSolarcastTracker = "https://docs.google.com/spreadsheets/d/1lvB_0XFFSwON4PQFoC8NdDv6INJTCw2f_KBZuMTZhZA/export?format=csv"

// Safecast service info
const SafecastV1UploadDomainDev = "dev.safecast.org"
const SafecastV1UploadDomain = "api.safecast.org"
const SafecastV1UploadPattern = "http://%s/measurements.json?%s"
var SafecastUploadURLs = [...]string {
    "http://ingest.safecast.org/v1/measurements",
}

// Slack service info
const SLACK_OPS_NONE =     -1
const SLACK_OPS_SAFECAST =  0
var SlackCommandSource = SLACK_OPS_NONE

var SlackCommandTime time.Time

const SLACK_MSG_UNSOLICITED =       0
const SLACK_MSG_UNSOLICITED_OPS =   1
const SLACK_MSG_REPLY =             2

// Our configuration, read out of a file for security reasons
var ServiceConfig TTServeConfig

// Paths for the file system shared among all TTSERVE instances
const TTConfigPath = "/config/config.json"
const TTDeviceLogPath = "/device-log"
const TTDeviceStampPath = "/device-stamp"
const TTDeviceStatusPath = "/device-status"
const TTInfluxQueryPath = "/influx-query"
const TTServerLogPath = "/server-log"
const TTServerStatusPath = "/server-status"
const TTGatewayStatusPath = "/gateway-status"
const TTServerCommandPath = "/command"
const TTServerControlPath = "/control"
const TTServerBuildPath = "/build"
const TTServerFTPCertPath = "/cert/ftp"
const TTServerSlackCommandControlFile = "slack_command.txt"
const TTServerRestartAllControlFile = "restart_all.txt"
var TTServeInstanceID = ""

// TTSERVE's address and ports
const TTServerHTTPAddress = "tt.safecast.org"
const TTServerUDPAddress = "tt-udp.safecast.org"
var   TTServerUDPAddressIPv4 = ""                   // Looked up dynamically
const TTServerFTPAddress = "tt-ftp.safecast.org"
var   TTServerFTPAddressIPv4 = ""                   // Looked up dynamically
const TTServerHTTPPort string = ":80"
const TTServerHTTPPortAlternate string = ":8080"
const TTServerUDPPort string = ":8081"
const TTServerTCPPort string = ":8082"
const TTServerFTPPort int = 8083                    // plus 8084 plus the entire passive range
const TTServerTopicSend string = "/send"
const TTServerTopicRoot1 string = "/index.html"
const TTServerTopicRoot2 string = "/index.htm"
const TTServerTopicDeviceLog string = "/device-log/"
const TTServerTopicDeviceCheck string = "/check/"
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
var   ThisServerAddressIPv4 = ""                    // Looked up dynamically

// Dynamically computed state about this particular server
var   ThisServerServesUDP = false
var   ThisServerServesFTP = false
var   ThisServerServesMQQT = false
var   ThisServerIsMonitor = false
var   ThisServerBootTime time.Time
var   AllServersSlackRestartRequestTime time.Time
var   AllServersGithubRestartRequestTime time.Time

// Payload buffer format
const BUFF_FORMAT_PB_ARRAY byte  =  0

// Log-related
const logDateFormat string = "2006-01-02 15:04:05"

// Global Server Stats
type TTServeCounts struct {
    Restarts        uint32          `json:"restarts,omitempty"`
    UDP             uint32          `json:"received_device_udp,omitempty"`
    TCP             uint32          `json:"received_device_tcp,omitempty"`
    HTTP            uint32          `json:"received_all_http,omitempty"`
    HTTPSlack       uint32          `json:"received_slack_http,omitempty"`
    HTTPGithub      uint32          `json:"received_github_http,omitempty"`
    HTTPGUpdate     uint32          `json:"received_gateway_update_http,omitempty"`
    HTTPDevice      uint32          `json:"received_device_msg_http,omitempty"`
    HTTPGateway     uint32          `json:"received_gateway_msg_http,omitempty"`
    HTTPRelay       uint32          `json:"received_udp_to_http,omitempty"`
    HTTPRedirect    uint32          `json:"received_redirect_http,omitempty"`
    HTTPTTN         uint32          `json:"received_ttn_http,omitempty"`
    MQQTTTN         uint32          `json:"received_ttn_mqqt,omitempty"`
}
type TTServeStatus struct {
    Started             time.Time       `json:"started,omitempty"`
    AddressIPv4         string          `json:"publicIp,omitempty"`
    Services            string          `json:"services,omitempty"`
    AWSInstance         AWSInstanceIdentity `json:"aws,omitempty"`
    Count               TTServeCounts   `json:"counts,omitempty"`
}
var stats TTServeStatus

// Get the current value
func ServiceReadConfig() TTServeConfig {

    // Read the file and unmarshall if no error
    contents, err := ioutil.ReadFile(SafecastDirectory() + TTConfigPath)
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
