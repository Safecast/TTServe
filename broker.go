// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Outbound MQTT support for publishing to AWS IOT MQTT broker
package main

import (
    "fmt"
    MQTT "github.com/eclipse/paho.mqtt.golang"
    "encoding/json"
)

var brokerConnected bool
var brokerMqttClient MQTT.Client

func brokerConnect() (err error) {

	if brokerConnected {
		return;
	}

    mqttOpts := MQTT.NewClientOptions()
    mqttOpts.AddBroker(ServiceConfig.BrokerHost)
    mqttOpts.SetUsername(ServiceConfig.BrokerUsername)
    mqttOpts.SetPassword(ServiceConfig.BrokerPassword)

    mqttOpts.SetAutoReconnect(true)
    mqttOpts.SetCleanSession(true)

    onMqConnectionLost := func (client MQTT.Client, err error) {
        fmt.Printf("\n%s *** AWS IoT MQQT broker connection lost: %v\n\n", LogTime(), err)
    }
    mqttOpts.SetConnectionLostHandler(onMqConnectionLost)

    brokerMqttClient = MQTT.NewClient(mqttOpts)

        // Connect to the service
    if token := brokerMqttClient.Connect(); token.Wait() && token.Error() != nil {
        fmt.Printf("Error connecting to broker: %s\n", token.Error())
    } else {
		brokerConnected = true
	}

	return
	
}

// Send to anyone/everyone listening on that MQTT topic
func brokerPublish(sd SafecastData) {
//	return;
	
	// Init
	if !brokerConnected {
		brokerConnect()
		if !brokerConnected {
			return;
		}
	}

    // Marshal the safecast data to json
    scJSON, _ := json.Marshal(sd)
    topic := "all"
    if token := brokerMqttClient.Publish(topic, 0, false, scJSON); token.Wait() && token.Error() != nil {
		fmt.Printf("broker: %s\n", token.Error())
	}

}
