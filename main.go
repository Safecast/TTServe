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

// Derived from "ttnctl applications", the AppEUI and its Access Key
const appEui string = "70B3D57ED0000420"
const appAccessKey string = "bgCzOOs/5K16cuwP3/sGP9sea/4jKFwFEdTbYHw2fRE="

// Derived from https://staging.thethingsnetwork.org/wiki/Backend/Connect/Application
const ttnServer string = "tcp://staging.thethingsnetwork.org:1883"
const ttnTopic string = appEui + "/devices/+/up"

// Safecast-related
const SafecastV1UploadIP = "107.161.164.163"
const SafecastV1UploadURL = "http://" + SafecastV1UploadIP + "/scripts/indextest.php"
const SafecastV1QueryString = "api_key=z3sHhgousVDDrCVXhzMT"
var SafecastV2UploadURLs = [...]string {
    "http://ingest.safecast.org/v1/measurements",
}

// File system related paths relative to the server's HomeDir
const TTServerLogPath = "/safecast/log"
const TTServerCommandPath = "/safecast/command"
const TTServerBuildPath = "/safecast/build"
const TTServerFTPCertPath = "/safecast/cert/ftp"

// Buffered I/O header formats.  Note that although we are now starting with version number 0, we
// special case version number 8 because of the old style "single protocl buffer" message format that
// always begins with 0x08. (see ttnode/send.c)
const BUFF_FORMAT_PB_ARRAY byte  =  0
const BUFF_FORMAT_SINGLE_PB byte =  8

// This server-related
const TTServerHTTPAddress = "tt.safecast.org"
const TTServerUDPAddress = "tt-udp.safecast.org"
var	  TTServerUDPAddressIPv4 = ""
var	  iAmTTServerUDP = false
const TTServerFTPAddress = "tt-ftp.safecast.org"
var	  TTServerFTPAddressIPv4 = ""
var	  iAmTTServerFTP = false
const TTServerPortHTTP string = ":8080"
const TTServerPortHTTPAlternate string = ":80"
const TTServerPortUDP string = ":8081"
const TTServerPortFTP int = 8083    // plus 8084, and entire passive range
const TTServerTopicSend string = "/send"
const TTServerTopicTest string = "/test"
const TTServerTopicLog string = "/log/"
const TTServerTopicGithub string = "/github"
const TTServerTopicSlack string = "/slack"
const TTServerTopicRedirect1 string = "/scripts/"
const TTServerTopicRedirect2 string = "/"

// Our server
var TTServer string
var TTServerIP string

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
    TTN DataUpAppReq
    Transport string
    UploadedAt string
}
var reqQ chan IncomingReq
var reqQMaxLength = 0

// Main entry point for app
func main() {

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
        fmt.Printf("Can't resolve %s: %v\n", TTServerFTPAddressIPv4, err);
        os.Exit(0)
    }
	if len(addrs) < 1 {
        fmt.Printf("Can't resolve %s: %v\n", TTServerFTPAddressIPv4, err);
        os.Exit(0)
    }
	TTServerFTPAddressIPv4 = addrs[0]
	iAmTTServerFTP = TTServerFTPAddressIPv4 == TTServerIP

	// Display these truths
	fmt.Printf("'%s' '%s' '%s' %v %v\n", TTServerIP, TTServerUDPAddress, TTServerFTPAddress, iAmTTServerUDP, iAmTTServerFTP)

    // Set up our signal handler
    go signalHandler()

    // Set up our internal message queues
    ttnUpQ = make(chan MQTT.Message, 5)
    reqQ = make(chan IncomingReq, MAX_PENDING_REQUESTS)

    // Spawn the app request handler shared by both TTN and direct inbound server
    go commonRequestHandler()

    // Spawn the TTN inbound message handler
    go ttnInboundHandler()

    // Init our web request inbound server
    go webInboundHandler()

    // Init our UDP single-sample upload request inbound server
    go udpInboundHandler()

    // Init our FTP server
    go ftpInboundHandler()

    // Init our housekeeping process
    go timer1m()

    // Init our housekeeping process
    go timer15m()

    // Handle the inboound subscriber.  (This never returns.)
    ttnSubscriptionMonitor()

}

