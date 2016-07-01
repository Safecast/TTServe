/*
 * The Things Network to Safecast Message Publisher
 *
 * Contributors:
 *    Ray Ozzie
 */

package main

import (
    "os"
    "time"
    "sort"
    "bytes"
    "strings"
    "encoding/json"
    "io"
    "io/ioutil"
    "fmt"
    "net"
    "net/url"
    "net/http"
    "strconv"
    "github.com/golang/protobuf/proto"
    "github.com/rayozzie/teletype-proto/golang"
    MQTT "github.com/eclipse/paho.mqtt.golang"
)

//
// Constants
//

// Operational warning if devices aren't heard from in this period of time
const deviceWarningAfterMinutes = 30

// From "ttnctl applications", the AppEUI and its Access Key
const appEui string = "70B3D57ED0000420"
const appAccessKey string = "bgCzOOs/5K16cuwP3/sGP9sea/4jKFwFEdTbYHw2fRE="

// From https://staging.thethingsnetwork.org/wiki/Backend/Connect/Application
const ttnServer string = "tcp://staging.thethingsnetwork.org:1883"
const ttnTopic string = appEui + "/devices/+/up"

// Slack
const SlackOpsPostURL string = "https://hooks.slack.com/services/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"

// Safecast
const SafecastUploadURL = "http://107.161.164.163/scripts/indextest.php?api_key=%s"
const SafecastAppKey = "z3sHhgousVDDrCVXhzMT"

// This HTTP server
const ttServerPort string = ":8080"
const ttServer string = "http://api.teletype.io"
const ttServerURLSend string = "/send"
const ttServerURLGithub string = "/github"
const ttServerURLSlack string = "/slack"

// Other

const logDateFormat string = "2006-01-02 15:04:05"

//
// Structs
//

type IPInfoData struct {
    AS           string `json:"as"`
    City         string `json:"city"`
    Country      string `json:"country"`
    CountryCode  string `json:"countryCode"`
    ISP          string `json:"isp"`
    Latitude     float32 `json:"lat"`
    Longitude    float32 `json:"lon"`
    Organization string `json:"org"`
    IP           net.IP `json:"query"`
    Region       string `json:"region"`
    RegionName   string `json:"regionName"`
    Timezone     string `json:"timezone"`
    Zip          string `json:"zip"`
}

type SafecastData struct {
    CapturedAt   string `json:"captured_at,omitempty"`   // 2016-02-20T14:02:25Z
    ChannelID    string `json:"channel_id,omitempty"`    // nil
    DeviceID     string `json:"device_id,omitempty"`     // 140
    DeviceTypeID string `json:"devicetype_id,omitempty"` // nil
    Height       string `json:"height,omitempty"`        // 123
    ID           string `json:"id,omitempty"`            // 972298
    LocationName string `json:"location_name,omitempty"` // nil
    OriginalID   string `json:"original_id,omitempty"`   // 972298
    SensorID     string `json:"sensor_id,omitempty"`     // nil
    StationID    string `json:"station_id,omitempty"`    // nil
    Unit         string `json:"unit,omitempty"`          // cpm
    UserID       string `json:"user_id,omitempty"`       // 304
    Value        string `json:"value,omitempty"`         // 36
    Latitude     string `json:"latitude,omitempty"`      // 37.0105
    Longitude    string `json:"longitude,omitempty"`     // 140.9253
    BatVoltage   string `json:"bat_voltage,omitempty"`   // 0-N volts
    BatSOC       string `json:"bat_soc,omitempty"`       // 0%-100%
    WirelessSNR  string `json:"wireless_snr,omitempty"`  // -127db to +127db
    envTemp      string `json:"env_temp,omitempty"`      // Degrees centigrade
    envHumid     string `json:"env_humid,omitempty"`     // Percent RH
}

type seenDevice struct {
    originalDeviceNo uint32
    normalizedDeviceNo uint32
    seen time.Time
    notifiedAsUnseen bool
    minutesAgo int64
}

type dnDevice struct {
    devEui string
    topic string
}

