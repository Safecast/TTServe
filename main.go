// Teletype Message Publishing Service
package main

import (
    "bytes"
    "os"
    "os/signal"
    "syscall"
    "io"
    "io/ioutil"
    "net/http"
    "net"
    "fmt"
    "time"
    "strings"
    "encoding/json"
    "encoding/hex"
    "github.com/golang/protobuf/proto"
    "github.com/rayozzie/teletype-proto/golang"
    "hash/crc32"
    MQTT "github.com/eclipse/paho.mqtt.golang"
    ftp "github.com/fclairamb/ftpserver/server"
)

// Global debugging
const pollControlFilesQuickly bool = true

// TTN subscription info updated 2017-02-06 when upgrading to V2
const ttnMQQTMode = false
const ttnAppId string = "ttserve"
const ttnProcessId string = "ttserve"
const ttnAppAccessKey string = "ttn-account-v2.OFAp-VRdr1vrHqXf-iijSFaNdJSgIy5oVdmX2O2160g"
const ttnServer string = "tcp://eu.thethings.network:1883"
const ttnTopic string = "+/devices/+/up"
const ttnDownlinkURL = "https://integrations.thethingsnetwork.org/ttn-eu/api/v2/down/%s/%s?key=%s"

// Safecast-related
const SafecastV1UploadIP = "107.161.164.163"
const SafecastV1UploadURL = "http://" + SafecastV1UploadIP + "/scripts/indextest.php"
const SafecastV1QueryString = "api_key=z3sHhgousVDDrCVXhzMT"
var SafecastV2UploadURLs = [...]string {
    "http://ingest.safecast.org/v1/measurements",
}

// File system related paths relative to the server's HomeDir
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

// Buffered I/O header formats.  Note that although we are now starting with version number 0, we
// special case version number 8 because of the old style "single protocl buffer" message format that
// always begins with 0x08. (see ttnode/send.c)
const BUFF_FORMAT_PB_ARRAY byte  =  0
const BUFF_FORMAT_SINGLE_PB byte =  8

// This server-related
const TTServerHTTPAddress = "tt.safecast.org"
const TTServerUDPAddress = "tt-udp.safecast.org"
var   TTServerUDPAddressIPv4 = ""
var   iAmTTServerUDP = false
const TTServerFTPAddress = "tt-ftp.safecast.org"
var   TTServerFTPAddressIPv4 = ""
var   iAmTTServerFTP = false
const TTServerHTTPPort string = ":80"
const TTServerHTTPPortAlternate string = ":8080"
const TTServerUDPPort string = ":8081"
const TTServerFTPPort int = 8083    // plus 8084 plus the entire passive range
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
var   iAmTTServerMonitor = false

// Our server
var TTServer string
var TTServerIP string

// Auto-reboot
var TTServerBootTime time.Time
var TTServerRestartAllTime time.Time
var TTServerRestartGithubTime time.Time
var TTServerHealthTime time.Time

// Stats
var CountUDP = 0
var CountHTTPDevice = 0
var CountHTTPGateway = 0
var CountHTTPRelay = 0
var CountHTTPRedirect = 0
var CountTTN = 0

// Constants
const logDateFormat string = "2006-01-02 15:04:05"

// FTP
var (
    ftpServer *ftp.FtpServer
)

// Statics
var ttnEverConnected bool = false
var ttnFullyConnected bool = false
var ttnOutages uint16 = 0
var ttnMqttClient MQTT.Client
var ttnLastConnected string = "(never)"
var ttnLastDisconnectedTime time.Time
var ttnLastDisconnected string = "(never)"
var ttnUpQ chan MQTT.Message
var MAX_PENDING_REQUESTS int = 100

// Common app request
type IncomingReq struct {
    Payload []byte
    Longitude  float32
    Latitude   float32
    Altitude   float32
    Snr        float32
    Location   string
    ServerTime string
    Transport  string
    UploadedAt string
    TTNDevID  string
}
var reqQ chan IncomingReq
var reqQMaxLength = 0