// General periodic housekeeping
func timer1m() {
    for {
        time.Sleep(1 * 60 * time.Second)
    }
}

// General periodic housekeeping
func timer15m() {
    for {

		// Track expired devices.  We do this before the first sleep so we have a list of device ASAP
        sendExpiredSafecastDevicesToSlack()

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

// Kick off inbound messages coming from all sources, then serve HTTP
func ftpInboundHandler() {

    fmt.Printf("Now handling inbound FTP on: %s:%d\n", TTServer, TTServerPortFTP)

    ftpServer = ftp.NewFtpServer(NewTeletypeDriver())
    err := ftpServer.ListenAndServe()
    if err != nil {
        fmt.Printf("Error listening on FTP: %s\n", err)
    }

}

// Kick off inbound messages coming from all sources, then serve HTTP
func webInboundHandler() {

    http.HandleFunc(TTServerTopicSend, inboundWebSendHandler)
    fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServer, TTServerPortHTTP, TTServerTopicSend)

    http.HandleFunc(TTServerTopicGithub, inboundWebGithubHandler)
    fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServer, TTServerPortHTTP, TTServerTopicGithub)

    http.HandleFunc(TTServerTopicSlack, inboundWebSlackHandler)
    fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServer, TTServerPortHTTP, TTServerTopicSlack)

    http.HandleFunc(TTServerTopicRedirect1, inboundWebRedirectHandler)
    fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServer, TTServerPortHTTP, TTServerTopicRedirect1)

    http.HandleFunc(TTServerTopicRedirect2, inboundWebRedirectHandler)
    fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServer, TTServerPortHTTP, TTServerTopicRedirect2)

    http.HandleFunc(TTServerTopicLog, inboundWebLogHandler)

    http.HandleFunc(TTServerTopicTest, inboundWebTestHandler)

    go func() {
        http.ListenAndServe(TTServerPortHTTPAlternate, nil)
    }()

    http.ListenAndServe(TTServerPortHTTP, nil)

}