type ByKey []seenDevice
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool {
    // Primary:
    // By capture time, most recent last (so that the most recent is nearest your attention, at the bottom in Slack)
    if a[i].seen.Before(a[j].seen) {
        return true
    } else if a[i].seen.After(a[j].seen) {
        return false
    }
    // Secondary
    // In an attempt to keep things reasonably deterministic, use device number
    if (a[i].normalizedDeviceNo < a[j].normalizedDeviceNo) {
        return true
    } else if (a[i].normalizedDeviceNo > a[j].normalizedDeviceNo) {
        return false
    }
    return false
}

//
// Statics
//

var fullyConnected bool = false;
var mqttClient MQTT.Client
var upQ chan MQTT.Message
var reqQ chan DataUpAppReq
var dnDevices []dnDevice
var seenDevices []seenDevice

func main() {

    // Set up our message queue.  We shouldn't need much buffering, but buffer just in case.
    upQ = make(chan MQTT.Message, 5)
    reqQ = make(chan DataUpAppReq, 5)

    // Spawn the app request handler shared by both TTN and direct inbound server
    go appRequestHandler()

    // Spawn the TTN inbound message handler
    go ttnInboundHandler()

    // Init our inbound server
    go ttInboundHandler()

    // Init our timer

    go timer1m()

    // Handle the inboound subscriber.  (This never returns.)
    ttnSubscriptionHandler()

}

func timer1m() {
    for {
        time.Sleep(1 * 60 * time.Second)
        checkForSeenDevices()
    }
}

func ttInboundHandler () {
    http.HandleFunc(ttServerURLSend, handleInboundTTNPosts)
    fmt.Printf("Now handling inbound on: %s%s%s\n", ttServer, ttServerPort, ttServerURLSend)
    http.HandleFunc(ttServerURLGithub, GithubHandler)
    fmt.Printf("Now handling inbound on: %s%s%s\n", ttServer, ttServerPort, ttServerURLGithub)
    http.HandleFunc(ttServerURLSlack, SlackHandler)
    fmt.Printf("Now handling inbound on: %s%s%s\n", ttServer, ttServerPort, ttServerURLSlack)
    http.ListenAndServe(ttServerPort, nil)
}

func handleInboundTTNPosts(rw http.ResponseWriter, req *http.Request) {
    io.WriteString(rw, "This is the teletype API endpoint.")

    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        fmt.Printf("Error reading HTTP request body: \n%v\n", req)
        return
    }

    var AppReq DataUpAppReq
    err = json.Unmarshal(body, &AppReq)
    if err != nil {
        // Very common case where anyone comes to web page, such as google health check
        return
    }

    fmt.Printf("\n%s Received %d-byte payload from TTGATE\n", time.Now().Format(logDateFormat), len(AppReq.Payload))

    // We now have a TTN-like message, constructed as follws:
    //  1) the Payload came from the device itself
    //  2) TTGATE filled in the lat/lon/alt metadata, just in case the payload doesn't have location
    //  3) TTGATE filled in SNR if it had access to it
    //  4) We'll add the server's time, in case the payload lacked CapturedAt
    AppReq.Metadata[0].ServerTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")

    // Enqueue it for TTN-like processing
    reqQ <- AppReq

}

func ttnSubscriptionHandler () {

    // This is the main retry loop for connecting to the service.  We've found empirically that
    // there are fields in the client options structure that are NOT just "options" but rather are
    // actual state, so we need to reallocate it on each retry.

    for {
        var token MQTT.Token
        var opts *MQTT.ClientOptions

        // Allocate and set up the options
        opts = MQTT.NewClientOptions()
        opts.AddBroker(ttnServer)
        opts.SetUsername(appEui)
        opts.SetPassword(appAccessKey)
        opts.SetAutoReconnect(true)
        opts.SetConnectionLostHandler(onConnectionLost)

        // Connect to the MQTT service

        mqttClient = MQTT.NewClient(opts)

        if token = mqttClient.Connect(); token.Wait() && token.Error() != nil {

            fmt.Printf("Error connecting to service: %s\n", token.Error())
            time.Sleep(15 * time.Second)

        } else {

            // Subscribe to the topic
            if token = mqttClient.Subscribe(ttnTopic, 0, onMessageReceived); token.Wait() && token.Error() != nil {
                fmt.Printf("Error subscribing to topic %s\n", ttnTopic, token.Error())
                return
            }

            // Signal that it's ok for the handlers to start processing inbound stuff
            fmt.Printf("Now handling inbound on: %s mqtt:%s\n", ttnServer, ttnTopic)
            fullyConnected = true;

        }

        // Infinitely loop here
        for fullyConnected {
            time.Sleep(15 * 60 * time.Second)
            fmt.Printf("%s Alive\n", time.Now().Format(time.RFC850))
        }

        fmt.Printf("Lost connection; reconnecting...\n")

    }
}