// Main entry point for app
func main() {

    // Remember boot time
    TTServerBootTime = time.Now()

    // Get our external IP address
    rsp, err := http.Get("http://checkip.amazonaws.com")
    if err != nil {
        fmt.Printf("Can't get our own IP address: %v\n", err);
        os.Exit(0)
    }
    defer rsp.Body.Close()
    buf, err := ioutil.ReadAll(rsp.Body)
    if err != nil {
        fmt.Printf("Error fetching IP addr: %v\n", err);
        os.Exit(0)
    }
    TTServerIP = string(bytes.TrimSpace(buf))
    TTServer = "http://" + TTServerIP

    // Look up the two IP addresses that we KNOW have only a single A record,
    // and determine if WE are the server for those protocols
    addrs, err := net.LookupHost(TTServerUDPAddress)
    if err != nil {
        fmt.Printf("Can't resolve %s: %v\n", TTServerUDPAddress, err);
        os.Exit(0)
    }
    if len(addrs) < 1 {
        fmt.Printf("Can't resolve %s: %v\n", TTServerUDPAddress, err);
        os.Exit(0)
    }
    TTServerUDPAddressIPv4 = addrs[0]
    iAmTTServerUDP = TTServerUDPAddressIPv4 == TTServerIP

    addrs, err = net.LookupHost(TTServerFTPAddress)
    if err != nil {
        fmt.Printf("Can't resolve %s: %v\n", TTServerFTPAddress, err);
        os.Exit(0)
    }
    if len(addrs) < 1 {
        fmt.Printf("Can't resolve %s: %v\n", TTServerFTPAddress, err);
        os.Exit(0)
    }
    TTServerFTPAddressIPv4 = addrs[0]
    iAmTTServerFTP = TTServerFTPAddressIPv4 == TTServerIP
    iAmTTServerMonitor = iAmTTServerFTP

    // Get the date/time of the special files that we monitor
    TTServerRestartAllTime = ControlFileTime(TTServerRestartAllControlFile, "")
    TTServerRestartGithubTime = ControlFileTime(TTServerRestartGithubControlFile, "")
    TTServerHealthTime = ControlFileTime(TTServerHealthControlFile, "")

    // Set up our signal handler
    go signalHandler()

    // Set up our internal message queues
    ttnUpQ = make(chan MQTT.Message, 5)
    reqQ = make(chan IncomingReq, MAX_PENDING_REQUESTS)

    // Spawn the app request handler shared by both TTN and direct inbound server
    go commonRequestHandler()

    // Init our web request inbound server
    go webInboundHandler()

    // Init our UDP single-sample upload request inbound server
    if iAmTTServerUDP {
        go udpInboundHandler()
    }

    // Init our FTP server
    if iAmTTServerFTP {
        go ftpInboundHandler()
    }

    // Spawn the TTNhandlers
    if ttnMQQTMode && iAmTTServerUDP {
        go ttnMQQTInboundHandler()
        go ttnMQQTSubscriptionMonitor()
    }

    // Spawn timer tasks, and do the final one in-line
    go timer15m()
    timer1m()

}

// General periodic housekeeping
func timer1m() {
    for {
        time.Sleep(1 * 60 * time.Second)

        // Restart this instance if instructed to do so
        if (pollControlFilesQuickly) {
            ControlFileCheck()
        }

    }
}

// General periodic housekeeping
func timer15m() {
    for {

        // On the monitor role, track expired devices.
        // We do this before the first sleep so we have a list of device ASAP
        if iAmTTServerMonitor {
            sendExpiredSafecastDevicesToSlack()
        }

        // Sleep
        time.Sleep(15 * 60 * time.Second)

        // Report maximum inbound pending transactions
        if (reqQMaxLength > 1) {
            fmt.Printf("\n%s Request queue high water mark: %d concurrent requests\n", time.Now().Format(logDateFormat), reqQMaxLength)
            if (reqQMaxLength >= MAX_PENDING_REQUESTS) {
                fmt.Printf("\n***\n***\n*** RESTARTING defensively because of request queue overflow\n***\n***\n\n")
                os.Exit(0)
            }
        }

        // Post Safecast errors
        sendSafecastCommsErrorsToSlack(15)

        // Post long TTN outages
        if ttnMQQTMode && iAmTTServerUDP {
            ttnMQQTSubscriptionNotifier()
        }

        // Post stats
        fmt.Printf("\n%s Stats: UDP:%d HTTPDevice:%d HTTPGateway:%d HTTPRelay:%d HTTPRedirect:%d TTN:%d\n\n", time.Now().Format(logDateFormat),
            CountUDP, CountHTTPDevice, CountHTTPGateway, CountHTTPRelay, CountHTTPRedirect, CountTTN)

        // Restart this instance if instructed to do so
        ControlFileCheck()

    }

}

// Server health check
func healthCheck() string {
    s := ""
    var minutesAgo uint32 = uint32(int64(time.Now().Sub(TTServerBootTime) / time.Minute))
    var hoursAgo uint32 = minutesAgo / 60
    var daysAgo uint32 = hoursAgo / 24
    minutesAgo -= hoursAgo * 60
    hoursAgo -= daysAgo * 24
    if daysAgo != 0 {
        s = fmt.Sprintf("%s last restarted %dd %dh %dm ago", TTServerIP, daysAgo, hoursAgo, minutesAgo)
    } else if hoursAgo != 0 {
        s = fmt.Sprintf("%s last restarted %dh %dm ago", TTServerIP, hoursAgo, minutesAgo)
    } else {
        s = fmt.Sprintf("%s last restarted %dm ago", TTServerIP, minutesAgo)
    }
    return s
}

