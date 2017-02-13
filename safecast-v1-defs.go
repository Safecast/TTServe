// Safecast API data structures

package main

type SafecastDataV1Strings struct {
    CapturedAt		string `json:"captured_at,omitempty"`   // 2016-02-20T14:02:25Z
    ChannelID		string `json:"channel_id,omitempty"`    // nil
    DeviceID		string `json:"device_id,omitempty"`     // 140
    DeviceTypeID	string `json:"devicetype_id,omitempty"` // nil
    Height			string `json:"height,omitempty"`        // 123
    ID				string `json:"id,omitempty"`            // 972298
    LocationName	string `json:"location_name,omitempty"` // nil
    OriginalID		string `json:"original_id,omitempty"`   // 972298
    SensorID		string `json:"sensor_id,omitempty"`     // nil
    StationID		string `json:"station_id,omitempty"`    // nil
    Unit			string `json:"unit,omitempty"`          // cpm
    UserID			string `json:"user_id,omitempty"`       // 304
    Value			string `json:"value,omitempty"`         // 36
    Latitude		string `json:"latitude,omitempty"`      // 37.0105
    Longitude		string `json:"longitude,omitempty"`     // 140.9253
}


type SafecastDataV1Numerics struct {
    CapturedAt		string `json:"captured_at,omitempty"`   // 2016-02-20T14:02:25Z
    ChannelID		uint32 `json:"channel_id,omitempty"`    // nil
    DeviceID		uint32 `json:"device_id,omitempty"`     // 140
    DeviceTypeID	string `json:"devicetype_id,omitempty"` // nil
    Height			int32  `json:"height,omitempty"`        // 123
    ID				uint32 `json:"id,omitempty"`            // 972298
    LocationName	string `json:"location_name,omitempty"` // nil
    OriginalID		uint32 `json:"original_id,omitempty"`   // 972298
    SensorID		uint32 `json:"sensor_id,omitempty"`     // nil
    StationID		string `json:"station_id,omitempty"`    // nil
    Unit			string `json:"unit,omitempty"`          // cpm
    UserID			uint32 `json:"user_id,omitempty"`       // 304
    Value			float32 `json:"value,omitempty"`        // 36
    Latitude		float32 `json:"latitude,omitempty"`     // 37.0105
    Longitude		float32 `json:"longitude,omitempty"`    // 140.9253
}