func ttnInboundHandler() {

    // Dequeue and process the messages as they're enqueued
    for msg := range upQ {
        var AppReq DataUpAppReq

        // Unmarshal the payload and extract the base64 data
        err := json.Unmarshal(msg.Payload(), &AppReq)
        if err != nil {
            fmt.Printf("*** Payload doesn't have TTN data ***\n")
        } else {
            fmt.Printf("\n%s Received %d-byte payload from TTN:\n", time.Now().Format(logDateFormat), len(AppReq.Payload))
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
        }

    }

}

func appRequestHandler() {

    // Dequeue and process the messages as they're enqueued
    for AppReq := range reqQ {

        msg := &teletype.Telecast{}
        err := proto.Unmarshal(AppReq.Payload, msg)
        if err != nil {
            fmt.Printf("*** PB unmarshaling error: ", err);
        } else {

            // Do various things baed upon the message type
            switch msg.GetDeviceType() {

                // Is it something we recognize as being from safecast?
            case teletype.Telecast_BGEIGIE_NANO:
                fallthrough
            case teletype.Telecast_SIMPLECAST:
                metadata := AppReq.Metadata[0]
                ProcessSafecastMessage(msg, metadata.GatewayEUI, metadata.ServerTime, metadata.Lsnr, metadata.Latitude, metadata.Longitude, metadata.Altitude)
                // Display what we got from a non-Safecast device
            default:
                ProcessTelecastMessage(msg, AppReq.DevEUI)
            }
        }
    }
}

func onConnectionLost(client MQTT.Client, err error) {
    fullyConnected = false
    mqttClient = nil
    fmt.Printf("OnConnectionLost: %v\n", err)
}

func onMessageReceived(client MQTT.Client, message MQTT.Message) {
    fmt.Printf("\n%s Message Received:\n", time.Now().Format(logDateFormat))
    upQ <- message
}

func ProcessTelecastMessage(msg *teletype.Telecast, devEui string) {
    message := msg.GetMessage()
    args := strings.Split(message, " ")
    arg0 := args[0]
    arg0LC := strings.ToLower(args[0])
    argRest := strings.Trim(strings.TrimPrefix(message, arg0), " ")

    switch arg0LC {
    case "/echo":
        fallthrough
    case "/hello":
        fallthrough
    case "/hi":
        fmt.Printf("/hello from %s\n", devEui)
        if (argRest == "") {
            sendMessage(devEui, "@ttserve: Hello.")
        } else {
            sendMessage(devEui, "@ttserve: "+argRest)
        }
    case "":
        fmt.Printf("Ping from %s\n", devEui)
    default:
        fmt.Printf("Broadcast from %s: 'message'\n", devEui, message)
        broadcastMessage(message, devEui)
    }

}