// Check to see if we should restart
func ControlFileCheck() {

    // Slack restart
    if (ControlFileTime(TTServerRestartAllControlFile, "") != TTServerRestartAllTime) {
        sendToSafecastOps(fmt.Sprintf("** %s restarting **", TTServerIP))
        fmt.Printf("\n***\n***\n*** RESTARTING because of Slack 'restart-all' command\n***\n***\n\n")
        os.Exit(0)
    }

    // Github restart
    if (ControlFileTime(TTServerRestartGithubControlFile, "") != TTServerRestartGithubTime) {
        fmt.Printf("\n***\n***\n*** RESTARTING because of Github push command\n***\n***\n\n")
        os.Exit(0)
    }

    // Heath
    if (ControlFileTime(TTServerHealthControlFile, "") != TTServerHealthTime) {
        sendToSafecastOps(healthCheck())
        TTServerHealthTime = ControlFileTime(TTServerHealthControlFile, "")
    }

}

// Kick off inbound messages coming from all sources, then serve HTTP
func ftpInboundHandler() {

    fmt.Printf("Now handling inbound FTP on: %s:%d\n", TTServer, TTServerFTPPort)

    ftpServer = ftp.NewFtpServer(NewTeletypeDriver())
    err := ftpServer.ListenAndServe()
    if err != nil {
        fmt.Printf("Error listening on FTP: %s\n", err)
    }

}

// Upload a Safecast data structure the load balancer for the web service
func doUploadToWebLoadBalancer(data []byte, datalen int, addr string) {

    if true {
        fmt.Printf("\n%s Received %d-byte UDP payload from %s, routing to LB\n", time.Now().Format(logDateFormat), datalen, addr)
    }

    url := "http://" + TTServerHTTPAddress + TTServerHTTPPort + TTServerTopicSend

    req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
    req.Header.Set("User-Agent", "TTSERVE")
    req.Header.Set("Content-Type", "text/plain")
    httpclient := &http.Client{
        Timeout: time.Second * 15,
    }
    resp, err := httpclient.Do(req)
    if err != nil {
        fmt.Printf("HTTP POST error: %v\n", err);
    } else {
        resp.Body.Close()
    }

}

// Kick off inbound messages coming from all sources, then serve HTTP
func webInboundHandler() {

    // Spin up functions only available on the monitor role, of which there is only one
    if iAmTTServerMonitor {

        http.HandleFunc(TTServerTopicGithub, inboundWebGithubHandler)
        fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServer, TTServerHTTPPort, TTServerTopicGithub)

        http.HandleFunc(TTServerTopicSlack, inboundWebSlackHandler)
        fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServer, TTServerHTTPPort, TTServerTopicSlack)

    }

    // Spin up TTN
    if !ttnMQQTMode {

        http.HandleFunc(TTServerTopicTTN, inboundWebTTNHandler)
        fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServer, TTServerHTTPPort, TTServerTopicTTN)

    }

    // Spin up handler to handle misc web ping requests
    http.HandleFunc(TTServerTopicRoot1, inboundWebRootHandler)
    http.HandleFunc(TTServerTopicRoot2, inboundWebRootHandler)

    // Spin up log handlers
    http.HandleFunc(TTServerTopicLog, inboundWebLogHandler)
    http.HandleFunc(TTServerTopicValue, inboundWebValueHandler)

    // Spin up functions available on all roles
    http.HandleFunc(TTServerTopicSend, inboundWebSendHandler)
    fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServer, TTServerHTTPPort, TTServerTopicSend)

    http.HandleFunc(TTServerTopicRedirect1, inboundWebRedirectHandler)
    fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServer, TTServerHTTPPort, TTServerTopicRedirect1)

    http.HandleFunc(TTServerTopicRedirect2, inboundWebRedirectHandler)
    fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServer, TTServerHTTPPort, TTServerTopicRedirect2)

    go func() {
        http.ListenAndServe(TTServerHTTPPortAlternate, nil)
    }()

    http.ListenAndServe(TTServerHTTPPort, nil)

}

// Kick off UDP single-upload request server
func udpInboundHandler() {

    fmt.Printf("Now handling inbound UDP on: %s%s\n", TTServer, TTServerUDPPort)

    ServerAddr, err := net.ResolveUDPAddr("udp", TTServerUDPPort)
    if err != nil {
        fmt.Printf("Error resolving UDP port: \n%v\n", err)
        return
    }

    ServerConn, err := net.ListenUDP("udp", ServerAddr)
    if err != nil {
        fmt.Printf("Error listening on UDP port: \n%v\n", err)
        return
    }
    defer ServerConn.Close()

    for {
        buf := make([]byte, 4096)

        n, addr, err := ServerConn.ReadFromUDP(buf)
        if (err != nil) {
            fmt.Printf("UDP read error: \n%v\n", err)
            time.Sleep(1 * 60 * time.Second)
        } else {

            ttg := &TTGateReq{}
            ttg.Payload = buf[0:n]
            ttg.Transport = "udp:" + ipv4(addr.String())
            data, err := json.Marshal(ttg)
            if err == nil {
                go doUploadToWebLoadBalancer(data, n, ipv4(addr.String()))
                CountUDP++;
            }

        }

    }

}

