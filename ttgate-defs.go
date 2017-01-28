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