func ProcessSafecastMessage(msg *teletype.Telecast, ipInfo string, defaultTime string, defaultSNR float32, defaultLat float32, defaultLon float32, defaultAlt int32) {
    var theSNR float32

    // Process IPINFO data
    var info IPInfoData
    if ipInfo != "" {
        err := json.Unmarshal([]byte(ipInfo), &info)
        if (err != nil) {
            ipInfo = ""
        }
    }
    if (ipInfo != "") {
        fmt.Printf("Safecast message from %s/%s/%s:\n%s\n", info.City, info.Region, info.Country, msg)
    } else {
        fmt.Printf("Safecast message:\n%s\n", msg)
    }

    // Log it
    if (msg.DeviceIDNumber != nil) {
        updateDevice(msg.GetDeviceIDNumber())
    } else if msg.DeviceIDString != nil {
        i64, err := strconv.ParseInt(msg.GetDeviceIDString(), 10, 64)
        if (err == nil) {
            updateDevice(uint32(i64))
        }
    }

    // Determine if the device itself happens to be suppressing "slowly-changing" metadata during this upload.
    // If it is, we ourselves will use this as a hint not to spam the server with other slowly-changing data.
    deviceIsSuppressingMetadata := msg.BatteryVoltage == nil && msg.BatterySOC == nil && msg.EnvTemperature == nil &&  msg.EnvHumidity == nil

    // Generate the fields common to all uploads to safecast
    sc := SafecastData{}
    if msg.DeviceIDString != nil {
        sc.DeviceID = msg.GetDeviceIDString();
    } else if msg.DeviceIDNumber != nil {
        sc.DeviceID = strconv.FormatUint(uint64(msg.GetDeviceIDNumber()), 10);
    } else {
        sc.DeviceID = "UNKNOWN";
    }
    if msg.CapturedAt != nil {
        sc.CapturedAt = msg.GetCapturedAt()
    } else {
        sc.CapturedAt = defaultTime
    }

    // You would think that lat/lon/alt would be optional for all the uploads after
    // the first all-encompassing upload, but you'd be wrong.  Empirically I've found
    // that they are required fields for any record uploaded to the Safecast API.
    if msg.Latitude != nil {
        sc.Latitude = fmt.Sprintf("%f", msg.GetLatitude())
    } else {
        if (defaultLat != 0.0) {
            sc.Latitude = fmt.Sprintf("%f", defaultLat)
        }
    }
    if msg.Longitude != nil {
        sc.Longitude = fmt.Sprintf("%f", msg.GetLongitude())
    } else {
        if (defaultLon != 0.0) {
            sc.Longitude = fmt.Sprintf("%f", defaultLon)
        }
    }
    if msg.Altitude != nil {
        sc.Height = fmt.Sprintf("%d", msg.GetAltitude())
    } else {
        if (defaultAlt != 0.0) {
            sc.Height = fmt.Sprintf("%d", defaultAlt)
        }
    }

    // The first upload has everything
    sc1 := sc
    if msg.Unit == nil {
        sc1.Unit = "cpm"
    } else {
        sc1.Unit = fmt.Sprintf("%s", msg.GetUnit())
    }
    if msg.Value == nil {
        sc1.Value = ""
    } else {
        sc1.Value = fmt.Sprintf("%d", msg.GetValue())
    }
    if !deviceIsSuppressingMetadata {
        if msg.BatteryVoltage != nil {
            sc1.BatVoltage = fmt.Sprintf("%.4f", msg.GetBatteryVoltage())
        }
        if msg.BatterySOC != nil {
            sc1.BatSOC = fmt.Sprintf("%.2f", msg.GetBatterySOC())
        }
        if msg.EnvTemperature != nil {
            sc1.envTemp = fmt.Sprintf("%.2f", msg.GetEnvTemperature())
        }
        if msg.EnvHumidity != nil {
            sc1.envHumid = fmt.Sprintf("%.2f", msg.GetEnvHumidity())
        }
        if msg.WirelessSNR != nil {
            theSNR = msg.GetWirelessSNR()
        } else {
            theSNR = defaultSNR
        }
        if defaultSNR != 0.0 {
            sc1.WirelessSNR = fmt.Sprintf("%.1f", theSNR)
        }
    }
    uploadToSafecast(&sc1)

    // Due to Safecast API limitations, upload the metadata ase individual
    // web uploads.  Once this API limitation is removed, this code should
    // also be deleted.
    if !deviceIsSuppressingMetadata {
        if msg.BatteryVoltage != nil {
            sc2 := sc
            sc2.Unit = "bat_voltage"
            sc2.Value = sc1.BatVoltage
            uploadToSafecast(&sc2)
        }
        if msg.BatterySOC != nil {
            sc3 := sc
            sc3.Unit = "bat_soc"
            sc3.Value = sc1.BatSOC
            uploadToSafecast(&sc3)
        }
        if msg.EnvTemperature != nil {
            sc4 := sc
            sc4.Unit = "env_temp"
            sc4.Value = sc1.envTemp
            uploadToSafecast(&sc4)
        }
        if msg.EnvHumidity != nil {
            sc5 := sc
            sc5.Unit = "env_humid"
            sc5.Value = sc1.envHumid
            uploadToSafecast(&sc5)
        }
        if theSNR != 0.0 {
            sc6 := sc
            sc6.Unit = "wireless_snr"
            sc6.Value = sc1.WirelessSNR
            uploadToSafecast(&sc6)
        }
    }

}

