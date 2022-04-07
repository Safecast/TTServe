// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Safecast V1 API data structures, implemented in such a way
// that JSON strictness is quite forgiving.  This is necessary for
// messages received from Pointcast and Safecast Air devices.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type safecastDataV1ToParse struct {
	CapturedAtRaw   interface{} `json:"captured_at,omitempty"`
	DeviceTypeIDRaw interface{} `json:"devicetype_id,omitempty"`
	LocationNameRaw interface{} `json:"location_name,omitempty"`
	UnitRaw         interface{} `json:"unit,omitempty"`
	ChannelIDRaw    interface{} `json:"channel_id,omitempty"`
	DeviceIDRaw     interface{} `json:"device_id,omitempty"`
	OriginalIDRaw   interface{} `json:"original_id,omitempty"`
	SensorIDRaw     interface{} `json:"sensor_id,omitempty"`
	StationIDRaw    interface{} `json:"station_id,omitempty"`
	UserIDRaw       interface{} `json:"user_id,omitempty"`
	IDRaw           interface{} `json:"id,omitempty"`
	HeightRaw       interface{} `json:"height,omitempty"`
	ValueRaw        interface{} `json:"value,omitempty"`
	LatitudeRaw     interface{} `json:"latitude,omitempty"`
	LongitudeRaw    interface{} `json:"longitude,omitempty"`
}

// SafecastDataV1ToEmit is the data structure used to send things back to V1 Safecast devices
type SafecastDataV1ToEmit struct {
	CapturedAt   *string `json:"captured_at,omitempty"`
	DeviceID     *string `json:"device_id,omitempty"`
	Value        *string `json:"value,omitempty"`
	Unit         *string `json:"unit,omitempty"`
	Latitude     *string `json:"latitude,omitempty"`
	Longitude    *string `json:"longitude,omitempty"`
	Height       *string `json:"height,omitempty"`
	LocationName *string `json:"location_name,omitempty"`
	ChannelID    *string `json:"channel_id,omitempty"`
	OriginalID   *string `json:"original_id,omitempty"`
	SensorID     *string `json:"sensor_id,omitempty"`
	StationID    *string `json:"station_id,omitempty"`
	UserID       *string `json:"user_id,omitempty"`
	ID           *string `json:"id,omitempty"`
	DeviceTypeID *string `json:"devicetype_id,omitempty"`
}

// SafecastDataV1 is the "loose" JSON data structure used by all V1 safecast devices
type SafecastDataV1 struct {
	CapturedAt   *string  `json:"captured_at,omitempty"`
	DeviceTypeID *string  `json:"devicetype_id,omitempty"`
	LocationName *string  `json:"location_name,omitempty"`
	Unit         *string  `json:"unit,omitempty"`
	ChannelID    *uint32  `json:"channel_id,omitempty"`
	DeviceID     *uint32  `json:"device_id,omitempty"`
	OriginalID   *uint32  `json:"original_id,omitempty"`
	SensorID     *uint32  `json:"sensor_id,omitempty"`
	StationID    *uint32  `json:"station_id,omitempty"`
	UserID       *uint32  `json:"user_id,omitempty"`
	ID           *uint32  `json:"id,omitempty"`
	Height       *float64 `json:"height,omitempty"`
	Value        *float64 `json:"value,omitempty"`
	Latitude     *float64 `json:"latitude,omitempty"`
	Longitude    *float64 `json:"longitude,omitempty"`
}

