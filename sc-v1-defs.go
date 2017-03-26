// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Safecast V1 API data structures, implemented in such a way
// that JSON strictness is quite forgiving.  This is necessary for
// messages received from Pointcast and Safecast Air devices.
package main

import (
	"io"
	"encoding/json"
	"strconv"
	"strings"
)

type SafecastDataV1ToParse struct {
    CapturedAtRaw	interface{} `json:"captured_at,omitempty"`
    DeviceTypeIdRaw	interface{} `json:"devicetype_id,omitempty"`
    LocationNameRaw	interface{} `json:"location_name,omitempty"`
    UnitRaw			interface{} `json:"unit,omitempty"`
    ChannelIdRaw	interface{} `json:"channel_id,omitempty"`
    DeviceIdRaw		interface{} `json:"device_id,omitempty"`
    OriginalIdRaw	interface{} `json:"original_id,omitempty"`
    SensorIdRaw		interface{} `json:"sensor_id,omitempty"`
    StationIdRaw	interface{} `json:"station_id,omitempty"`
    UserIdRaw		interface{} `json:"user_id,omitempty"`
    IdRaw			interface{} `json:"id,omitempty"`
    HeightRaw		interface{} `json:"height,omitempty"`
    ValueRaw		interface{} `json:"value,omitempty"`
    LatitudeRaw		interface{} `json:"latitude,omitempty"`
    LongitudeRaw	interface{} `json:"longitude,omitempty"`
}

type SafecastDataV1 struct {
    CapturedAt		*string  `json:"captured_at,omitempty"`
    DeviceTypeId	*string  `json:"devicetype_id,omitempty"`
    LocationName	*string  `json:"location_name,omitempty"`
    Unit			*string  `json:"unit,omitempty"`
    ChannelId		*uint32  `json:"channel_id,omitempty"`
    DeviceId		*uint32  `json:"device_id,omitempty"`
    OriginalId		*uint32  `json:"original_id,omitempty"`
    SensorId		*uint32  `json:"sensor_id,omitempty"`
    StationId		*uint32  `json:"station_id,omitempty"`
    UserId			*uint32  `json:"user_id,omitempty"`
    Id				*uint32  `json:"id,omitempty"`
    Height			*float32 `json:"height,omitempty"`
    Value			*float32 `json:"value,omitempty"`
    Latitude		*float32 `json:"latitude,omitempty"`
    Longitude		*float32 `json:"longitude,omitempty"`
}

func SafecastV1Decode(r io.Reader) (out *SafecastDataV1, err error) {

	// Create a new instance, and decode the I/O stream into the fields as well
	// as the interfaces{}, which, when queried, can supply us not only with values
	// but also with type information.
	in := new(SafecastDataV1ToParse)
	out = new(SafecastDataV1)
	err = json.NewDecoder(r).Decode(in)
	if err != nil {
		return
	}

	// Go through things that pass straight through
	switch t := in.CapturedAtRaw.(type) {
	case string:
		out.CapturedAt = &t
	}
	switch t := in.DeviceTypeIdRaw.(type) {
	case string:
		out.DeviceTypeId = &t
	}
	switch t := in.LocationNameRaw.(type) {
	case string:
		out.LocationName = &t
	}
	switch t := in.UnitRaw.(type) {
	case string:
		out.Unit = &t
	}
	
	// Now go through each Raw interface and unpack the value into the corresponding non-Raw field
	switch t := in.ChannelIdRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
	    u64, err := strconv.ParseUint(t, 10, 32)
	    if err == nil {
			u32 := uint32(u64)
	        out.ChannelId = &u32
	    }
	case float64:
		u32 := uint32(t)
		out.ChannelId = &u32
	}

	switch t := in.DeviceIdRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
	    u64, err := strconv.ParseUint(t, 10, 32)
	    if err == nil {
			u32 := uint32(u64)
	        out.DeviceId = &u32
	    }
	case float64:
		u32 := uint32(t)
		out.DeviceId = &u32
	}

	switch t := in.OriginalIdRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
	    u64, err := strconv.ParseUint(t, 10, 32)
	    if err == nil {
			u32 := uint32(u64)
	        out.OriginalId = &u32
	    }
	case float64:
		u32 := uint32(t)
		out.OriginalId = &u32
	}

	switch t := in.SensorIdRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
	    u64, err := strconv.ParseUint(t, 10, 32)
	    if err == nil {
			u32 := uint32(u64)
	        out.SensorId = &u32
	    }
	case float64:
		u32 := uint32(t)
		out.SensorId = &u32
	}

	switch t := in.StationIdRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
	    u64, err := strconv.ParseUint(t, 10, 32)
	    if err == nil {
			u32 := uint32(u64)
	        out.StationId = &u32
	    }
	case float64:
		u32 := uint32(t)
		out.StationId = &u32
	}

	switch t := in.UserIdRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
	    u64, err := strconv.ParseUint(t, 10, 32)
	    if err == nil {
			u32 := uint32(u64)
	        out.UserId = &u32
	    }
	case float64:
		u32 := uint32(t)
		out.UserId = &u32
	}

	switch t := in.IdRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
	    u64, err := strconv.ParseUint(t, 10, 32)
	    if err == nil {
			u32 := uint32(u64)
	        out.Id = &u32
	    }
	case float64:
		u32 := uint32(t)
		out.Id = &u32
	}

	switch t := in.HeightRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
	    f64, err := strconv.ParseFloat(t, 32)
	    if err == nil {
			f32 := float32(f64)
	        out.Height = &f32
	    }
	case float64:
		f32 := float32(t)
		out.Height = &f32
	}

	switch t := in.ValueRaw.(type) {
	case string:
		// This is to correct for a safecast-air bug
		// observed on 2017-02-15 wherein if the first char
		// is a space AND the resulting value is 0, it is
		// a bogus value that should be ignored
		var beginsWithSpace = t[0] == ' '
		t = strings.TrimSpace(t)
	    f64, err := strconv.ParseFloat(t, 32)
	    if err == nil {
			f32 := float32(f64)
			if (f32 != 0 || !beginsWithSpace) {
		        out.Value = &f32
			}
	    }
	case float64:
		f32 := float32(t)
		out.Value = &f32
	}

	switch t := in.LatitudeRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
	    f64, err := strconv.ParseFloat(t, 32)
	    if err == nil {
			f32 := float32(f64)
	        out.Latitude = &f32
	    }
	case float64:
		f32 := float32(t)
		out.Latitude = &f32
	}

	switch t := in.LongitudeRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
	    f64, err := strconv.ParseFloat(t, 32)
	    if err == nil {
			f32 := float32(f64)
	        out.Longitude = &f32
	    }
	case float64:
		f32 := float32(t)
		out.Longitude = &f32
	}

	// Done
	return
}
