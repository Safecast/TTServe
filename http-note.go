// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the routing from a note
package main

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	ttdata "github.com/Safecast/ttdefs"
	"github.com/blues/note-go/note"
)

// Schemas for the different file types
type sensorTRACKER struct {
	CPM         float64 `json:"cpm,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	Humidity    float64 `json:"humidity,omitempty"`
	Pressure    float64 `json:"pressure,omitempty"`
	Voltage     float64 `json:"voltage,omitempty"`
	Distance    float64 `json:"distance,omitempty"`
	Seconds     float64 `json:"seconds,omitempty"`
	Velocity    float64 `json:"velocity,omitempty"`
	Bearing     float64 `json:"bearing,omitempty"`
}
type sensorBAT struct {
	Voltage float64 `json:"voltage,omitempty"`
}
type sensorINA struct {
	Voltage float64 `json:"voltage,omitempty"`
	Current float64 `json:"current,omitempty"`
}
type sensorBME struct {
	Temperature float64 `json:"temp,omitempty"`
	Humidity    float64 `json:"humid,omitempty"`
	Pressure    float64 `json:"press,omitempty"`
}
type sensorRAD struct {
	CPM     float64 `json:"cpm,omitempty"`
	Seconds float64 `json:"secs,omitempty"`
}
type sensorAIR struct {
	Pm01_0     float64  `json:"pm01_0,omitempty"`
	Pm02_5     float64  `json:"pm02_5,omitempty"`
	Pm10_0     float64  `json:"pm10_0,omitempty"`
	Count00_30 uint32   `json:"c00_30,omitempty"`
	Count00_50 uint32   `json:"c00_50,omitempty"`
	Count01_00 uint32   `json:"c01_00,omitempty"`
	Count02_50 uint32   `json:"c02_50,omitempty"`
	Count05_00 uint32   `json:"c05_00,omitempty"`
	Count10_00 uint32   `json:"c10_00,omitempty"`
	CountSecs  uint32   `json:"csecs,omitempty"`
	Pm01_0cf1  *float64 `json:"pm01_0cf1,omitempty"`
	Pm02_5cf1  *float64 `json:"pm02_5cf1,omitempty"`
	Pm10_0cf1  *float64 `json:"pm10_0cf1,omitempty"`
	Samples    uint32   `json:"csamples,omitempty"`
	Model      string   `json:"sensor,omitempty"`
	Voltage    *float64 `json:"voltage,omitempty"`
	TempOLD    *float64 `json:"temp,omitempty"`
	HumidOLD   *float64 `json:"humid,omitempty"`
	PressOLD   *float64 `json:"press,omitempty"`
	Temp       *float64 `json:"temperature,omitempty"`
	Humid      *float64 `json:"humidity,omitempty"`
	Press      *float64 `json:"pressure,omitempty"`
	Charging   *bool    `json:"charging,omitempty"`
	USB        *bool    `json:"usb,omitempty"`
	Indoors    *bool    `json:"indoors,omitempty"`
	CPM        float64  `json:"cpm,omitempty"`
	USV        float64  `json:"usv,omitempty"`
}
type sensorTRACK struct {
	Lat      float64 `json:"lat,omitempty"`
	Lon      float64 `json:"lon,omitempty"`
	Distance float64 `json:"distance,omitempty"`
	Seconds  float64 `json:"seconds,omitempty"`
	Velocity float64 `json:"velocity,omitempty"`
	Bearing  float64 `json:"bearing,omitempty"`
}

// Handle inbound HTTP requests from individual data Notes, in test mode
func inboundWebNoteHandlerTest(rw http.ResponseWriter, req *http.Request) {
	noteHandler(rw, req, true)
}

// Handle inbound HTTP requests from individual data Notes, in production mode
func inboundWebNoteHandler(rw http.ResponseWriter, req *http.Request) {
	noteHandler(rw, req, false)
}

// Handle inbound HTTP requests from individual data Notes
func noteHandler(rw http.ResponseWriter, req *http.Request, testMode bool) {
	var body []byte
	var err error

	// Count the request
	stats.Count.HTTP++

	// Get the remote address, and only add this to the count if it's likely from
	// the internal HTTP load balancer.
	remoteAddr, isReal, abusive := getRequestorIPv4(req)
	if abusive {
		return
	}
	if !isReal {
		remoteAddr = "internal address"
	}
	transportStr := "notehub:" + remoteAddr

	// Read the body as a byte array
	body, err = ioutil.ReadAll(req.Body)
	if err != nil {
		return

	}

	// Exit if it's there's nothing there
	if len(body) == 0 {
		return
	}

	// Unmarshal into a notehub Event structure, and exit if badly formatted
	e := note.Event{}
	err = json.Unmarshal(body, &e)
	if err != nil {
		return
	}

	// Convert to Safecast data, and exit if failure
	sd, upload, log, err := noteToSD(e, transportStr, testMode)
	if err != nil {
		fmt.Printf("NOTE ignored: %s\n%s\n", err, body)
		return
	}

	// Display info about it
	fmt.Printf("\n%s Received payload for %s from %s in %s\n", LogTime(), sd.DeviceUID, transportStr,
		e.TowerLocation+" "+e.TowerCountry)

	// Send it to the Ingest service
	if upload {
		go SafecastUpload(sd)
	}

	// Add native event data and log it
	if log {
		native := map[string]interface{}{}
		err = json.Unmarshal(body, &native)
		if err == nil {
			sd.Native = &native
			go SafecastLog(sd)
		}
	}

}

// Deterministic way to convert a Notecard DeviceUID to a Safecast DeviceID, in a way that
// reserves the low 2^20 addresses for fixed allocation as per Rob agreement (see ttnode/src/io.c)
func notecardDeviceUIDToSafecastDeviceID(notecardDeviceUID string) (safecastDeviceURN string, safecastDeviceID uint32) {
	safecastDeviceURN = "note:" + notecardDeviceUID
	safecastDeviceID = crc32.ChecksumIEEE([]byte(safecastDeviceURN))
	if safecastDeviceID < 1048576 {
		safecastDeviceID = ^safecastDeviceID
	}
	return
}

// ReformatFromNote reformats to our standard normalized data format
func noteToSD(e note.Event, transport string, testMode bool) (sd ttdata.SafecastData, upload bool, log bool, err error) {

	// Mark it as to whether or not it is a test measurement
	isTest := testMode
	var dev ttdata.Dev
	sd.Dev = &dev
	if isTest {
		sd.Dev.Test = &isTest
	}

	// Device movement
	if e.Moved != 0 {
		deviceMovedAt := time.Unix(e.Moved, 0).Format("2006-01-02T15:04:05Z")
		dev.Moved = &deviceMovedAt
	}
	if e.Orientation != "" {
		dev.Orientation = &e.Orientation
	}

	// Device temp & voltage
	if e.Temp != 0.0 {
		dev.Temp = &e.Temp
	}
	if e.Voltage != 0.0 {
		var bat ttdata.Bat
		bat.Voltage = &e.Voltage
		sd.Bat = &bat
	}

	// Cell info
	if e.Rat != "" {
		dev.Rat = &e.Rat
	}
	if e.Bars != 0 {
		dev.Bars = &e.Bars
	}

	// Convert to safecast device ID
	sd.DeviceUID, sd.DeviceID = notecardDeviceUIDToSafecastDeviceID(e.DeviceUID)

	// Serial number
	sd.DeviceSN = e.DeviceSN

	// Product UID is REQUIRED of anything which passed through from notehub
	if e.ProductUID == "" {
		err = fmt.Errorf("note: event has no product UID")
		return
	}
	sd.DeviceClass = e.ProductUID

	// Optional device contact info, for accountability
	if e.DeviceContact != nil {
		sd.DeviceContactName = e.DeviceContact.Name
		sd.DeviceContactOrg = e.DeviceContact.Affiliation
		sd.DeviceContactRole = e.DeviceContact.Role
		sd.DeviceContactEmail = e.DeviceContact.Email
	}

	// When captured on the device
	if e.When != 0 {
		capturedAt := time.Unix(e.When, 0).Format("2006-01-02T15:04:05Z")
		sd.CapturedAt = &capturedAt
	}

	// Service-related
	var svc ttdata.Service
	sd.Service = &svc
	svc.Transport = &transport
	if e.Routed != 0 {
		uploadedAt := time.Unix(e.Routed, 0).Format("2006-01-02T15:04:05Z")
		sd.UploadedAt = &uploadedAt
	} else {
		uploadedAt := NowInUTC()
		svc.UploadedAt = &uploadedAt
	}

	// Where captured
	if e.Where != "" {
		var loc ttdata.Loc
		sd.Loc = &loc
		var lat, lon float64
		lat = float64(e.WhereLat)
		lon = float64(e.WhereLon)
		sd.Loc.Lat = &lat
		sd.Loc.Lon = &lon
		sd.Loc.Olc = &e.Where
		if e.WhereLocation != "" {
			sd.Loc.LocName = &e.WhereLocation
		}
		if e.WhereCountry != "" {
			sd.Loc.LocCountry = &e.WhereCountry
		}
		if e.WhereTimeZone != "" {
			sd.Loc.LocZone = &e.WhereTimeZone
		}
	}

	// If there's no body, bail
	if e.Body == nil {
		err = fmt.Errorf("note: no recognizable sensor data")
		return
	}
	var sensorJSON []byte
	sensorJSON, err = json.Marshal(e.Body)
	if err != nil {
		return
	}

	// Decompose the body with a per-notefile schema
	upload = true
	log = true
	switch e.NotefileID {

	case "_session.qo":
		upload = false
		// Everything in session has been captured above
		return

	case "_air.qo":
		s := sensorAIR{}
		err = json.Unmarshal(sensorJSON, &s)
		if err != nil {
			return
		}
		switch s.Model {
		case "lnd712":
			var lnd ttdata.Lnd
			lnd.U712 = &s.CPM
			lnd.USv = &s.USV
			sd.Lnd = &lnd
		default:
			var pms ttdata.Pms
			pms.Pm01_0 = &s.Pm01_0
			pms.Pm02_5 = &s.Pm02_5
			pms.Pm10_0 = &s.Pm10_0
			pms.Count00_30 = &s.Count00_30
			pms.Count00_50 = &s.Count00_50
			pms.Count01_00 = &s.Count01_00
			pms.Count02_50 = &s.Count02_50
			pms.Count05_00 = &s.Count05_00
			pms.Count10_00 = &s.Count10_00
			pms.CountSecs = &s.CountSecs
			pms.Samples = &s.Samples
			pms.Pm01_0cf1 = s.Pm01_0cf1
			pms.Pm02_5cf1 = s.Pm02_5cf1
			pms.Pm10_0cf1 = s.Pm10_0cf1
			pms.Model = &s.Model
			sd.Pms = &pms
		}
		if s.Indoors != nil {
			indoors := true
			sd.Dev.Indoors = &indoors
		}
		if s.Voltage != nil {
			var bat ttdata.Bat
			bat.Voltage = s.Voltage
			bat.Charging = s.Charging
			bat.Line = s.USB
			sd.Bat = &bat
		}
		if s.TempOLD != nil {
			var env ttdata.Env
			env.Temp = s.TempOLD
			env.Humid = s.HumidOLD
			env.Press = s.PressOLD
			sd.Env = &env
		}
		if s.Temp != nil {
			var env ttdata.Env
			env.Temp = s.Temp
			env.Humid = s.Humid
			env.Press = s.Press
			sd.Env = &env
		}

	case "_track.qo":
		s := sensorTRACKER{}
		err = json.Unmarshal(sensorJSON, &s)
		if err != nil {
			return
		}
		var bat ttdata.Bat
		bat.Voltage = &s.Voltage
		sd.Bat = &bat
		var env ttdata.Env
		env.Temp = &s.Temperature
		if s.Humidity != 0 {
			env.Humid = &s.Humidity
		}
		if s.Pressure != 0 {
			env.Press = &s.Pressure
		}
		sd.Env = &env
		if s.CPM > 0 {
			var lnd ttdata.Lnd
			lnd.U7318 = &s.CPM
			sd.Lnd = &lnd
		}
		var track ttdata.Track
		track.Distance = &s.Distance
		var secs = uint32(s.Seconds)
		track.Seconds = &secs
		track.Velocity = &s.Velocity
		track.Bearing = &s.Bearing
		sd.Track = &track

	case "bat.qo":
		s := sensorBAT{}
		err = json.Unmarshal(sensorJSON, &s)
		if err != nil {
			return
		}
		var bat ttdata.Bat
		bat.Voltage = &s.Voltage
		sd.Bat = &bat

	case "bat-ina219.qo":
		s := sensorINA{}
		err = json.Unmarshal(sensorJSON, &s)
		if err != nil {
			return
		}
		var bat ttdata.Bat
		bat.Voltage = &s.Voltage
		bat.Current = &s.Current
		sd.Bat = &bat

	case "air-bme280.qo":
		s := sensorBME{}
		err = json.Unmarshal(sensorJSON, &s)
		if err != nil {
			return
		}
		var env ttdata.Env
		env.Temp = &s.Temperature
		env.Humid = &s.Humidity
		env.Press = &s.Pressure
		sd.Env = &env

	case "rad1-lnd7318u.qo":
		fallthrough
	case "rad0-lnd7318u.qo":
		s := sensorRAD{}
		err = json.Unmarshal(sensorJSON, &s)
		if err != nil {
			return
		}
		var lnd ttdata.Lnd
		lnd.U7318 = &s.CPM
		sd.Lnd = &lnd

	case "rad1-lnd7318c.qo":
		fallthrough
	case "rad0-lnd7318c.qo":
		s := sensorRAD{}
		err = json.Unmarshal(sensorJSON, &s)
		if err != nil {
			return
		}
		var lnd ttdata.Lnd
		lnd.C7318 = &s.CPM
		sd.Lnd = &lnd

	case "rad1-lnd7128ec.qo":
		fallthrough
	case "rad0-lnd7128ec.qo":
		s := sensorRAD{}
		err = json.Unmarshal(sensorJSON, &s)
		if err != nil {
			return
		}
		var lnd ttdata.Lnd
		lnd.EC7128 = &s.CPM
		sd.Lnd = &lnd

	case "aq0-pms7003.qo":
		fallthrough
	case "aq0-pms5003.qo":
		s := sensorAIR{}
		err = json.Unmarshal(sensorJSON, &s)
		if err != nil {
			return
		}
		var pms ttdata.Pms
		pms.Pm01_0 = &s.Pm01_0
		pms.Pm02_5 = &s.Pm02_5
		pms.Pm10_0 = &s.Pm10_0
		pms.Count00_30 = &s.Count00_30
		pms.Count00_50 = &s.Count00_50
		pms.Count01_00 = &s.Count01_00
		pms.Count02_50 = &s.Count02_50
		pms.Count05_00 = &s.Count05_00
		pms.Count10_00 = &s.Count10_00
		pms.CountSecs = &s.CountSecs
		pms.Samples = &s.Samples
		model := strings.Split(strings.Split(e.NotefileID, "-")[1], ".")[0]
		pms.Model = &model
		sd.Pms = &pms

	case "aq1-pms7003.qo":
		fallthrough
	case "aq1-pms5003.qo":
		s := sensorAIR{}
		err = json.Unmarshal(sensorJSON, &s)
		if err != nil {
			return
		}
		var pms ttdata.Pms2
		pms.Pm01_0 = &s.Pm01_0
		pms.Pm02_5 = &s.Pm02_5
		pms.Pm10_0 = &s.Pm10_0
		pms.Count00_30 = &s.Count00_30
		pms.Count00_50 = &s.Count00_50
		pms.Count01_00 = &s.Count01_00
		pms.Count02_50 = &s.Count02_50
		pms.Count05_00 = &s.Count05_00
		pms.Count10_00 = &s.Count10_00
		pms.CountSecs = &s.CountSecs
		pms.Samples = &s.Samples
		model := strings.Split(strings.Split(e.NotefileID, "-")[1], ".")[0]
		pms.Model = &model
		sd.Pms2 = &pms

	case "track.qo":
		s := sensorTRACK{}
		err = json.Unmarshal(sensorJSON, &s)
		if err != nil {
			return
		}
		var track ttdata.Track
		track.Lat = &s.Lat
		track.Lon = &s.Lon
		track.Distance = &s.Distance
		var secs = uint32(s.Seconds)
		track.Seconds = &secs
		track.Velocity = &s.Velocity
		track.Bearing = &s.Bearing
		sd.Track = &track

	default:
		upload = false
		log = false
		fmt.Printf("*** note-go: no sensor data in file %s", e.NotefileID)
		return

	}

	// Done
	return

}
