// Teletype Message Publishing Service
package main

import (
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

// This HTTP server-related
const ttServerPort string = ":8080"
const ttServerPortUDP string = ":8081"
const ttServerPortTCP string = ":8082"
const ttServerURLSend string = "/send"
const ttServerURLGithub string = "/github"
const ttServerURLSlack string = "/slack"

// Our server
var ttServer string

// Constants
const logDateFormat string = "2006-01-02 15:04:05"

// Statics
var everConnected bool = false
var fullyConnected bool = false
var mqttClient MQTT.Client
var lastConnected string = "(never)"
var lastDisconnected string = "(never)"
var upQ chan MQTT.Message
var reqQ chan DataUpAppReq

// Main entry point for app
func main() {

    // Get our external IP address
    ip, err := ipify.GetIp()
    if err != nil {
        ttServer = "http://api.teletype.io"
    } else {
        ttServer = "http://" + ip
    }

    // Set up our internal message queues
    upQ = make(chan MQTT.Message, 5)
    reqQ = make(chan DataUpAppReq, 5)

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
        sendSafecastCommsErrorsToSlack(15)
    }
}

// Kick off inbound messages coming from all sources, then serve HTTP
func webInboundHandler() {

    http.HandleFunc(ttServerURLSend, inboundWebTTGateHandler)
    fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", ttServer, ttServerPort, ttServerURLSend)

    http.HandleFunc(ttServerURLGithub, inboundWebGithubHandler)
    fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", ttServer, ttServerPort, ttServerURLGithub)

    http.HandleFunc(ttServerURLSlack, inboundWebSlackHandler)
    fmt.Printf("Now handling inbound HTTP on: %s%s%s\n", ttServer, ttServerPort, ttServerURLSlack)

    http.ListenAndServe(ttServerPort, nil)
}

// Kick off UDP server
func udpInboundHandler() {

    fmt.Printf("Now handling inbound UDP on: %s%s\n", ttServer, ttServerPortUDP)

    ServerAddr, err := net.ResolveUDPAddr("udp", ttServerPortUDP)
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
        } else {

            // Construct a TTN-like message
            //  1) the Payload comes from UDP
            //  2) We'll add the server's time, in case the payload lacked CapturedAt
            //  3) Everything else is null

            var AppReq DataUpAppReq
            AppReq.Payload = buf[0:n]
            AppReq.Metadata = make([]AppMetadata, 1)
            AppReq.Metadata[0].ServerTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")

            fmt.Printf("\n%s Received %d-byte UDP payload from %s\n", time.Now().Format(logDateFormat), len(AppReq.Payload), addr)

            // Enqueue it for TTN-like processing
            reqQ <- AppReq

        }
    }

}