// SafecastV1Decode converts a loosely-formatted message to a structured safecast object
func SafecastV1Decode(r io.Reader) (out *SafecastDataV1, emit *SafecastDataV1ToEmit, err error) {

	// Create a new instance, and decode the I/O stream into the fields as well
	// as the interfaces{}, which, when queried, can supply us not only with values
	// but also with type information.
	in := new(safecastDataV1ToParse)
	out = new(SafecastDataV1)
	emit = new(SafecastDataV1ToEmit)
	err = json.NewDecoder(r).Decode(in)
	if err != nil {
		return
	}

	// Go through things that pass straight through
	switch t := in.CapturedAtRaw.(type) {
	case string:
		out.CapturedAt = &t
		emit.CapturedAt = &t
	}
	switch t := in.DeviceTypeIDRaw.(type) {
	case string:
		out.DeviceTypeID = &t
		emit.DeviceTypeID = &t
	}
	switch t := in.LocationNameRaw.(type) {
	case string:
		out.LocationName = &t
		emit.LocationName = &t
	}
	switch t := in.UnitRaw.(type) {
	case string:
		out.Unit = &t
		emit.Unit = &t
	}

	// Now go through each Raw interface and unpack the value into the corresponding non-Raw field
	switch t := in.ChannelIDRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
		u64, err := strconv.ParseUint(t, 10, 32)
		if err == nil {
			u32 := uint32(u64)
			out.ChannelID = &u32
			emit.ChannelID = &t
		}
	case float64:
		u32 := uint32(t)
		out.ChannelID = &u32
		str := fmt.Sprintf("%v", t)
		emit.ChannelID = &str
	}

	switch t := in.DeviceIDRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
		u64, err := strconv.ParseUint(t, 10, 32)
		if err == nil {
			u32 := uint32(u64)
			out.DeviceID = &u32
			emit.DeviceID = &t
		}
	case float64:
		u32 := uint32(t)
		out.DeviceID = &u32
		str := fmt.Sprintf("%v", t)
		emit.DeviceID = &str
	}

	switch t := in.OriginalIDRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
		u64, err := strconv.ParseUint(t, 10, 32)
		if err == nil {
			u32 := uint32(u64)
			out.OriginalID = &u32
			emit.OriginalID = &t
		}
	case float64:
		u32 := uint32(t)
		out.OriginalID = &u32
		str := fmt.Sprintf("%v", t)
		emit.OriginalID = &str
	}

	switch t := in.SensorIDRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
		u64, err := strconv.ParseUint(t, 10, 32)
		if err == nil {
			u32 := uint32(u64)
			out.SensorID = &u32
			emit.SensorID = &t
		}
	case float64:
		u32 := uint32(t)
		out.SensorID = &u32
		str := fmt.Sprintf("%v", t)
		emit.SensorID = &str
	}

	switch t := in.StationIDRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
		u64, err := strconv.ParseUint(t, 10, 32)
		if err == nil {
			u32 := uint32(u64)
			out.StationID = &u32
			emit.StationID = &t
		}
	case float64:
		u32 := uint32(t)
		out.StationID = &u32
		str := fmt.Sprintf("%v", t)
		emit.StationID = &str
	}

	switch t := in.UserIDRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
		u64, err := strconv.ParseUint(t, 10, 32)
		if err == nil {
			u32 := uint32(u64)
			out.UserID = &u32
			emit.UserID = &t
		}
	case float64:
		u32 := uint32(t)
		out.UserID = &u32
		str := fmt.Sprintf("%v", t)
		emit.UserID = &str
	}

	switch t := in.IDRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
		u64, err := strconv.ParseUint(t, 10, 32)
		if err == nil {
			u32 := uint32(u64)
			out.ID = &u32
			emit.ID = &t
		}
	case float64:
		u32 := uint32(t)
		out.ID = &u32
		str := fmt.Sprintf("%v", t)
		emit.ID = &str
	}

	switch t := in.HeightRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
		f64, err := strconv.ParseFloat(t, 64)
		if err == nil {
			out.Height = &f64
			emit.Height = &t
		}
	case float64:
		f64 := float64(t)
		out.Height = &f64
		str := fmt.Sprintf("%v", t)
		emit.Height = &str
	}

	switch t := in.ValueRaw.(type) {
	case string:
		// This is to correct for a safecast-air bug
		// observed on 2017-02-15 wherein if the first char
		// is a space AND the resulting value is 0, it is
		// a bogus value that should be ignored
		var beginsWithSpace = t[0] == ' '
		t = strings.TrimSpace(t)
		f64, err := strconv.ParseFloat(t, 64)
		if err == nil {
			if f64 != 0 || !beginsWithSpace {
				out.Value = &f64
				emit.Value = &t
			}
		}
	case float64:
		f64 := float64(t)
		out.Value = &f64
		str := fmt.Sprintf("%v", t)
		emit.Value = &str
	}

	switch t := in.LatitudeRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
		f64, err := strconv.ParseFloat(t, 64)
		if err == nil {
			out.Latitude = &f64
			emit.Latitude = &t
		}
	case float64:
		f64 := float64(t)
		out.Latitude = &f64
		str := fmt.Sprintf("%v", t)
		emit.Latitude = &str
	}

	switch t := in.LongitudeRaw.(type) {
	case string:
		t = strings.TrimSpace(t)
		f64, err := strconv.ParseFloat(t, 64)
		if err == nil {
			out.Longitude = &f64
			emit.Longitude = &t
		}
	case float64:
		f64 := float64(t)
		out.Longitude = &f64
		str := fmt.Sprintf("%v", t)
		emit.Longitude = &str
	}

	// Done
	return
}