// Handle inbound HTTP requests from the gateway or directly from the device
func inboundWebSendHandler(rw http.ResponseWriter, req *http.Request) {
    var AppReq IncomingReq
    var ReplyToDeviceID uint32 = 0

    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        fmt.Printf("Error reading HTTP request body: \n%v\n", req)
        return
    }

    switch (req.UserAgent()) {

        // UDP messages that were relayed to the TTSERVE HTTP load balancer, JSON-formatted
    case "TTSERVE": {
        var ttg TTGateReq

        err = json.Unmarshal(body, &ttg)
        if err != nil {
            fmt.Printf("*** Received badly formatted HTTP request from %s: \n%v\n", req.UserAgent(), body)
            return
        }

        // Process it.  Note there is no possibility of a reply.
        processBuffer(AppReq, "device on cellular", ttg.Transport, ttg.Payload)
        CountHTTPRelay++;

    }

        // Messages that come from TTGATE are JSON-formatted
    case "TTGATE": {
        var ttg TTGateReq

        err = json.Unmarshal(body, &ttg)
        if err != nil {
            return
        }

        // Copy into the app req structure
        AppReq.Latitude = ttg.Latitude
        AppReq.Longitude = ttg.Longitude
        AppReq.Altitude = float32(ttg.Altitude)
        AppReq.Snr = ttg.Snr
        AppReq.Location = ttg.Location

        // Process it
        ReplyToDeviceID = processBuffer(AppReq, "Lora gateway", "lora-http:"+ipv4(req.RemoteAddr), ttg.Payload)
        CountHTTPGateway++;

    }

        // Messages directly from devices are hexified
    case "TTNODE": {

        // After the single solarproto unit is upgraded, we can remove this.
        buf, err := hex.DecodeString(string(body))
        if err != nil {
            fmt.Printf("Hex decoding error: %v\n%v\n", err, string(body))
            return
        }

        // Process it
        ReplyToDeviceID = processBuffer(AppReq, "device on cellular", "http:"+ipv4(req.RemoteAddr), buf)
        CountHTTPDevice++;

    }

    default: {

        // A web crawler, etc.
        return

    }

    }

    // Outbound message processing
    if (ReplyToDeviceID != 0) {

        // Delay just in case there's a chance that request processing may generate a reply
        // to this request.  It's no big deal if we miss it, though, because it will just be
        // picked up on the next call.
        time.Sleep(1 * time.Second)

        // See if there's an outbound message waiting for this device.
        isAvailable, payload := TelecastOutboundPayload(ReplyToDeviceID)
        if (isAvailable) {

			// Responses for now are always hex-encoded for easy device processing
            hexPayload := hex.EncodeToString(payload)
            io.WriteString(rw, hexPayload)
            sendToSafecastOps(fmt.Sprintf("Device %d picked up its pending command\n", ReplyToDeviceID))
        }

    }

}

// Handle inbound HTTP requests from TTN
func inboundWebTTNHandler(rw http.ResponseWriter, req *http.Request) {
    var AppReq IncomingReq
    var ttn UplinkMessage
    var ReplyToDeviceID uint32 = 0

    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        fmt.Printf("Error reading HTTP request body: \n%v\n", req)
        return
    }

    // Unmarshal the payload and extract the base64 data
    err = json.Unmarshal(body, &ttn)
    if err != nil {
        fmt.Printf("\n*** Web TTN payload doesn't have TTN data *** %v\n%s\n\n", err, body)
        return
    }

    // Copy fields to the app request structure
    AppReq.TTNDevID = ttn.DevID
    AppReq.Longitude = ttn.Metadata.Longitude
    AppReq.Latitude = ttn.Metadata.Latitude
    AppReq.Altitude = float32(ttn.Metadata.Altitude)
    if (len(ttn.Metadata.Gateways) >= 1) {
        AppReq.Snr = ttn.Metadata.Gateways[0].SNR
        AppReq.Location = ttn.Metadata.Gateways[0].GtwID
    }

    ReplyToDeviceID = processBuffer(AppReq, "TTN", "ttn-http:"+ttn.DevID, ttn.PayloadRaw)
    CountTTN++

    // Outbound message processing
    if (ReplyToDeviceID != 0) {

        // Delay just in case there's a chance that request processing may generate a reply
        // to this request.  It's no big deal if we miss it, though, because it will just be
        // picked up on the next call.
        time.Sleep(1 * time.Second)

        // See if there's an outbound message waiting for this device.
        isAvailable, payload := TelecastOutboundPayload(ReplyToDeviceID)
        if (isAvailable) {
            jmsg := &DownlinkMessage{}
            jmsg.DevID = ttn.DevID
            jmsg.FPort = 1
            jmsg.Confirmed = false
            jmsg.PayloadRaw = payload
            jdata, jerr := json.Marshal(jmsg)
            if jerr != nil {
                fmt.Printf("dl j marshaling error: ", jerr)
            } else {

                url := fmt.Sprintf(ttnDownlinkURL, ttnAppId, ttnProcessId, ttnAppAccessKey)

                fmt.Printf("\nHTTP POST to %s\n%s\n\n", url, jdata)

                req, err := http.NewRequest("POST", url, bytes.NewBuffer(jdata))
                req.Header.Set("User-Agent", "TTSERVE")
                req.Header.Set("Content-Type", "text/plain")
                httpclient := &http.Client{
                    Timeout: time.Second * 15,
                }
                resp, err := httpclient.Do(req)
                if err != nil {
                    fmt.Printf("\n*** HTTPS POST error: %v\n\n", err);
                    sendToSafecastOps(fmt.Sprintf("Error transmitting command to device %d: %v\n", ReplyToDeviceID, err))
                } else {
                    resp.Body.Close()
                    sendToSafecastOps(fmt.Sprintf("Device %d picked up its pending command\n", ReplyToDeviceID))
                }

            }
        }

    }

}

