// Copyright © 2015 Safecast
// Use of this source code is governed by the Creative Commons Non-Commercial
// license.  These definitions are derived from:
// https://api.safecast.org/
package main

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
    EnvTemp      string `json:"env_temp,omitempty"`      // Degrees centigrade
    EnvHumid     string `json:"env_humid,omitempty"`     // Percent RH
}
