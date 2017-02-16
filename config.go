// Configuration Parameters
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

// Paths for the file system shared among all TTSERVE instances
const TTServerLogPath = "/log"
const TTServerStampPath = "/stamp"
const TTServerCommandPath = "/command"
const TTServerControlPath = "/control"
const TTServerValuePath = "/value"
const TTServerBuildPath = "/build"
const TTServerFTPCertPath = "/cert/ftp"
const TTServerRestartGithubControlFile = "restart_github.txt"
const TTServerRestartAllControlFile = "restart_all.txt"
const TTServerHealthControlFile = "health.txt"

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
const TTServerTopicLog string = "/log/"
const TTServerTopicValue string = "/device/"
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
var   AllServersSlackHealthRequestTime time.Time

// Buffered I/O header formats coordinated with TTNODE.  Note that although we are now starting
// with version number 0, we special-case version number 8 because of the old style "single protocl buffer"
// message format that always begins with 0x08. (see ttnode/send.c)
const BUFF_FORMAT_PB_ARRAY byte  =  0
const BUFF_FORMAT_SINGLE_PB byte =  8

// Log-related
const logDateFormat string = "2006-01-02 15:04:05"

// Global Server Stats
var CountUDP = 0
var CountHTTPDevice = 0
var CountHTTPGateway = 0
var CountHTTPRelay = 0
var CountHTTPRedirect = 0
var CountTTN = 0