// Process a payload buffer
func processBuffer(req IncomingReq, from string, transport string, buf []byte) (DeviceID uint32) {
    var ReplyToDeviceID uint32 = 0
    var AppReq IncomingReq = req

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
        _, ReplyToDeviceID = getDeviceIDFromPayload(AppReq.Payload)

        // Enqueue the app request
        AppReq.UploadedAt = fmt.Sprintf("%s", time.Now().Format("2006-01-02 15:04:05"))
        reqQ <- AppReq
        monitorReqQ()
    }

    case BUFF_FORMAT_PB_ARRAY: {

        fmt.Printf("\n%s Received %d-byte BUFFERED payload from %s %s\n", time.Now().Format(logDateFormat), buf_length, from, AppReq.Transport)

        if !validBulkPayload(buf, buf_length) {
            return 0
        }

        // Loop over the various things in the buffer
        UploadedAt := fmt.Sprintf("%s", time.Now().Format("2006-01-02 15:04:05"))
        count := int(buf[1])
        lengthArrayOffset := 2
        payloadOffset := lengthArrayOffset + count

        for i:=0; i<count; i++ {

            // Extract the length
            length := int(buf[lengthArrayOffset+i])

            // Construct the app request
            AppReq.Payload = buf[payloadOffset:payloadOffset+length]

            // Extract the device ID from the message, which we will need later
            _, ReplyToDeviceID = getDeviceIDFromPayload(AppReq.Payload)

            // Add ServerTime in case the payload lacked CapturedAt
            AppReq.ServerTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")

            fmt.Printf("\n%s Received %d-byte (%d/%d) payload from %s %s\n", time.Now().Format(logDateFormat), len(AppReq.Payload),
                i+1, count, from, AppReq.Transport)

            // Enqueue AppReq
            AppReq.UploadedAt = UploadedAt
            reqQ <- AppReq
            monitorReqQ()

            // Bump the payload offset
            payloadOffset += length;

        }
    }

    default: {
        fmt.Printf("\n%s Received INVALID %d-byte HTTP buffered payload from DEVICE\n", time.Now().Format(logDateFormat), buf_length)
    }
    }

    return ReplyToDeviceID

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

// Function to clean up an error string to eliminate the filename
func errorString(err error) string {
    errString := fmt.Sprintf("%s", err)
    s := strings.Split(errString, ":")
    return s[len(s)-1]
}

// Handle inbound HTTP requests to fetch log files
func inboundWebLogHandler(rw http.ResponseWriter, req *http.Request) {

    // Set response mime type
    rw.Header().Set("Content-Type", "application/json")

    // Log it
    filename := req.RequestURI[len(TTServerTopicLog):]
    fmt.Printf("%s WEB REQUEST for %s\n", time.Now().Format(logDateFormat), filename)

    // Open the file
    file := SafecastDirectory() + TTServerLogPath + "/" + filename
    fd, err := os.Open(file)
    if err != nil {
        io.WriteString(rw, errorString(err))
        return
    }
    defer fd.Close()

    // Copy the file to output
    io.Copy(rw, fd)

}