// Upload the data structure to the Safecast service
func uploadToSafecast(sc *SafecastData) {

    scJSON, _ := json.Marshal(sc)
    fmt.Printf("About to upload to %s:\n%s\n", SafecastUploadURL, scJSON)
    req, err := http.NewRequest("POST", fmt.Sprintf(SafecastUploadURL, SafecastAppKey), bytes.NewBuffer(scJSON))
    req.Header.Set("User-Agent", "TTSERVE")
    req.Header.Set("Content-Type", "application/json")
    httpclient := &http.Client{}

    resp, err := httpclient.Do(req)
    if err != nil {
        fmt.Printf("*** Error uploading %s to Safecast %s\n\n", sc.Unit, err)
    } else {
        resp.Body.Close()
    }

}

// Get the downlink channel for a given devEui
func getDnTopic(devEui string) string {
    var e dnDevice

    for _, e = range dnDevices {
        if (devEui == e.devEui) {
            return e.topic
        }
    }

    e.devEui = devEui
    e.topic = appEui+"/devices/"+devEui+"/down"

    dnDevices = append(dnDevices, e)

    return e.topic

}

// Get the downlink channel for a given devEui
func sendMessage(devEui string, message string) {

    deviceType := teletype.Telecast_TTSERVE
    tmsg := &teletype.Telecast {}
    tmsg.DeviceType = &deviceType
    tmsg.Message = proto.String(message)
    tdata, terr := proto.Marshal(tmsg)
    if terr != nil {
        fmt.Printf("t marshaling error: ", terr)
    }

    jmsg := &DataDownAppReq {}
    jmsg.Payload = tdata
    jmsg.FPort = 1
    jmsg.TTL = "1h"
    jdata, jerr := json.Marshal(jmsg)
    if jerr != nil {
        fmt.Printf("j marshaling error: ", jerr)
    }

    fmt.Printf("Send %s: %s\n", getDnTopic(devEui), jdata)

    // Only do MQTT operations if we're fully initialized
    if (fullyConnected) {
        mqttClient.Publish(getDnTopic(devEui), 0, false, jdata)
    }

}

// Broadcast a message to all known devices except the one specified by 'skip'
func broadcastMessage(message string, skipDevEui string) {
    if (skipDevEui == "") {
        fmt.Printf("Broadcast '%s'\n", message)
    } else {
        fmt.Printf("Skipping %s, broadcast '%s'\n", skipDevEui, message)
    }
    for _, e := range dnDevices {
        if (e.devEui != skipDevEui) {
            sendMessage(e.devEui, message)
        }
    }
}

// Github webhook
func GithubHandler(rw http.ResponseWriter, req *http.Request) {
    var reason string

    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        fmt.Printf("Github webhook: error reading body:", err)
        return
    }
    var p PushPayload
    err = json.Unmarshal(body, &p)
    if err != nil {
        fmt.Printf("Github webhook: error unmarshaling body:", err)
        return
    }

    if (p.HeadCommit.Commit.Message != "m") {
        sendToSlack(fmt.Sprintf("** Restarting ** %s %s", p.HeadCommit.Commit.Committer.Name, p.HeadCommit.Commit.Message))
        reason = fmt.Sprintf("%s pushed %s's commit to GitHub: %s", p.Pusher.Name, p.HeadCommit.Commit.Committer.Name, p.HeadCommit.Commit.Message)
    } else {
        // Handle 'git commit -mm' and 'git commit -amm', used in dev intermediate builds, in a more aesthetically pleasing manner.
        reason = fmt.Sprintf("%s pushed %s's commit to GitHub", p.Pusher.Name, p.HeadCommit.Commit.Committer.Name)
    }
    fmt.Printf("\n***\n***\n*** RESTARTING because\n*** %s\n***\n***\n\n", reason)

    os.Exit(0)

}

