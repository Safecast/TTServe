// Copyright 2019 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

type NoteEvent struct {
    Req string                  `json:"req,omitempty"`
    Rsp string					`json:"rsp,omitempty"`
    Error string                `json:"err,omitempty"`
	NoteID string				`json:"note,omitempty"`
    Deleted bool                `json:"deleted,omitempty"`
    Sent bool					`json:"queued,omitempty"`
    Bulk bool                   `json:"bulk,omitempty"`
    NotefileID string           `json:"file,omitempty"`
    DeviceUID string            `json:"device,omitempty"`
	DeviceSN string				`json:"sn,omitempty"`
	AppUID string				`json:"app,omitempty"`
	ProductUID string			`json:"product,omitempty"`
	EndpointID string			`json:"endpoint,omitempty"`
	TowerCountry string			`json:"tower_country,omitempty"`
	TowerLocation string		`json:"tower_location,omitempty"`
	TowerTimeZone string		`json:"tower_timezone,omitempty"`
	TowerLat float64			`json:"tower_lat,omitempty"`
	TowerLon float64			`json:"tower_lon,omitempty"`
	When int64					`json:"when,omitempty"`
	Where string				`json:"where,omitempty"`
	WhereLat float64			`json:"where_lat,omitempty"`
	WhereLon float64			`json:"where_lon,omitempty"`
	Routed int64				`json:"routed,omitempty"`
    Updates int32               `json:"updates,omitempty"`
    Body *interface{}			`json:"body,omitempty"`
    Payload []byte              `json:"payload,omitempty"`
}