// Handle inbound HTTP requests to fetch log files
func inboundWebValueHandler(rw http.ResponseWriter, req *http.Request) {

    // Set response mime type
    rw.Header().Set("Content-Type", "application/json")

    // Log it
    filename := req.RequestURI[len(TTServerTopicValue):]
    fmt.Printf("%s WEB REQUEST for %s\n", time.Now().Format(logDateFormat), filename)

    // Open the file
    file := SafecastDirectory() + TTServerValuePath + "/" + filename + ".json"
    fd, err := os.Open(file)
    if err != nil {
        io.WriteString(rw, errorString(err))
        return
    }
    defer fd.Close()

    // Copy the file to output
    io.Copy(rw, fd)

}

// Handle inbound HTTP requests for root
func inboundWebRootHandler(rw http.ResponseWriter, req *http.Request) {

    io.WriteString(rw, fmt.Sprintf("Hello. (%s)\n", TTServerIP))

}

// Handle inbound HTTP requests from the Teletype Gateway
func inboundWebRedirectHandler(rw http.ResponseWriter, req *http.Request) {
    var sV1 SafecastDataV1
    var sV2 SafecastDataV2

    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        fmt.Printf("Error reading HTTP request body: \n%v\n", req)
        return
    }

    // postSafecastV1ToSafecast
    // Attempt to unmarshal it as a Safecast V1 data structure
    err = json.Unmarshal(body, &sV1)
    if (err != nil) {
        if (req.RequestURI != "/" && req.RequestURI != "/favicon.ico") {
            fmt.Printf("\n%s HTTP request '%s' ignored\n", time.Now().Format(logDateFormat), req.RequestURI);
        }
        if (req.RequestURI == "/") {
            io.WriteString(rw, fmt.Sprintf("Live Free or Die. (%s)\n", TTServerIP))
        }
    } else {

        // Convert to V2 format
        sV2 = SafecastV1toV2(sV1)
        fmt.Printf("\n%s Received redirect payload for %d from %s\n", time.Now().Format(logDateFormat), sV2.DeviceID, "pnt-http:"+ipv4(req.RemoteAddr))
        if true {
            fmt.Printf("%s\n", body)
        }

        // For backward compatibility,post it to V1 with an URL that is preserved.  Also post to V2
        urlV1 := SafecastV1UploadURL
        if (req.URL.RawQuery != "") {
            urlV1 = fmt.Sprintf("%s?%s", urlV1, req.URL.RawQuery)
        }
        UploadedAt := fmt.Sprintf("%s", time.Now().Format("2006-01-02 15:04:05"))
        SafecastV1Upload(sV1, urlV1)
        SafecastV2Upload(UploadedAt, sV2)
        SafecastWriteToLogs(UploadedAt, sV2)
        CountHTTPRedirect++

        // It is an error if there is a pending outbound payload for this device, so remove it and report it
        isAvailable, _ := TelecastOutboundPayload(sV2.DeviceID)
        if (isAvailable) {
            sendToSafecastOps(fmt.Sprintf("%d is not capable of processing commands (cancelled)\n", sV2.DeviceID))
        }

    }

}

// Notify Slack if there is an outage
func ttnMQQTSubscriptionNotifier() {
    if (ttnEverConnected) {
        if (!ttnFullyConnected) {
            minutesOffline := int64(time.Now().Sub(ttnLastDisconnectedTime) / time.Minute)
            if (minutesOffline > 15) {
                sendToSafecastOps(fmt.Sprintf("TTN has been unavailable for %d minutes (outage began at %s UTC)", minutesOffline, ttnLastDisconnected))
            }
        } else {
            if (ttnOutages > 1) {
                sendToSafecastOps(fmt.Sprintf("TTN has had %d brief outages in the past 15m", ttnOutages))
                ttnOutages = 0;
            }
        }
    }
}

