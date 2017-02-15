// Safecast API data structures

package main

import (
	"io"
	"encoding/json"
	"strconv"
)

type SafecastDataV1 struct {
    CapturedAt		*string `json:"captured_at,omitempty"`
    DeviceTypeID	*string `json:"devicetype_id,omitempty"`
    LocationName	*string `json:"location_name,omitempty"`
    Unit			*string `json:"unit,omitempty"`
    ChannelIDRaw	interface{} `json:"channel_id,omitempty"`
    ChannelID		*uint32
    DeviceIDRaw		interface{} `json:"device_id,omitempty"`
    DeviceID		*uint32
    OriginalIDRaw	interface{} `json:"original_id,omitempty"`
    OriginalID		*uint32
    SensorIDRaw		interface{} `json:"sensor_id,omitempty"`
    SensorID		*uint32
    StationIDRaw	interface{} `json:"station_id,omitempty"`
    StationID		*uint32
    UserIDRaw		interface{} `json:"user_id,omitempty"`
    UserID			*uint32
    IDRaw			interface{} `json:"id,omitempty"`
    ID				*uint32
    HeightRaw		interface{} `json:"height,omitempty"`
    Height			*int32
    ValueRaw		interface{} `json:"value,omitempty"`
    Value			*float32
    LatitudeRaw		interface{} `json:"latitude,omitempty"`
    Latitude		*float32
    LongitudeRaw	interface{} `json:"longitude,omitempty"`
    Longitude		*float32
}

func Decode(r io.Reader) (x *SafecastDataV1, err error) {
	var i32 int32
	var u32 uint32
	var u64 uint64
	var f32 float32
	var f64 float64
	x = new(SafecastDataV1)
	if err = json.NewDecoder(r).Decode(x); err != nil {
		return
	}

	switch t := x.ChannelIDRaw.(type) {
	case string:
	    u64, err = strconv.ParseUint(t, 10, 32)
	    if err == nil {
			u32 = uint32(u64)
	        x.ChannelID = &u32
	    }
	case float64:
		u32 = uint32(t)
		x.ChannelID = &u32
	}

	switch t := x.DeviceIDRaw.(type) {
	case string:
	    u64, err = strconv.ParseUint(t, 10, 32)
	    if err == nil {
			u32 = uint32(u64)
	        x.DeviceID = &u32
	    }
	case float64:
		u32 = uint32(t)
		x.DeviceID = &u32
	}

	switch t := x.OriginalIDRaw.(type) {
	case string:
	    u64, err = strconv.ParseUint(t, 10, 32)
	    if err == nil {
			u32 = uint32(u64)
	        x.OriginalID = &u32
	    }
	case float64:
		u32 = uint32(t)
		x.OriginalID = &u32
	}

	switch t := x.SensorIDRaw.(type) {
	case string:
	    u64, err = strconv.ParseUint(t, 10, 32)
	    if err == nil {
			u32 = uint32(u64)
	        x.SensorID = &u32
	    }
	case float64:
		u32 = uint32(t)
		x.SensorID = &u32
	}

	switch t := x.StationIDRaw.(type) {
	case string:
	    u64, err = strconv.ParseUint(t, 10, 32)
	    if err == nil {
			u32 = uint32(u64)
	        x.StationID = &u32
	    }
	case float64:
		u32 = uint32(t)
		x.StationID = &u32
	}

	switch t := x.UserIDRaw.(type) {
	case string:
	    u64, err = strconv.ParseUint(t, 10, 32)
	    if err == nil {
			u32 = uint32(u64)
	        x.UserID = &u32
	    }
	case float64:
		u32 = uint32(t)
		x.UserID = &u32
	}

	switch t := x.IDRaw.(type) {
	case string:
	    u64, err = strconv.ParseUint(t, 10, 32)
	    if err == nil {
			u32 = uint32(u64)
	        x.ID = &u32
	    }
	case float64:
		u32 = uint32(t)
		x.ID = &u32
	}

	switch t := x.HeightRaw.(type) {
	case string:
	    f64, err = strconv.ParseFloat(t, 32)
	    if err == nil {
			i32 = int32(f64)
	        x.Height = &i32
	    }
	case float64:
		i32 = int32(t)
		x.Height = &i32
	}

	switch t := x.ValueRaw.(type) {
	case string:
	    f64, err = strconv.ParseFloat(t, 32)
	    if err == nil {
			f32 = float32(f64)
	        x.Value = &f32
	    }
	case float64:
		f32 = float32(t)
		x.Value = &f32
	}

	switch t := x.LatitudeRaw.(type) {
	case string:
	    f64, err = strconv.ParseFloat(t, 32)
	    if err == nil {
			f32 = float32(f64)
	        x.Latitude = &f32
	    }
	case float64:
		f32 = float32(t)
		x.Latitude = &f32
	}

	switch t := x.LongitudeRaw.(type) {
	case string:
	    f64, err = strconv.ParseFloat(t, 32)
	    if err == nil {
			f32 = float32(f64)
	        x.Longitude = &f32
	    }
	case float64:
		f32 = float32(t)
		x.Longitude = &f32
	}

	return
}