// Kick off TCP server
func tcpInboundHandler() {

    fmt.Printf("Now handling inbound TCP on: %s%s\n", ttServer, ttServerPortTCP)

    ServerAddr, err := net.ResolveTCPAddr("tcp", ttServerPortTCP)
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
            continue
        }

        n, err := conn.Read(buf)
        if err != nil {
            fmt.Printf("TCP read error: \n%v\n", err)
            ServerConn.Close()
            continue
        }

        // Construct a TTN-like message
        //  1) the Payload comes from UDP
        //  2) We'll add the server's time, in case the payload lacked CapturedAt
        //  3) Everything else is null
        var AppReq DataUpAppReq
        AppReq.Payload = buf[0:n]
        AppReq.Metadata = make([]AppMetadata, 1)
        AppReq.Metadata[0].ServerTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")

        // Test the received data to see if it's just some random port scanner trying to probe what we are
        msg := &teletype.Telecast{}
        err = proto.Unmarshal(AppReq.Payload, msg)
        if (err != nil) {

            fmt.Printf("\n%s Ignoring %d-byte TCP port scan probe from %s\n", time.Now().Format(logDateFormat), len(AppReq.Payload), conn.RemoteAddr().String())

        } else {

            fmt.Printf("\n%s Received %d-byte TCP payload from %s\n", time.Now().Format(logDateFormat), len(AppReq.Payload), conn.RemoteAddr().String())

            // Enqueue it for TTN-like processing
            reqQ <- AppReq

            // Delay to see if we can pick up a reply for this request.  This is certainly
            // controversial because it slows down the incoming message processing, however there
            // is a trivial fix:  Create many instances of this goroutine on the service instead
            // of just one.
            time.Sleep(1 * time.Second)

            // See if there's an outbound message waiting for this app.  If so, send it now because we
            // know that there's a narrow receive window open.
            isAvailable, outboundPayload := getOutboundPayload(AppReq.Payload)
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
    var AppReq DataUpAppReq

    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        fmt.Printf("Error reading HTTP request body: \n%v\n", req)
        return
    }

    switch (req.UserAgent()) {

    case "TTGATE": {

        err = json.Unmarshal(body, &AppReq)
        if err != nil {
            io.WriteString(rw, "This is the teletype API endpoint.")
            // Very common case where anyone comes to web page, such as google health check
            return
        }

        fmt.Printf("\n%s Received %d-byte HTTP payload from TTGATE\n", time.Now().Format(logDateFormat), len(AppReq.Payload))

        // We now have a TTN-like message, constructed as follows:
        //  1) the Payload came from the device itself
        //  2) TTGATE filled in the lat/lon/alt metadata, just in case the payload doesn't have location
        //  3) TTGATE filled in SNR if it had access to it
        //  4) We'll add the server's time, in case the payload lacked CapturedAt
        AppReq.Metadata[0].ServerTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")

    }

    case "TTRELAY": {

        inboundPayload, err := hex.DecodeString(string(body))
        if err != nil {
            fmt.Printf("Hex decoding error: ", err)
            return
        }

        AppReq.Payload = inboundPayload
        fmt.Printf("\n%s Received %d-byte HTTP payload from DEVICE\n", time.Now().Format(logDateFormat), len(AppReq.Payload))

        // We now have a TTN-like message, constructed as follws:
        //  1) the Payload came from the device itself
        //  2) We'll add the server's time, in case the payload lacked CapturedAt
        AppReq.Metadata = make([]AppMetadata, 1)
        AppReq.Metadata[0].ServerTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")

    }

    default: {

        // A web crawler, etc.
        return

    }

    }

    // Enqueue AppReq for TTN-like processing
    reqQ <- AppReq

    // Delay to see if we can pick up a reply for this request.  This is certainly
    // controversial because it slows down the incoming message processing, however there
    // is a trivial fix:  Create many instances of this goroutine on the service instead
    // of just one.
    time.Sleep(1 * time.Second)

    // See if there's an outbound message waiting for this app.  If so, send it now because we
    // know that there's a narrow receive window open.
    isAvailable, outboundPayload := getOutboundPayload(AppReq.Payload)
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
            fullyConnected = false
            lastDisconnected = time.Now().Format(logDateFormat)
            fmt.Printf("\n%s *** TTN Connection Lost: %v\n\n", time.Now().Format(logDateFormat), err)
            sendToSafecastOps(fmt.Sprintf("connection lost: %v\n", err))
            sendToTTNOps(fmt.Sprintf("Connection lost from this server to %s: %v\n", ttnServer, err))
        }
        mqttOpts.SetConnectionLostHandler(onMqConnectionLost)

        // The "connect" handler subscribes to the topic, and subscribes with a receiver callback
        onMqConnectionMade := func (client MQTT.Client) {

            // Function to process received messages
            onMqMessageReceived := func(client MQTT.Client, message MQTT.Message) {
                upQ <- message
            }

            // Subscribe to the upstream topic
            if token := client.Subscribe(ttnTopic, 0, onMqMessageReceived); token.Wait() && token.Error() != nil {
                // Treat subscription failure as a connection failure
                fmt.Printf("Error subscribing to topic %s\n", ttnTopic, token.Error())
                fullyConnected = false
            } else {
                // Successful subscription
                fullyConnected = true
                lastConnected = time.Now().Format(logDateFormat)
                if (everConnected) {
                    sendToSafecastOps("TTN connection restored")
                    sendToTTNOps(fmt.Sprintf("Connection restored from this server to %s\n", ttnServer))
                    fmt.Printf("\n%s *** TTN Connection Restored\n\n", time.Now().Format(logDateFormat))
                } else {
                    everConnected = true
                    fmt.Printf("TTN Connection Established\n")
                }
            }

        }
        mqttOpts.SetOnConnectHandler(onMqConnectionMade)

        // Create the client session context, saving it
        // in a global so that it may also be used to Publish
        mqttClient = MQTT.NewClient(mqttOpts)

        // Connect to the service
        if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
            fmt.Printf("Error connecting to service: %s\n", token.Error())
        } else {

            fmt.Printf("Now handling inbound MQTT on: %s mqtt:%s\n", ttnServer, ttnTopic)
            for consecutiveFailures := 0; consecutiveFailures < 3; {
                time.Sleep(60 * time.Second)
                if fullyConnected {
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
        mqttClient = nil
        time.Sleep(5 * time.Second)
        fmt.Printf("\n***\n")
        fmt.Printf("*** Last time connection was successfully made: %s\n", lastConnected)
        fmt.Printf("*** Last time connection was lost: %s\n", lastDisconnected)
        fmt.Printf("*** Now attempting to reconnect: %s\n", time.Now().Format(logDateFormat))
        fmt.Printf("***\n\n")

    }
}

// Send to a ttn device outbound
func ttnOutboundPublish(devEui string, payload []byte) {
    if fullyConnected {
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
        mqttClient.Publish(topic, 0, false, jdata)
    }
}

// Handle inbound pulled from TTN's upstream mqtt message queue
func ttnInboundHandler() {

    // Dequeue and process the messages as they're enqueued
    for msg := range upQ {
        var AppReq DataUpAppReq

        // Unmarshal the payload and extract the base64 data
        err := json.Unmarshal(msg.Payload(), &AppReq)
        if err != nil {
            fmt.Printf("*** Payload doesn't have TTN data ***\n")
        } else {
            fmt.Printf("\n%s Received %d-byte payload from TTN\n", time.Now().Format(logDateFormat), len(AppReq.Payload))
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
            reqQ <- AppReq

            // See if there's an outbound message waiting for this app.  If so, send it now because we
            // know that there's a narrow receive window open.
            isAvailable, Payload := getOutboundPayload(AppReq.Payload)
            if isAvailable {
                ttnOutboundPublish(AppReq.DevEUI, Payload)
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
        err := proto.Unmarshal(AppReq.Payload, msg)
        if err != nil {
            fmt.Printf("*** PB unmarshaling error: \n", err)
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
            metadata := AppReq.Metadata[0]
            ProcessSafecastMessage(msg, checksum, metadata.GatewayEUI,
                metadata.ServerTime,
                metadata.Lsnr,
                metadata.Latitude, metadata.Longitude, metadata.Altitude)

            // Handle messages from non-safecast devices
        default:
            ProcessTelecastMessage(msg, AppReq.DevEUI)
        }
    }
}
