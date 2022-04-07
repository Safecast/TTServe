// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Outbound MQTT support for publishing to AWS IOT MQTT broker
package main

import (
	"encoding/json"
	"fmt"

	ttdata "github.com/Safecast/safecast-go"
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

var brokerConnected bool
var brokerMqttClient MQTT.Client

func brokerOutboundPublisher() {

	mqttOpts := MQTT.NewClientOptions()
	mqttOpts.AddBroker(ServiceConfig.BrokerHost)
	mqttOpts.SetUsername(ServiceConfig.BrokerUsername)
	mqttOpts.SetPassword(ServiceConfig.BrokerPassword)

	mqttOpts.SetAutoReconnect(true)
	mqttOpts.SetCleanSession(true)

	onMqConnectionLost := func(client MQTT.Client, err error) {
		fmt.Printf("\n%s *** MQTT broker connection lost: %s: %v\n\n", LogTime(), ServiceConfig.BrokerHost, err)
	}
	mqttOpts.SetConnectionLostHandler(onMqConnectionLost)

	brokerMqttClient = MQTT.NewClient(mqttOpts)

	// Connect to the service
	if token := brokerMqttClient.Connect(); token.Wait() && token.Error() != nil {
		fmt.Printf("Error connecting to broker: %s\n", token.Error())
	} else {
		fmt.Printf("Broker: connected\n")
		brokerConnected = true
	}

}

// Send to anyone/everyone listening on that MQTT topic
func brokerPublish(sd ttdata.SafecastData) {

	// Init
	if !brokerConnected {
		return
	}

	// We don't publish anything without a captured date, because it confuses too many systems
	if sd.CapturedAt == nil || *sd.CapturedAt == "" {
		return
	}

	// Delete the legacy device ID so that it doesn't confuse anyone.  It has been superceded
	// by the device URN
	sd.DeviceID = 0

	// Marshal the safecast data to json
	scJSON, _ := json.Marshal(sd)
	topic := fmt.Sprintf("device/%s", sd.DeviceUID)
	if token := brokerMqttClient.Publish(topic, 0, false, scJSON); token.Wait() && token.Error() != nil {
		fmt.Printf("broker: %s\n", token.Error())
	}

}
