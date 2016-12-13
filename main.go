// Teletype Message Publishing Service
package main

import (
    "os"
	"os/signal"
	"syscall"
    "io"
    "io/ioutil"
    "net/http"
    "net"
    "fmt"
    "time"
    "encoding/json"
    "encoding/hex"
    "github.com/rdegges/go-ipify"
    "github.com/golang/protobuf/proto"
    "github.com/rayozzie/teletype-proto/golang"
	"github.com/fclairamb/ftpserver/server"
    "hash/crc32"
    MQTT "github.com/eclipse/paho.mqtt.golang"
)

// Derived from "ttnctl applications", the AppEUI and its Access Key
const appEui string = "70B3D57ED0000420"
const appAccessKey string = "bgCzOOs/5K16cuwP3/sGP9sea/4jKFwFEdTbYHw2fRE="

// Derived from https://staging.thethingsnetwork.org/wiki/Backend/Connect/Application
const ttnServer string = "tcp://staging.thethingsnetwork.org:1883"
const ttnTopic string = appEui + "/devices/+/up"

// Safecast-related
const SafecastUploadIP = "107.161.164.163"
const SafecastUploadURL = "http://" + SafecastUploadIP + "/scripts/indextest.php?api_key=%s"
const SafecastAppKey = "z3sHhgousVDDrCVXhzMT"

// File system related paths relative to the server's HomeDir
const TTServerLogPath = "/safecast/log"
const TTServerBuildPath = "/safecast/build"

// This server-related
const TTServerAddress = "api.teletype.io"
const TTServerPortFTP int = 21
const TTServerPort string = ":8080"
const TTServerPortUDP string = ":8081"
const TTServerPortTCP string = ":8082"
const TTServerURLSend string = "/send"
const TTServerURLGithub string = "/github"
const TTServerURLSlack string = "/slack"

// Our server
var TTServer string

// Constants
const logDateFormat string = "2006-01-02 15:04:05"

// FTP
var (
	ftpServer *server.FtpServer
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
var MAX_PENDING_REQUESTS int = 25

// Common app request
type IncomingReq struct {
    TTN DataUpAppReq
    Transport string
}
var reqQ chan IncomingReq
var reqQMaxLength = 0

// Main entry point for app
func main() {

    // Get our external IP address
    ip, err := ipify.GetIp()
    if err != nil {
        TTServer = "http://" + TTServerAddress
    } else {
        TTServer = "http://" + ip
    }

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

    // Init our UDP request inbound server
    go udpInboundHandler()

    // Init our UDP request inbound server
    go tcpInboundHandler()

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
        sendExpiredSafecastDevicesToSlack()
    }
}

// General periodic housekeeping
func timer15m() {
    for {
        time.Sleep(15 * 60 * time.Second)

        // Report maximum inbound pending transactions
        if (reqQMaxLength > 1) {
            fmt.Printf("%s Maximum request queue length reached %d\n", time.Now().Format(logDateFormat), reqQMaxLength)
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

	ftpServer = server.NewFtpServer(NewTeletypeDriver())
	err := ftpServer.ListenAndServe()
	if err != nil {
	    fmt.Printf("Error listening on FTP: %s\n", err)
	}
	
}

// Kick off inbound messages coming from all sources, then serve HTTP
func webInboundHandler() {

    http.HandleFunc(TTServerURLSend, inboundWebTTGateHandler)
    fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServer, TTServerPort, TTServerURLSend)

    http.HandleFunc(TTServerURLGithub, inboundWebGithubHandler)
    fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServer, TTServerPort, TTServerURLGithub)

    http.HandleFunc(TTServerURLSlack, inboundWebSlackHandler)
    fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", TTServer, TTServerPort, TTServerURLSlack)

    http.ListenAndServe(TTServerPort, nil)
}

