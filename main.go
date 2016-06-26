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


// From https://staging.thethingsnetwork.org/wiki/Backend/Connect/Application
var ttnServer string = "tcp://staging.thethingsnetwork.org:1883"
// From "ttnctl applications", the AppEUI and its Access Key
var appEui string = "70B3D57ED0000420"
var appAccessKey string = "bgCzOOs/5K16cuwP3/sGP9sea/4jKFwFEdTbYHw2fRE="

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
	IP           net.IP `json:"ip"`
	HostName     string `json:"hostname"`
	City         string `json:"city"`
	Region       string `json:"region"`
	Country      string `json:"country"`
	Location     string `json:"loc"`
	Organization string `json:"org"`
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

    // Spawn the TTN inbound message handler
    go ttnInboundHandler()

    // Spawn the app request handler shared by both TTN and direct inbound server
    go appRequestHandler()

    // Init our inbound server
    go ttnDirectServer()

    // Handle the inboound subscriber.  (This never returns.)
    ttnSubscriptionHandler()

}

func ttnDirectServer () {
    http.HandleFunc("/", handleInboundTTNPosts)
    http.ListenAndServe(":8080", nil)
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

            fmt.Printf("Connected to %s\n", ttnServer)

            // Subscribe to the topic
            topic := appEui+"/devices/+/up"
            if token = mqttClient.Subscribe(topic, 0, onMessageReceived); token.Wait() && token.Error() != nil {
                fmt.Printf("Error subscribing to topic %s\n", topic, token.Error())
                return
            }

            // Signal that it's ok for the handlers to start processing inbound stuff
            fmt.Printf("Subscribed to %s and now fully connected.\n", topic)
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

        fmt.Printf("Received from topic %s:\n%s\n", msg.Topic(), msg.Payload())

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

        devEui := AppReq.DevEUI;
        fmt.Printf("\n%s Received message from Device: %s\n", time.Now().Format(logDateFormat), devEui)

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
                ProcessTelecastMessage(msg, devEui)
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
        if (argRest == "") {
            sendMessage(devEui, "@ttserve: Hello.")
        } else {
            sendMessage(devEui, "@ttserve: "+argRest)
        }
    case "":
        // Do nothing, because this is just an intentional "ping" to the server
    default:
        broadcastMessage(message, devEui)
    }

}

func ProcessSafecastMessage(msg *teletype.Telecast, ipInfo string, defaultTime string, snr float32, defaultLat float32, defaultLon float32, defaultAlt int32) {
	var theSNR float32
	var sentVoltage bool

	// Process IPINFO data
	var info IPInfoData
	if ipInfo != "" {
		json.Unmarshal([]byte(ipInfo), &info)
	}
	
    // Debug
    fmt.Printf("Safecast message from %s/%s/%s:\n%s\n", info.City, info.Region, info.Country, msg)
	
    // Generate the fields common to all uploads to safecast
    sc := &SafecastData{}
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
    if msg.Latitude != nil {
        sc.Latitude = fmt.Sprintf("%f", msg.GetLatitude())
    } else {
        sc.Latitude = fmt.Sprintf("%f", defaultLat)
    }
    if msg.Longitude != nil {
        sc.Longitude = fmt.Sprintf("%f", msg.GetLongitude())
    } else {
        sc.Longitude = fmt.Sprintf("%f", defaultLon)
    }
    if msg.Altitude != nil {
        sc.Height = fmt.Sprintf("%d", msg.GetAltitude())
    } else {
        sc.Height = fmt.Sprintf("%d", defaultAlt)
    }

    // The first upload has everything
    sc1 := sc
    if msg.Unit == nil {
        sc1.Unit = "cpm"
    } else {
        sc1.Unit = fmt.Sprintf("%s", msg.GetUnit())
    }
    if msg.Value == nil {
        sc1.Value = "?"
    } else {
        sc1.Value = fmt.Sprintf("%d", msg.GetValue())
    }
    if msg.BatteryVoltage != nil {
        sc1.BatVoltage = fmt.Sprintf("%.4f", msg.GetBatteryVoltage())
    }
    if msg.BatterySOC != nil {
        sc1.BatSOC = fmt.Sprintf("%.2f", msg.GetBatterySOC())
    }

    if msg.WirelessSNR != nil {
		theSNR = msg.GetWirelessSNR()
    } else {
		// this could be 0.0 if it weren't present in the message,
		// and so a check for theSNR == 0 will be our "is present" test.
		theSNR = snr
	}
    sc1.WirelessSNR = fmt.Sprintf("%.1f", theSNR)

    if msg.EnvTemperature != nil {
        sc1.envTemp = fmt.Sprintf("%.2f", msg.GetEnvTemperature())
    }
    if msg.EnvHumidity != nil {
        sc1.envHumid = fmt.Sprintf("%.2f", msg.GetEnvHumidity())
    }
    uploadToSafecast(sc1)

    // The following uploads have individual values

    if msg.BatteryVoltage != nil {
        sc2 := sc
        sc2.Unit = "bat_voltage"
        sc2.Value = sc1.BatVoltage
        uploadToSafecast(sc2)
		sentVoltage = true
    } else {
		sentVoltage = false
	}

    if msg.BatterySOC != nil {
        sc3 := sc
        sc3.Unit = "bat_soc"
        sc3.Value = sc1.BatSOC
        uploadToSafecast(sc3)
    }

	// Note that we suppress this to only happen when we also send the voltage.
	// We do this because this is a very slowly-changing value, and since it is
	// gateway-supplied it is unthrottled and present on every entry.
	// Since the voltage is device-throttled (for the same reason of being too
	// noisy), we use the fact that the voltage was uploaded as a way
	// of throttling when we also upload the SNR.
    if theSNR != 0  && sentVoltage {
        sc4 := sc
        sc4.Unit = "wireless_snr"
        sc4.Value = sc1.WirelessSNR
        uploadToSafecast(sc4)
    }

    if msg.EnvTemperature != nil {
        sc5 := sc
        sc5.Unit = "env_temp"
        sc5.Value = sc1.envTemp
        uploadToSafecast(sc5)
    }

    if msg.EnvHumidity != nil {
        sc6 := sc
        sc6.Unit = "env_humid"
        sc6.Value = sc1.envHumid
        uploadToSafecast(sc6)
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