// Kick off UDP single-upload request server
func udpInboundHandler() {

    fmt.Printf("Now handling inbound UDP single-uploads on: %s%s\n", TTServer, TTServerPortUDP)

    ServerAddr, err := net.ResolveUDPAddr("udp", TTServerPortUDP)
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
            buf_format := buf[0]
            buf_length := n

            switch (buf_format) {

            case BUFF_FORMAT_SINGLE_PB: {

                // Construct a TTN-like message
                //  1) the Payload comes from UDP
                //  2) We'll add the server's time, in case the payload lacked CapturedAt
                //  3) Everything else is null

                var AppReq IncomingReq
                AppReq.TTN.Payload = buf[0:buf_length]
                AppReq.TTN.Metadata = make([]AppMetadata, 1)
                AppReq.TTN.Metadata[0].ServerTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")

                fmt.Printf("\n%s Received %d-byte UDP payload from %s\n", time.Now().Format(logDateFormat), len(AppReq.TTN.Payload), addr)

                // Enqueue it for TTN-like processing
                AppReq.Transport = "udp:" + addr.String()
                AppReq.UploadedAt = fmt.Sprintf("%s", time.Now().Format("2006-01-02 15:04:05"))
                reqQ <- AppReq
                monitorReqQ()

            }

            case BUFF_FORMAT_PB_ARRAY: {

	            fmt.Printf("\n%s Received %d-byte UDP BUFFERED payload from %s\n", time.Now().Format(logDateFormat), buf_length, addr)

                if !validBulkPayload(buf, buf_length) {
                    continue;
                }

                // Loop over the various things in the buffer
                UploadedAt := fmt.Sprintf("%s", time.Now().Format("2006-01-02 15:04:05"))
                count := int(buf[1])
                lengthArrayOffset := 2
                payloadOffset := lengthArrayOffset + count

                for i:=0; i<count; i++ {

                    // Extract the length
                    length := int(buf[lengthArrayOffset+i])

                    // Construct a TTN-like message
                    //  1) the Payload comes from UDP
                    //  2) We'll add the server's time, in case the payload lacked CapturedAt
                    //  3) Everything else is null

                    var AppReq IncomingReq
                    AppReq.TTN.Payload = buf[payloadOffset:payloadOffset+length]
                    AppReq.TTN.Metadata = make([]AppMetadata, 1)
                    AppReq.TTN.Metadata[0].ServerTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")

                    fmt.Printf("\n%s Received %d-byte UDP (%d/%d) payload from %s\n", time.Now().Format(logDateFormat), len(AppReq.TTN.Payload),
                        i+1, count, addr)

                    // Enqueue it for TTN-like processing
                    AppReq.Transport = "udp:" + addr.String()
                    AppReq.UploadedAt = UploadedAt
                    reqQ <- AppReq
                    monitorReqQ()

                    // Bump the payload offset
                    payloadOffset += length;
                }
            }

            default: {

                fmt.Printf("\n%s Received INVALID %d-byte UDP payload from %s\n", time.Now().Format(logDateFormat), buf_length, addr)

            }

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

        // Messages from TTGATE are binary and take the form of a TTN-compatible payload
    case "TTGATE": {

        err = json.Unmarshal(body, &AppReq.TTN)
        if err != nil {
            io.WriteString(rw, "Hello.")
            // Very common case where anyone comes to web page, such as a google health check or our own TTGATE health check
            return
        }

        // For now we only handle single-PB format from TTGATE
        if (AppReq.TTN.Payload[0] != BUFF_FORMAT_SINGLE_PB) {
            return
        }

        fmt.Printf("\n%s Received %d-byte HTTP payload from TTGATE\n", time.Now().Format(logDateFormat), len(AppReq.TTN.Payload))

		// Extract the device ID from the message, which we will need later
	    _, ReplyToDeviceID = getDeviceIDFromPayload(AppReq.TTN.Payload)
		
        // We now have a TTN-like message, constructed as follows:
        //  1) the Payload came from the device itself
        //  2) TTGATE filled in the lat/lon/alt metadata, just in case the payload doesn't have location
        //  3) TTGATE filled in SNR if it had access to it
        //  4) We'll add the server's time, in case the payload lacked CapturedAt
        AppReq.TTN.Metadata[0].ServerTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")

        // Enqueue AppReq for TTN-like processing
        AppReq.Transport = "http:" + req.RemoteAddr
        AppReq.UploadedAt = fmt.Sprintf("%s", time.Now().Format("2006-01-02 15:04:05"))
        reqQ <- AppReq
        monitorReqQ()

    }

        // Messages from devices are hexified
    case "TTNODE": {

        buf, err := hex.DecodeString(string(body))
        if err != nil {
            fmt.Printf("Hex decoding error: ", err)
            return
        }

        buf_format := buf[0]
        buf_length := len(buf)

        switch (buf_format) {

        case BUFF_FORMAT_SINGLE_PB: {

            fmt.Printf("\n%s Received %d-byte HTTP payload from DEVICE\n", time.Now().Format(logDateFormat), buf_length)

            // We now have a TTN-like message, constructed as follws:
            //  1) the Payload came from the device itself
            //  2) We'll add the server's time, in case the payload lacked CapturedAt
            AppReq.TTN.Payload = buf
            AppReq.TTN.Metadata = make([]AppMetadata, 1)
            AppReq.TTN.Metadata[0].ServerTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")

			// Extract the device ID from the message, which we will need later
		    _, ReplyToDeviceID = getDeviceIDFromPayload(AppReq.TTN.Payload)

            // Enqueue AppReq for TTN-like processing
            AppReq.Transport = "http:" + req.RemoteAddr
            AppReq.UploadedAt = fmt.Sprintf("%s", time.Now().Format("2006-01-02 15:04:05"))
            reqQ <- AppReq
            monitorReqQ()
        }

        case BUFF_FORMAT_PB_ARRAY: {

            fmt.Printf("\n%s Received %d-byte HTTP BUFFERED payload from DEVICE\n", time.Now().Format(logDateFormat), buf_length)

            if !validBulkPayload(buf, buf_length) {
                return
            }

            // Loop over the various things in the buffer
            UploadedAt := fmt.Sprintf("%s", time.Now().Format("2006-01-02 15:04:05"))
            count := int(buf[1])
            lengthArrayOffset := 2
            payloadOffset := lengthArrayOffset + count

            for i:=0; i<count; i++ {

                // Extract the length
                length := int(buf[lengthArrayOffset+i])

                // Construct the TTN-like messager
                AppReq.TTN.Payload = buf[payloadOffset:payloadOffset+length]

				// Extract the device ID from the message, which we will need later
			    _, ReplyToDeviceID = getDeviceIDFromPayload(AppReq.TTN.Payload)

                // We now have a TTN-like message, constructed as follws:
                //  1) the Payload came from the device itself
                //  2) We'll add the server's time, in case the payload lacked CapturedAt
                AppReq.TTN.Metadata = make([]AppMetadata, 1)
                AppReq.TTN.Metadata[0].ServerTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")

                fmt.Printf("\n%s Received %d-byte HTTP (%d/%d) payload from %s\n", time.Now().Format(logDateFormat), len(AppReq.TTN.Payload),
                    i+1, count, req.RemoteAddr)

                // Enqueue AppReq for TTN-like processing
                AppReq.Transport = "http:" + req.RemoteAddr
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
            hexPayload := hex.EncodeToString(payload)
            io.WriteString(rw, hexPayload)
            sendToSafecastOps(fmt.Sprintf("Device %d picked up its pending command\n", ReplyToDeviceID))
        }

    }

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

// Handle inbound HTTP requests to test
func inboundWebTestHandler(rw http.ResponseWriter, req *http.Request) {

    fmt.Printf("***** Test *****\n")
    io.WriteString(rw, "This is a test of the emergency broadcast system.\n")

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
            fmt.Printf("\n%s HTTP request '%s' does not contain valid Safecast JSON\n", time.Now().Format(logDateFormat), req.RequestURI);
        }
        if (req.RequestURI == "/") {
            io.WriteString(rw, "Live Free or Die.\n")
        }
    } else {

        // Convert to V2 format
        sV2 = SafecastV1toV2(sV1)
        fmt.Printf("\n%s Received redirect payload from Device ID %d\n%s\n", time.Now().Format(logDateFormat), sV2.DeviceID, body)

        // For backward compatibility,post it to V1 with an URL that is preserved.  Also post to V2
        urlV1 := SafecastV1UploadURL
        if (req.URL.RawQuery != "") {
            urlV1 = fmt.Sprintf("%s?%s", urlV1, req.URL.RawQuery)
        }
        UploadedAt := fmt.Sprintf("%s", time.Now().Format("2006-01-02 15:04:05"))
        SafecastV1Upload(sV1, urlV1)
        SafecastV2Upload(UploadedAt, sV2)
        SafecastV2Log(UploadedAt, sV2)

		// It is an error if there is a pending outbound payload for this device, so remove it and report it
        isAvailable, _ := TelecastOutboundPayload(sV2.DeviceID)
		if (isAvailable) {
	        sendToSafecastOps(fmt.Sprintf("%d is not capable of processing commands (cancelled)\n", sV2.DeviceID))
		}
		
    }

}

// Subscribe to TTN inbound messages, then monitor connection status
func ttnSubscriptionMonitor() {

    for {

        // Allocate and set up the options
        mqttOpts := MQTT.NewClientOptions()
        mqttOpts.AddBroker(ttnServer)
        mqttOpts.SetUsername(appEui)
        mqttOpts.SetPassword(appAccessKey)

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
            fmt.Printf("Error connecting to service: %s\n", token.Error())
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
        jmsg := &DataDownAppReq{}
        jmsg.Payload = payload
        jmsg.FPort = 1
        jmsg.TTL = "1h"
        jdata, jerr := json.Marshal(jmsg)
        if jerr != nil {
            fmt.Printf("j marshaling error: ", jerr)
        }
        topic := appEui + "/devices/" + devEui + "/down"
        fmt.Printf("Send %s: %s\n", topic, jdata)
        ttnMqttClient.Publish(topic, 0, false, jdata)
    }
}

// Handle inbound pulled from TTN's upstream mqtt message queue
func ttnInboundHandler() {

    // Dequeue and process the messages as they're enqueued
    for msg := range ttnUpQ {
        var AppReq IncomingReq

        // Unmarshal the payload and extract the base64 data
        err := json.Unmarshal(msg.Payload(), &AppReq.TTN)
        if err != nil {
            fmt.Printf("*** Payload doesn't have TTN data ***\n")
        } else {
            fmt.Printf("\n%s Received %d-byte payload from TTN\n", time.Now().Format(logDateFormat), len(AppReq.TTN.Payload))
            // Note that there is some missing code here.  Ideally, in the appreq
            // we supply json-formatted IPINFO.  TTGATE tunneled this through
            // the GatewayEUI field of the DataUpAppReq, but in the TTN case we don't
            // have such a luxury.  That said, the DataUpAppReq for TTN DOES have the
            // Latitude/Longitude fields to work with.  Ideally we would then use
            // TinyGeocoder (or Yahoo or Google) *here* and would then encode the results
            // in the GatewayEUI field in a way that is compatible with TTGATE.
            // I haven't written this code simply because this requires registering
            // for a Yahoo/Google account, and potentially paying.  Since none of the
            // code here is actually utilizing geo, we'll wait until then.
            AppReq.Transport = "ttn:" + AppReq.TTN.DevEUI
            AppReq.UploadedAt = fmt.Sprintf("%s", time.Now().Format("2006-01-02 15:04:05"))
            reqQ <- AppReq
            monitorReqQ()

            // See if there's an outbound message waiting for this app.  If so, send it now because we
            // know that there's a narrow receive window open.
            isAvailable, deviceID := getDeviceIDFromPayload(AppReq.TTN.Payload)
            if isAvailable {
                isAvailable, payload := TelecastOutboundPayload(deviceID)
                if (isAvailable) {
                    ttnOutboundPublish(AppReq.TTN.DevEUI, payload)
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
        err := proto.Unmarshal(AppReq.TTN.Payload, msg)
        if err != nil {
            fmt.Printf("*** PB unmarshaling error: \n", err)
            fmt.Printf("*** ");
            for i:=0; i<len(AppReq.TTN.Payload); i++ {
                fmt.Printf("%02x", AppReq.TTN.Payload[i]);
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
        var nullDeviceID uint32 = 0
        msg.RelayDevice1 = &nullDeviceID
        msg.RelayDevice2 = &nullDeviceID
        msg.RelayDevice3 = &nullDeviceID
        msg.RelayDevice4 = &nullDeviceID
        msg.RelayDevice5 = &nullDeviceID
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
            metadata := AppReq.TTN.Metadata[0]
            ProcessSafecastMessage(msg, checksum, metadata.GatewayEUI, AppReq.UploadedAt, AppReq.Transport,
                metadata.ServerTime,
                metadata.Lsnr,
                metadata.Latitude, metadata.Longitude, metadata.Altitude)

            // Handle messages from non-safecast devices
        default:
            ProcessTelecastMessage(msg, AppReq.TTN.DevEUI)
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
    for {
        switch <-ch {
        case syscall.SIGTERM:
            ftpServer.Stop()
            break
        }
    }
}
