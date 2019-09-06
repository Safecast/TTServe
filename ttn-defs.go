// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license found with the
// source code from where this was derived:
// https://github.com/TheThingsNetwork/ttn/core/types

package main

import (
	"time"
)

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
	Frequency  float64           `json:"frequency,omitempty"`
	Modulation string            `json:"modulation,omitempty"`
	DataRate   string            `json:"data_rate,omitempty"`
	Bitrate    uint32            `json:"bit_rate,omitempty"`
	CodingRate string            `json:"coding_rate,omitempty"`
	Gateways   []GatewayMetadata `json:"gateways,omitempty"`
	LocationMetadata
}

// GatewayMetadata contains metadata of a TTN gateway
type GatewayMetadata struct {
	GtwID      string   `json:"gtw_id,omitempty"`
	GtwTrusted bool     `json:"gtw_trusted,omitempty"`
	Timestamp  uint32   `json:"timestamp,omitempty"`
	Time       JSONTime `json:"time,omitempty"`
	Channel    uint32   `json:"channel"`
	RSSI       float64  `json:"rssi,omitempty"`
	SNR        float64  `json:"snr,omitempty"`
	RFChain    uint32   `json:"rf_chain,omitempty"`
	LocationMetadata
}

// JSONTime is a time.Time that marshals to/from RFC3339Nano format
type JSONTime time.Time

// LocationMetadata contains GPS coordinates
type LocationMetadata struct {
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
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


// MarshalText implements the encoding.TextMarshaler interface
func (t JSONTime) MarshalText() ([]byte, error) {
	if time.Time(t).IsZero() || time.Time(t).Unix() == 0 {
		return []byte{}, nil
	}
	stamp := time.Time(t).UTC().Format(time.RFC3339Nano)
	return []byte(stamp), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface
func (t *JSONTime) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*t = JSONTime{}
		return nil
	}
	time, err := time.Parse(time.RFC3339Nano, string(text))
	if err != nil {
		return err
	}
	*t = JSONTime(time)
	return nil
}

// BuildTime builds a new JSONTime
func BuildTime(unixNano int64) JSONTime {
	if unixNano == 0 {
		return JSONTime{}
	}
	return JSONTime(time.Unix(0, 0).Add(time.Duration(unixNano)).UTC())
}
