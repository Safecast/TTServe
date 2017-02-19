// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

type TTGateReq struct {
	Payload    []byte  `json:"payload"`
	Longitude  float32 `json:"longitude,omitempty"`
	Latitude   float32 `json:"latitude,omitempty"`
	Altitude   int32   `json:"altitude,omitempty"`
	Snr        float32 `json:"snr,omitempty"`
	Location   string  `json:"location,omitempty"`
	Transport  string  `json:"transport,omitempty"`
}
