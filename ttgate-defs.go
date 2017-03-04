// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

type TTGateReq struct {

	// Message info
	Payload				[]byte		`json:"payload,omitempty"`

	// Message-related info generated by the gateway
	Snr					float32		`json:"gateway_lora_snr,omitempty"`
	ReceivedAt			string		`json:"gateway_received,omitempty"`

	// Gateway info
	Longitude			float32		`json:"gateway_longitude,omitempty"`
	Latitude			float32		`json:"gateway_latitude,omitempty"`
	Altitude			int32		`json:"gateway_altitude,omitempty"`
	Location			string		`json:"gateway_location,omitempty"`
	GatewayId			string		`json:"gateway_id,omitempty"`
	GatewayName			string		`json:"gateway_name,omitempty"`
	MessagesReceived	uint32		`json:"gateway_msgs_received,omitempty"`
	DevicesSeen			string		`json:"gateway_devices,omitempty"`
	IPInfo				IPInfoData	`json:"gateway_location,omitempty"`

	// Service Info, when this message is being routed service-to-service
	Transport			string		`json:"service_transport,omitempty"`

}
