// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license found with the
// source code from where this was derived:
// https://github.com/TheThingsNetwork/ttn/core/types
package main

import (
	"time"
)

// DataUpAppReq represents the actual payloads sent to application on uplink
// UplinkMessage represents an application-layer uplink message
type UplinkMessage struct {
	AppID          string                 `json:"app_id,omitempty"`
	DevID          string                 `json:"dev_id,omitempty"`
	HardwareSerial string                 `json:"hardware_serial,omitempty"`
	FPort          uint8                  `json:"port"`
	FCnt           uint32                 `json:"counter"`
	IsRetry        bool                   `json:"is_retry,omitempty"`
	PayloadRaw     []byte                 `json:"payload_raw"`
	PayloadFields  map[string]interface{} `json:"payload_fields,omitempty"`
	Metadata       Metadata               `json:"metadata,omitempty"`
}

// Metadata contains metadata of a message
type Metadata struct {
	Time       JSONTime          `json:"time,omitempty,omitempty"`
	Frequency  float32           `json:"frequency,omitempty"`
	Modulation string            `json:"modulation,omitempty"`
	DataRate   string            `json:"data_rate,omitempty"`
	Bitrate    uint32            `json:"bit_rate,omitempty"`
	CodingRate string            `json:"coding_rate,omitempty"`
	Gateways   []GatewayMetadata `json:"gateways,omitempty"`
	LocationMetadata
}

type GatewayMetadata struct {
	GtwID      string   `json:"gtw_id,omitempty"`
	GtwTrusted bool     `json:"gtw_trusted,omitempty"`
	Timestamp  uint32   `json:"timestamp,omitempty"`
	Time       JSONTime `json:"time,omitempty"`
	Channel    uint32   `json:"channel"`
	RSSI       float32  `json:"rssi,omitempty"`
	SNR        float32  `json:"snr,omitempty"`
	RFChain    uint32   `json:"rf_chain,omitempty"`
	LocationMetadata
}

// JSONTime is a time.Time that marshals to/from RFC3339Nano format
type JSONTime time.Time

// LocationMetadata contains GPS coordinates
type LocationMetadata struct {
	Latitude  float32 `json:"latitude,omitempty"`
	Longitude float32 `json:"longitude,omitempty"`
	Altitude  int32   `json:"altitude,omitempty"`
}

// DownlinkMessage represents an application-layer downlink message
type DownlinkMessage struct {
	AppID         string                 `json:"app_id,omitempty"`
	DevID         string                 `json:"dev_id,omitempty"`
	FPort         uint8                  `json:"port"`
	Confirmed     bool                   `json:"confirmed,omitempty"`
	PayloadRaw    []byte                 `json:"payload_raw,omitempty"`
	PayloadFields map[string]interface{} `json:"payload_fields,omitempty"`
}