// Subscribe to TTN inbound messages, then monitor connection status
func ttnMQQTSubscriptionMonitor() {

    for {

        // Allocate and set up the options
        mqttOpts := MQTT.NewClientOptions()
        mqttOpts.AddBroker(ttnServer)
        mqttOpts.SetUsername(ttnAppId)
        mqttOpts.SetPassword(ttnAppAccessKey)

        // Do NOT automatically reconnect upon failure
        mqttOpts.SetAutoReconnect(false)
        mqttOpts.SetCleanSession(true)

        // Handle lost connections
        onMqConnectionLost := func (client MQTT.Client, err error) {
            ttnFullyConnected = false
            ttnLastDisconnectedTime = time.Now()
            ttnLastDisconnected = time.Now().Format(logDateFormat)
            ttnOutages = ttnOutages+1
            fmt.Printf("\n%s *** TTN Connection Lost: %v\n\n", time.Now().Format(logDateFormat), err)
            sendToTTNOps(fmt.Sprintf("Connection lost from this server to %s: %v\n", ttnServer, err))
        }
        mqttOpts.SetConnectionLostHandler(onMqConnectionLost)

        // The "connect" handler subscribes to the topic, and subscribes with a receiver callback
        onMqConnectionMade := func (client MQTT.Client) {

            // Function to process received messages
            onMqMessageReceived := func(client MQTT.Client, message MQTT.Message) {
                ttnUpQ <- message
            }

            // Subscribe to the upstream topic
            if token := client.Subscribe(ttnTopic, 0, onMqMessageReceived); token.Wait() && token.Error() != nil {
                // Treat subscription failure as a connection failure
                fmt.Printf("Error subscribing to topic %s\n", ttnTopic, token.Error())
                ttnFullyConnected = false
                ttnLastDisconnectedTime = time.Now()
                ttnLastDisconnected = time.Now().Format(logDateFormat)
            } else {
                // Successful subscription
                ttnFullyConnected = true
                ttnLastConnected = time.Now().Format(logDateFormat)
                if (ttnEverConnected) {
                    minutesOffline := int64(time.Now().Sub(ttnLastDisconnectedTime) / time.Minute)
                    // Don't bother reporting quick outages, generally caused by server restarts
                    if (minutesOffline >= 5) {
                        sendToSafecastOps(fmt.Sprintf("TTN returned (%d-minute outage began at %s UTC)", minutesOffline, ttnLastDisconnected))
                    }
                    sendToTTNOps(fmt.Sprintf("Connection restored from this server to %s\n", ttnServer))
                    fmt.Printf("\n%s *** TTN Connection Restored\n\n", time.Now().Format(logDateFormat))
                } else {
                    ttnEverConnected = true
                    fmt.Printf("TTN Connection Established\n")
                }
            }

        }
        mqttOpts.SetOnConnectHandler(onMqConnectionMade)

        // Create the client session context, saving it
        // in a global so that it may also be used to Publish
        ttnMqttClient = MQTT.NewClient(mqttOpts)

        // Connect to the service
        if token := ttnMqttClient.Connect(); token.Wait() && token.Error() != nil {
            fmt.Printf("Error connecting to TTN service: %s\n", token.Error())
            time.Sleep(60 * time.Second);
        } else {

            fmt.Printf("Now handling inbound MQTT on: %s mqtt:%s\n", ttnServer, ttnTopic)
            for consecutiveFailures := 0; consecutiveFailures < 3; {
                time.Sleep(60 * time.Second);
                if ttnFullyConnected {
                    if false {
                        fmt.Printf("\n%s TTN Alive\n", time.Now().Format(logDateFormat))
                    }
                    consecutiveFailures = 0
                } else {
                    fmt.Printf("\n%s TTN *** UNREACHABLE ***\n", time.Now().Format(logDateFormat))
                    consecutiveFailures += 1
                }
            }

        }

        // Failure
        mqttOpts = nil
        ttnMqttClient = nil
        time.Sleep(5 * time.Second)
        fmt.Printf("\n***\n")
        fmt.Printf("*** Last time connection was successfully made: %s\n", ttnLastConnected)
        fmt.Printf("*** Last time connection was lost: %s\n", ttnLastDisconnected)
        fmt.Printf("*** Now attempting to reconnect: %s\n", time.Now().Format(logDateFormat))
        fmt.Printf("***\n\n")

    }
}

// Send to a ttn device outbound
func ttnOutboundPublish(devEui string, payload []byte) {
    if ttnFullyConnected {
        jmsg := &DownlinkMessage{}
        jmsg.PayloadRaw = payload
        jmsg.FPort = 1
        jdata, jerr := json.Marshal(jmsg)
        if jerr != nil {
            fmt.Printf("j marshaling error: ", jerr)
        }
        topic := ttnAppId + "/devices/" + devEui + "/down"
        fmt.Printf("Send %s: %s\n", topic, jdata)
        ttnMqttClient.Publish(topic, 0, false, jdata)
    }
}

// Handle inbound pulled from TTN's upstream mqtt message queue
func ttnMQQTInboundHandler() {

    // Dequeue and process the messages as they're enqueued
    for msg := range ttnUpQ {
        var ttn UplinkMessage
        var AppReq IncomingReq

        // Unmarshal the payload and extract the base64 data
        err := json.Unmarshal(msg.Payload(), &ttn)
        if err != nil {
            fmt.Printf("\n*** Payload doesn't have TTN data *** %v\n%s\n\n", err, msg.Payload())
        } else {

            // Copy fields to the app request structure
            AppReq.Payload = ttn.PayloadRaw
            AppReq.TTNDevID = ttn.DevID
            AppReq.Longitude = ttn.Metadata.Longitude
            AppReq.Latitude = ttn.Metadata.Latitude
            AppReq.Altitude = float32(ttn.Metadata.Altitude)
            if (len(ttn.Metadata.Gateways) >= 1) {
                AppReq.Snr = ttn.Metadata.Gateways[0].SNR
                AppReq.Location = ttn.Metadata.Gateways[0].GtwID
            }

            AppReq.Transport = "ttn-mqqt:" + AppReq.TTNDevID
            fmt.Printf("\n%s Received %d-byte payload from %s\n", time.Now().Format(logDateFormat), len(AppReq.Payload), AppReq.Transport)
            AppReq.UploadedAt = fmt.Sprintf("%s", time.Now().Format("2006-01-02 15:04:05"))
            reqQ <- AppReq
            monitorReqQ()
            CountTTN++

            // See if there's an outbound message waiting for this app.  If so, send it now because we
            // know that there's a narrow receive window open.
            isAvailable, deviceID := getDeviceIDFromPayload(AppReq.Payload)
            if isAvailable {
                isAvailable, payload := TelecastOutboundPayload(deviceID)
                if (isAvailable) {
                    ttnOutboundPublish(AppReq.TTNDevID, payload)
                    sendToSafecastOps(fmt.Sprintf("Device %d picked up its pending command\n", deviceID))
                }
            }
        }

    }

}

