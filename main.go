/*
 * The Things Network to Safecast Message Publisher
 *
 * Contributors:
 *    Ray Ozzie
 */

package main

import (
    "time"
    "bytes"
    "strings"
    "encoding/json"
    "io"
    "io/ioutil"
    "fmt"
    MQTT "github.com/eclipse/paho.mqtt.golang"
    "net"
    "net/http"
    "strconv"
    "github.com/golang/protobuf/proto"
    "github.com/rayozzie/teletype-proto/golang"
)


// From "ttnctl applications", the AppEUI and its Access Key
var appEui string = "70B3D57ED0000420"
var appAccessKey string = "bgCzOOs/5K16cuwP3/sGP9sea/4jKFwFEdTbYHw2fRE="
// From https://staging.thethingsnetwork.org/wiki/Backend/Connect/Application
var ttnServer string = "tcp://staging.thethingsnetwork.org:1883"
var ttnTopic string = appEui + "/devices/+/up"

// Our HTTP server
var ttServerPort string = ":8080"
var ttServer string = "http://api.teletype.io"
var ttServerURLSend string = "/send"
var ttServerURLGithub string = "/github"

var fullyConnected bool = false;
var logDateFormat string = "2006-01-02 15:04:05"

var mqttClient MQTT.Client
var upQ chan MQTT.Message
var reqQ chan DataUpAppReq

type dnDevice struct {
    devEui string
    topic string
}

var dnDevices []dnDevice

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

    // Handle the inboound subscriber.  (This never returns.)
    ttnSubscriptionHandler()

}

func ttInboundHandler () {
    http.HandleFunc(ttServerURLSend, handleInboundTTNPosts)
    fmt.Printf("Now handling inbound on: %s%s%s\n", ttServer, ttServerPort, ttServerURLSend)
    http.HandleFunc(ttServerURLGithub, GithubHandler)
    fmt.Printf("Now handling inbound on: %s%s%s\n", ttServer, ttServerPort, ttServerURLGithub)
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

        fmt.Printf("\n%s Received %d-byte payload from TTN:\n", time.Now().Format(logDateFormat), len(msg.Payload()))

        // Unmarshal the payload and extract the base64 data
        err := json.Unmarshal(msg.Payload(), &AppReq)
        if err != nil {
            fmt.Printf("*** Payload doesn't have TTN data ***\n")
        } else {
            // Enqueue it for processing
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
        json.Unmarshal([]byte(ipInfo), &info)
    }

    fmt.Printf("Safecast message from %s/%s/%s:\n%s\n", info.City, info.Region, info.Country, msg)

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
    if msg.Latitude != nil {
        sc1.Latitude = fmt.Sprintf("%f", msg.GetLatitude())
    } else {
		if (defaultLat != 0.0) {
	        sc1.Latitude = fmt.Sprintf("%f", defaultLat)
		}
    }
    if msg.Longitude != nil {
        sc1.Longitude = fmt.Sprintf("%f", msg.GetLongitude())
    } else {
		if (defaultLon != 0.0) {
	        sc1.Longitude = fmt.Sprintf("%f", defaultLon)
		}
    }
    if msg.Altitude != nil {
        sc1.Height = fmt.Sprintf("%d", msg.GetAltitude())
    } else {
		if (defaultAlt != 0.0) {
	        sc1.Height = fmt.Sprintf("%d", defaultAlt)
		}
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

    SafecastUploadURL := "http://107.161.164.163/scripts/indextest.php?api_key=%s"
    SafecastAppKey := "z3sHhgousVDDrCVXhzMT"
    UploadURL := fmt.Sprintf(SafecastUploadURL, SafecastAppKey)

    scJSON, _ := json.Marshal(sc)

    fmt.Printf("About to upload to %s:\n%s\n", SafecastUploadURL, scJSON)
    req, err := http.NewRequest("POST", UploadURL, bytes.NewBuffer(scJSON))
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
type test_struct struct {
    Test string
}

func GithubHandler(rw http.ResponseWriter, req *http.Request) {
    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        fmt.Printf("Github webhook: error reading body:", err)
        return
    }
    fmt.Printf("\nReceived Github notification on HTTP\nBody:\n%s\n", string(body))
    var t test_struct
    err = json.Unmarshal(body, &t)
    if err != nil {
        fmt.Printf("Github webhook: error unmarshaling body:", err)
        return
    }
    fmt.Printf("Github Unmarshaled:\n%s\n", string(t.Test))
}