// Kick off UDP server
func udpInboundHandler() {

    fmt.Printf("Now handling inbound UDP on: %s%s\n", TTServer, TTServerPortUDP)

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
        buf := make([]byte, 1024)

        n, addr, err := ServerConn.ReadFromUDP(buf)
        if (err != nil) {
            fmt.Printf("UDP read error: \n%v\n", err)
            time.Sleep(1 * 60 * time.Second)
        } else {

            // Construct a TTN-like message
            //  1) the Payload comes from UDP
            //  2) We'll add the server's time, in case the payload lacked CapturedAt
            //  3) Everything else is null

            var AppReq IncomingReq
            AppReq.TTN.Payload = buf[0:n]
            AppReq.TTN.Metadata = make([]AppMetadata, 1)
            AppReq.TTN.Metadata[0].ServerTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")

            fmt.Printf("\n%s Received %d-byte UDP payload from %s\n", time.Now().Format(logDateFormat), len(AppReq.TTN.Payload), addr)

            // Enqueue it for TTN-like processing
            AppReq.Transport = "udp:" + addr.String()
            reqQ <- AppReq
            monitorReqQ()

        }
    }

}

// Kick off TCP server
func tcpInboundHandler() {

    fmt.Printf("Now handling inbound TCP on: %s%s\n", TTServer, TTServerPortTCP)

    ServerAddr, err := net.ResolveTCPAddr("tcp", TTServerPortTCP)
    if err != nil {
        fmt.Printf("Error resolving TCP port: \n%v\n", err)
        return
    }

    ServerConn, err := net.ListenTCP("tcp", ServerAddr)
    if err != nil {
        fmt.Printf("Error listening on TCP port: \n%v\n", err)
        return
    }
    defer ServerConn.Close()

    for {
        buf := make([]byte, 1024)

        conn, err := ServerConn.AcceptTCP()
        if err != nil {
            fmt.Printf("Error accepting TCP session: \n%v\n", err)
			// We see "use of closed network connection" when port scanners hit us
            time.Sleep(10 * time.Second)
            continue
        }

        n, err := conn.Read(buf)
        if err != nil {
            fmt.Printf("TCP read error: \n%v\n", err)
            ServerConn.Close()
            time.Sleep(1 * 60 * time.Second)
            continue
        }

        // Construct a TTN-like message
        //  1) the Payload comes from UDP
        //  2) We'll add the server's time, in case the payload lacked CapturedAt
        //  3) Everything else is null
        var AppReq IncomingReq
        AppReq.TTN.Payload = buf[0:n]
        AppReq.TTN.Metadata = make([]AppMetadata, 1)
        AppReq.TTN.Metadata[0].ServerTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")

        // Test the received data to see if it's just some random port scanner trying to probe what we are
        msg := &teletype.Telecast{}
        err = proto.Unmarshal(AppReq.TTN.Payload, msg)
        if (err != nil) {

            fmt.Printf("\n%s Ignoring %d-byte TCP port scan probe from %s\n", time.Now().Format(logDateFormat), len(AppReq.TTN.Payload), conn.RemoteAddr().String())

        } else {

            fmt.Printf("\n%s Received %d-byte TCP payload from %s\n", time.Now().Format(logDateFormat), len(AppReq.TTN.Payload), conn.RemoteAddr().String())

            // Enqueue it for TTN-like processing
            AppReq.Transport = "tcp:" + conn.RemoteAddr().String()
            reqQ <- AppReq
            monitorReqQ()

            // Delay to see if we can pick up a reply for this request.  This is certainly
            // controversial because it slows down the incoming message processing, however there
            // is a trivial fix:  Create many instances of this goroutine on the service instead
            // of just one.
            time.Sleep(1 * time.Second)

            // See if there's an outbound message waiting for this app.  If so, send it now because we
            // know that there's a narrow receive window open.
            isAvailable, outboundPayload := getOutboundPayload(AppReq.TTN.Payload)
            if isAvailable {
                conn.Write(outboundPayload)
            }

        }

        // Close the connection
        conn.Close()
    }

}