// Slack webhook
func SlackHandler(rw http.ResponseWriter, req *http.Request) {
    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        fmt.Printf("Slack webhook: error reading body:", err)
        return
    }
    urlParams, err := url.ParseQuery(string(body))
    if err != nil {
        fmt.Printf("Slack webhook: error parsing body:", err)
        return
    }

    // Extract useful information
    user := urlParams["user_name"][0]
    message := urlParams["text"][0]
    args := strings.Split(message, " ")
    argsLC := strings.Split(strings.ToLower(message), " ")
    messageAfterFirstWord := strings.Join(args[1:], " ")

    // If this is from ourselves, bail.
    if (user == "slackbot") {
        return
    }

    // Process special queries

    switch (argsLC[0]) {
    case "status":
        if (messageAfterFirstWord == "") {
            doDeviceSummary()
        }
    case "hello":
        if len(args) == 1 {
            sendToSlack(fmt.Sprintf("Hello back, %s.", user))
        } else {
            sendToSlack(fmt.Sprintf("Back at you: %s", messageAfterFirstWord))
        }
    default:
        // Default is to do nothing
    }

}

func sendToSlack(msg string) {

    type SlackData struct {
        Message string `json:"text"`
    }

    m := SlackData{}
    m.Message = msg

    mJSON, _ := json.Marshal(m)
    req, err := http.NewRequest("POST", SlackOpsPostURL, bytes.NewBuffer(mJSON))
    req.Header.Set("User-Agent", "TTSERVE")
    req.Header.Set("Content-Type", "application/json")

    httpclient := &http.Client{}
    resp, err := httpclient.Do(req)
    if err != nil {
        fmt.Printf("*** Error uploading %s to Slack  %s\n\n", msg, err)
    } else {
        resp.Body.Close()
    }

}

func updateDevice(DeviceID uint32) {
    var dev seenDevice

    // For dual-sensor devices, collapse them to a single entry
    dev.originalDeviceNo = DeviceID
    dev.normalizedDeviceNo = dev.originalDeviceNo
    if ((dev.normalizedDeviceNo & 0x01) != 0) {
        dev.normalizedDeviceNo = dev.normalizedDeviceNo - 1
    }

    // Remember when we saw it

    // Attempt to update the existing entry
    found := false
    for i:=0; i<len(seenDevices); i++ {
        if dev.normalizedDeviceNo == seenDevices[i].normalizedDeviceNo {
            seenDevices[i].seen = time.Now().UTC()
            found = true
            break
        }
    }

    // Add a new array entry if necessary

    if (!found) {
        dev.seen = time.Now().UTC()
        dev.notifiedAsUnseen = false
        seenDevices = append(seenDevices, dev)
    }

}

// Update message ages and notify
func checkForSeenDevices() {
    expiration := time.Now().Add(-(time.Duration(deviceWarningAfterMinutes)*time.Minute))
    for i:=0; i<len(seenDevices); i++ {
        seenDevices[i].minutesAgo = int64(time.Now().Sub(seenDevices[i].seen)/time.Minute)
        if (!seenDevices[i].notifiedAsUnseen) {
            if seenDevices[i].seen.Before(expiration) {
                seenDevices[i].notifiedAsUnseen = true;
                sendToSlack(fmt.Sprintf("** Warning**  Device %d hasn't been seen for %d minutes!", seenDevices[i].originalDeviceNo, seenDevices[i].minutesAgo))
            }
        }
    }
}

// Get a summary of devices that are older than this many minutes ago
func doDeviceSummary() {

    // First, age out the expired devices and recompute when last seen
    checkForSeenDevices()

    // Next sort the device list
    sortedDevices := seenDevices
    sort.Sort(ByKey(sortedDevices))

    // Iterate over all devices
    devices := 0
    s := ""
    for i:=0; i<len(sortedDevices); i++ {
        devices++
        if (i > 0) {
            s = fmt.Sprintf("%s\n", s)
        }
        id := sortedDevices[i].originalDeviceNo

        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements|%010d>", s, id, id)

        s = fmt.Sprintf("%s (", s)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=bat_voltage|V>", s, id)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=bat_soc|%%>", s, id)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=env_temp|T>", s, id)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=env_humid|H>", s, id)
        s = fmt.Sprintf("%s<http://dev.safecast.org/en-US/devices/%d/measurements?order=captured_at+desc&unit=wireless_snr|S>", s, id)
        s = fmt.Sprintf("%s)", s)

        if (sortedDevices[i].minutesAgo == 0) {
            s = fmt.Sprintf("%s last seen just now", s)
        } else {
            s = fmt.Sprintf("%s last seen %dm ago", s, sortedDevices[i].minutesAgo)
        }
    }

    // Send it to Slack

    if (devices == 0) {
        sendToSlack("No devices yet.")
    } else {
        sendToSlack(s)
    }

}
