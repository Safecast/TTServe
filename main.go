/*
 * The Things Network to Safecast Message Publisher
 *
 * Contributors:
 *    Ray Ozzie
 */

package main

import (
    "bytes"
    "strings"
    "encoding/json"
    "fmt"
    MQTT "github.com/eclipse/paho.mqtt.golang"
    "net/http"
    "strconv"
    "github.com/golang/protobuf/proto"
    "github.com/rayozzie/teletype-proto/golang"
)

var appEui string
var appAccessKey string

var mqttClient MQTT.Client
var upQ chan MQTT.Message

type dnDevice struct {
    devEui string
    topic string
}

var dnDevices []dnDevice

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

    ourServerName := "Teletype Service"

    // From https://staging.thethingsnetwork.org/wiki/Backend/Connect/Application
    ttnServer := "tcp://staging.thethingsnetwork.org:1883"

    // From "ttnctl applications", the AppEUI and its Access Key
    appEui = "70B3D57ED0000420"
    appAccessKey = "bgCzOOs/5K16cuwP3/sGP9sea/4jKFwFEdTbYHw2fRE="

    // Set up our message queue.  We shouldn't need much buffering, but buffer just in case.
    upQ = make(chan MQTT.Message, 5)

    // Set up connection options

    type MqttConsumer struct {
        client *MQTT.Client
    }

    opts := MQTT.NewClientOptions()
    opts.AddBroker(ttnServer)
    opts.SetClientID(ourServerName)
    opts.SetUsername(appEui)
    opts.SetPassword(appAccessKey)

    // Connect to the MQTT service

    mqttClient = MQTT.NewClient(opts)
    if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
        fmt.Printf("Error connecting to service: %s\n", token.Error())
        return
    }

    fmt.Printf("Connected to %s\n", ttnServer)

    // Subscribe to the topics

    if token := mqttClient.Subscribe(appEui+"/devices/+/up", 1, onMessageReceived); token.Wait() && token.Error() != nil {
        fmt.Printf("Error subscribing to topic: %s\n", token.Error())
        return
    }

    fmt.Printf("Subscribed to uplinked message queue\n")

    // Dequeue and process the messages as they're enqueued
    for msg := range upQ {
        var AppReq DataUpAppReq
        //        fmt.Printf("Received from topic %s:\n%s\n", msg.Topic(), msg.Payload())

        // Unmarshal the payload and extract the base64 data
        err := json.Unmarshal(msg.Payload(), &AppReq)
        if err != nil {
            fmt.Printf("*** Payload doesn't have TTN data ***\n")
        } else {
            devEui := AppReq.DevEUI;
            fmt.Printf("Received message from Device: %s\n", devEui)
            metadata := AppReq.Metadata[0]
            //            fmt.Printf("Unmarshaled as:\n%v\n", AppReq)

            // Unmarshal the buffer into a golang object
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
                    ProcessSafecastMessage(msg, metadata.ServerTime, metadata.Latitude, metadata.Longitude, metadata.Altitude)
                    // Display what we got from a non-Safecast device
                default:
                    ProcessTelecastMessage(msg, devEui)
                }
            }

        }

    }

}

func onMessageReceived(client MQTT.Client, message MQTT.Message) {
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

func ProcessSafecastMessage(msg *teletype.Telecast, defaultTime string, defaultLat float32, defaultLon float32, defaultAlt int32) {

    // Debug
    fmt.Printf("Received Safecast Message:\n")
    fmt.Printf("%s\n", msg)

    // Generate the fields common to all uploads to safecast
    sc := &SafecastData{}
    if (msg.DeviceIDString != nil) {
        sc.DeviceID = msg.GetDeviceIDString();
    } else if (msg.DeviceIDNumber != nil) {
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
    if (msg.Unit == nil) {
        sc1.Unit = "cpm"
    } else {
        sc1.Unit = fmt.Sprintf("%s", msg.GetUnit())
    }
    if (msg.Value == nil) {
        sc1.Value = "?"
    } else {
        sc1.Value = fmt.Sprintf("%d", msg.GetValue())
    }
    if (msg.BatteryVoltage != nil) {
        sc1.BatVoltage = fmt.Sprintf("%.4f", msg.GetBatteryVoltage())
    }
    if (msg.BatterySOC != nil) {
        sc1.BatSOC = fmt.Sprintf("%.2f", msg.GetBatterySOC())
    }
    if (msg.WirelessSNR != nil) {
        sc1.WirelessSNR = fmt.Sprintf("%.1f", msg.GetWirelessSNR())
    }
    if (msg.EnvTemperature != nil) {
        sc1.envTemp = fmt.Sprintf("%.2f", msg.GetEnvTemperature())
    }
    if (msg.EnvHumidity != nil) {
        sc1.envHumid = fmt.Sprintf("%.2f", msg.GetEnvHumidity())
    }
	uploadToSafecast(sc1)

	// The following uploads have individual values
	
    if (msg.BatteryVoltage != nil) {
        sc2 := sc
        sc2.Unit = "bat_voltage"
        sc2.Value = sc1.BatVoltage
		uploadToSafecast(sc2)
    }

    if (msg.BatterySOC != nil) {
        sc3 := sc
        sc3.Unit = "bat_soc"
        sc3.Value = sc1.BatSOC
		uploadToSafecast(sc3)
    }

    if (msg.WirelessSNR != nil) {
        sc4 := sc
        sc4.Unit = "wireless_snr"
        sc4.Value = sc1.WirelessSNR
		uploadToSafecast(sc4)
    }

    if (msg.EnvTemperature != nil) {
        sc5 := sc
        sc5.Unit = "env_temp"
        sc5.Value = sc1.envTemp
		uploadToSafecast(sc5)
    }

    if (msg.EnvHumidity != nil) {
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
    mqttClient.Publish(getDnTopic(devEui), 0, false, jdata)

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