// Handle inbound HTTP requests from the Teletype Gateway
func inboundWebTTGateHandler(rw http.ResponseWriter, req *http.Request) {
    var AppReq IncomingReq

    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        fmt.Printf("Error reading HTTP request body: \n%v\n", req)
        return
    }

    switch (req.UserAgent()) {

    case "TTGATE": {

        err = json.Unmarshal(body, &AppReq.TTN)
        if err != nil {
            io.WriteString(rw, "Hello.")
            // Very common case where anyone comes to web page, such as a google health check or our own TTGATE health check
            return
        }

        fmt.Printf("\n%s Received %d-byte HTTP payload from TTGATE\n", time.Now().Format(logDateFormat), len(AppReq.TTN.Payload))

        // We now have a TTN-like message, constructed as follows:
        //  1) the Payload came from the device itself
        //  2) TTGATE filled in the lat/lon/alt metadata, just in case the payload doesn't have location
        //  3) TTGATE filled in SNR if it had access to it
        //  4) We'll add the server's time, in case the payload lacked CapturedAt
        AppReq.TTN.Metadata[0].ServerTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")

    }

    case "TTRELAY": {

        inboundPayload, err := hex.DecodeString(string(body))
        if err != nil {
            fmt.Printf("Hex decoding error: ", err)
            return
        }

        AppReq.TTN.Payload = inboundPayload
        fmt.Printf("\n%s Received %d-byte HTTP payload from DEVICE\n", time.Now().Format(logDateFormat), len(AppReq.TTN.Payload))

        // We now have a TTN-like message, constructed as follws:
        //  1) the Payload came from the device itself
        //  2) We'll add the server's time, in case the payload lacked CapturedAt
        AppReq.TTN.Metadata = make([]AppMetadata, 1)
        AppReq.TTN.Metadata[0].ServerTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")

    }

    default: {

        // A web crawler, etc.
        return

    }

    }

    // Enqueue AppReq for TTN-like processing
    AppReq.Transport = "http:" + req.RemoteAddr
    reqQ <- AppReq
    monitorReqQ()

    // Delay to see if we can pick up a reply for this request.  This is certainly
    // controversial because it slows down the incoming message processing, however there
    // is a trivial fix:  Create many instances of this goroutine on the service instead
    // of just one.
    time.Sleep(1 * time.Second)

    // See if there's an outbound message waiting for this app.  If so, send it now because we
    // know that there's a narrow receive window open.
    isAvailable, outboundPayload := getOutboundPayload(AppReq.TTN.Payload)
    if isAvailable {
        hexPayload := hex.EncodeToString(outboundPayload)
        io.WriteString(rw, hexPayload)
        fmt.Printf("HTTP Reply payload: %s\n", hexPayload)
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
            // we supply json-formatted IPINFO to ttserve.  TTGATE tunneled this through
            // the GatewayEUI field of the DataUpAppReq, but in the TTN case we don't
            // have such a luxury.  That said, the DataUpAppReq for TTN DOES have the
            // Latitude/Longitude fields to work with.  Ideally we would then use
            // TinyGeocoder (or Yahoo or Google) *here* and would then encode the results
            // in the GatewayEUI field in a way that is compatible with TTGATE.
            // I haven't written this code simply because this requires registering
            // for a Yahoo/Google account, and potentially paying.  Since none of the
            // code here is actually utilizing geo, we'll wait until then.
            AppReq.Transport = "ttn:" + AppReq.TTN.DevEUI
            reqQ <- AppReq
            monitorReqQ()

            // See if there's an outbound message waiting for this app.  If so, send it now because we
            // know that there's a narrow receive window open.
            isAvailable, Payload := getOutboundPayload(AppReq.TTN.Payload)
            if isAvailable {
                ttnOutboundPublish(AppReq.TTN.DevEUI, Payload)
            }
        }

    }

}

// Get any outbound payload waiting for the node who sent us an AppReq
func getOutboundPayload(inboundPayload []byte) (isAvailable bool, outboundPayload []byte) {

    // Extract the telecast message from the AppReq
    msg := &teletype.Telecast{}
    err := proto.Unmarshal(inboundPayload, msg)
    if err != nil {
        return false, nil
    }

    // Ask telecast to retrieve any pending outbound message
    return TelecastOutboundPayload(msg)

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
        case teletype.Telecast_SIMPLECAST:
            metadata := AppReq.TTN.Metadata[0]
            ProcessSafecastMessage(msg, checksum, metadata.GatewayEUI, AppReq.Transport,
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