// Get any outbound payload waiting for the node who sent us an AppReq.  If
// the device ID is not found, guarantee that a 0 is returned for the device ID.
func getDeviceIDFromPayload(inboundPayload []byte) (isAvailable bool, deviceID uint32) {

    // Extract the telecast message from the AppReq
    msg := &teletype.Telecast{}
    err := proto.Unmarshal(inboundPayload, msg)
    if err != nil {
        return false, 0
    }

    return true, TelecastDeviceID(msg)

}

// Common handler for messages incoming either from TTN or HTTP
func commonRequestHandler() {

    // Dequeue and process the messages as they're enqueued
    for AppReq := range reqQ {

        // Unmarshal the message
        msg := &teletype.Telecast{}
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
        deviceID := TelecastDeviceID(msg)
        fmt.Printf("%s sent by %d\n", time.Now().Format(logDateFormat), deviceID)
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
        case teletype.Telecast_BGEIGIE_NANO:
            fallthrough
        case teletype.Telecast_SOLARCAST:
            ProcessSafecastMessage(msg, checksum, AppReq.Location, AppReq.UploadedAt, AppReq.Transport,
                AppReq.ServerTime,
                AppReq.Snr,
                AppReq.Latitude, AppReq.Longitude, AppReq.Altitude)

            // Handle messages from non-safecast devices
        default:
            ProcessTelecastMessage(msg, AppReq.TTNDevID)
        }
    }
}

// Monitor the queue length
func monitorReqQ() {
    elements := len(reqQ)

    if (elements > reqQMaxLength) {
        reqQMaxLength = elements

        if (reqQMaxLength > 1) {
            fmt.Printf("\n%s Requests pending reached new maximum of %d\n", time.Now().Format(logDateFormat), reqQMaxLength)
        }

    }

    // We have observed once that the HTTP stack got messed up to the point where the queue just grew forever
    // because nothing was getting serviced.  In this case, abort and restart the process.
    if (reqQMaxLength >= MAX_PENDING_REQUESTS) {
        fmt.Printf("\n***\n***\n*** RESTARTING defensively because of request queue overflow\n***\n***\n\n")
        os.Exit(0)
    }

}

// Record the occurrence of another impossible error.
// If this is called too many times, restart the server, for robustness.
var impossibleErrorCount = 0;

func impossibleError() {

    impossibleErrorCount = impossibleErrorCount + 1

    if (impossibleErrorCount < 25) {
        return;
    }

    fmt.Printf("\n***\n***\n*** RESTARTING defensively because of impossible errors\n***\n***\n\n")
    os.Exit(0)

}

// Our app's signal handler
func signalHandler() {
    ch := make(chan os.Signal)
    signal.Notify(ch, syscall.SIGTERM)
    signal.Notify(ch, syscall.SIGINT)
    signal.Notify(ch, syscall.SIGSEGV)
    for {
        switch <-ch {
        case syscall.SIGINT:
            fmt.Printf("\n***\n***\n*** Exiting at user's request \n***\n***\n\n")
            os.Exit(0)
        case syscall.SIGTERM:
            ftpServer.Stop()
            break
        }
    }
}

// Extract just the IPV4 address, eliminating the port
func ipv4(Str1 string) string {
    Str2 := strings.Split(Str1, ":")
    if len(Str2) > 0 {
        return Str2[0]
    }
    return Str1
}

// Get the modified time of a special file
func ControlFileTime(controlfilename string, message string) (restartTime time.Time) {

    filename := SafecastDirectory() + TTServerControlPath + "/" + controlfilename

    // Overwrite the file if requested to do so
    if (message != "") {
        fd, err := os.OpenFile(filename, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0666)
        if (err == nil) {
            fd.WriteString(message);
            fd.Close();
        }
    }

    // Get the file date/time, returning a stable time if we fail
    file, err := os.Stat(filename)
    if err != nil {
        return TTServerBootTime
    }

    return file.ModTime()
}
