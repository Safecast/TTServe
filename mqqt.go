// Inbound MQQT support
package main

import (
	"time"
    "fmt"
    MQTT "github.com/eclipse/paho.mqtt.golang"
    "encoding/json"
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

// Handle inbound pulled from TTN's upstream mqtt message queue
func MqqtInboundHandler() {

    // Set up our internal message queues
    ttnUpQ = make(chan MQTT.Message, 5)

	// Now that the queue is created, monitor it
    go MqqtSubscriptionMonitor()

    // Dequeue and process the messages as they're enqueued
    for msg := range ttnUpQ {
        var ttn UplinkMessage
        var AppReq IncomingAppReq

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
            AppReq.UploadedAt = nowInUTC()
			AppReqPush(AppReq)
            CountTTN++

            // See if there's an outbound message waiting for this app.  If so, send it now because we
            // know that there's a narrow receive window open.
            isAvailable, deviceID := getReplyDeviceIDFromPayload(AppReq.Payload)
            if isAvailable {
                isAvailable, payload := TelecastOutboundPayload(deviceID)
                if (isAvailable) {
                    ttnOutboundPublish(AppReq.TTNDevID, payload)
                    sendToSafecastOps(fmt.Sprintf("Device %d picked up its pending command\n", deviceID), SLACK_MSG_UNSOLICITED)
                }
            }
        }

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

// Notify Slack if there is an outage
func MqqtSubscriptionNotifier() {
    if (ttnEverConnected) {
        if (!ttnFullyConnected) {
            minutesOffline := int64(time.Now().Sub(ttnLastDisconnectedTime) / time.Minute)
            if (minutesOffline > 15) {
                sendToSafecastOps(fmt.Sprintf("TTN has been unavailable for %d minutes (outage began at %s UTC)", minutesOffline, ttnLastDisconnected), SLACK_MSG_UNSOLICITED)
            }
        } else {
            if (ttnOutages > 1) {
                sendToSafecastOps(fmt.Sprintf("TTN has had %d brief outages in the past 15m", ttnOutages), SLACK_MSG_UNSOLICITED)
                ttnOutages = 0;
            }
        }
    }
}

// Subscribe to TTN inbound messages, then monitor connection status
func MqqtSubscriptionMonitor() {

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
                        sendToSafecastOps(fmt.Sprintf("TTN returned (%d-minute outage began at %s UTC)", minutesOffline, ttnLastDisconnected), SLACK_MSG_UNSOLICITED)
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
