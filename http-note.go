// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the routing from a note
package main

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"strings"
	"time"

	ttdata "github.com/Safecast/safecast-go"
	"github.com/blues/note-go/note"
)

// Schemas for the different file types
type sensorTRACKER struct {
	Model       string  `json:"sensor,omitempty"`
	CPM         float64 `json:"cpm,omitempty"`
	USV         float64 `json:"usv,omitempty"`
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
	Samples    uint32   `json:"csamples,omitempty"`
	Pm01_0cf1  *float64 `json:"pm01_0cf1,omitempty"`
	Pm02_5cf1  *float64 `json:"pm02_5cf1,omitempty"`
	Pm10_0cf1  *float64 `json:"pm10_0cf1,omitempty"`
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
	CPMCount   int      `json:"cpm_count,omitempty"`
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
	body, err = io.ReadAll(req.Body)
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

	// If this is an air reading, annotate it with AQI if possible
	aqiCalculate(&sd)

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

// Determines whether or not this deviceUID came from notehub
func safecastDeviceUIDIsFromNotehub(deviceUID string) bool {
	return strings.HasPrefix(deviceUID, "note:")
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

	// For loopback protection, make sure that the notecard deviceUID only has
	// a single colon in it, indicating that it's a notecard and not a notehub "webhook"
	// deviceUID that *we* may have sent it.
	if strings.Count(e.DeviceUID, ":") > 1 {
		err = fmt.Errorf("note: can't process a notehub 'webhook' event")
		return
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

	// When captured on the device
	if e.When != 0 {
		capturedAt := time.Unix(e.When, 0).Format("2006-01-02T15:04:05Z")
		sd.CapturedAt = &capturedAt
	}

	// Service-related
	var svc ttdata.Service
	sd.Service = &svc
	svc.Transport = &transport
	uploadedAt := NowInUTC()
	svc.UploadedAt = &uploadedAt

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
		case "lnd712": // Airnote Radiation
			var lnd ttdata.Lnd
			lnd.U712 = &s.CPM
			lnd.USv = &s.USV
			sd.Lnd = &lnd
		case "lnd7317": // Radnote, which is covered/shielded by default
			var lnd ttdata.Lnd
			lnd.C7318 = &s.CPM
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
			switch s.Model {
			default: // Airnote Radiation had no model field
				fallthrough
			case "lnd712": // Airnote Radiation
				var lnd ttdata.Lnd
				lnd.U712 = &s.CPM
				lnd.USv = &s.USV
				sd.Lnd = &lnd
			case "lnd7317": // Radnote, which is covered/shielded by default
				var lnd ttdata.Lnd
				lnd.C7318 = &s.CPM
				lnd.USv = &s.USV
				sd.Lnd = &lnd
			}
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

// notehubWebhookEventFromSD converts an SD to a Notehub webhook event
func notehubWebhookEventFromSD(sd ttdata.SafecastData) (deviceUID string, eventJSON []byte, err error) {

	// Form the body and event structures
	var body sensorAIR
	var event note.Event

	if sd.Lnd != nil {
		usvConversionFactor := 0
		if sd.Lnd.U7318 != nil {
			body.Model = "lnd7317"
			body.CPM = *sd.Lnd.U7318
			usvConversionFactor = 334
		}
		if sd.Lnd.C7318 != nil {
			body.Model = "lnd7317"
			body.CPM = *sd.Lnd.C7318
			usvConversionFactor = 334
		}
		if sd.Lnd.EC7128 != nil {
			body.Model = "lnd7128"
			body.CPM = *sd.Lnd.EC7128
			usvConversionFactor = 108
		}
		if sd.Lnd.U712 != nil {
			body.Model = "lnd712"
			body.CPM = *sd.Lnd.U712
			usvConversionFactor = 108
		}
		if sd.Lnd.W78017 != nil {
			body.Model = "lnd78017"
			body.CPM = *sd.Lnd.W78017
		}
		if sd.Lnd.USv != nil {
			body.USV = *sd.Lnd.USv
		} else if usvConversionFactor != 0 {
			body.USV = body.CPM / float64(usvConversionFactor)
		}
	}

	if sd.Pms != nil {
		if sd.Pms.Pm01_0 != nil {
			body.Pm01_0 = *sd.Pms.Pm01_0
		}
		if sd.Pms.Pm02_5 != nil {
			body.Pm02_5 = *sd.Pms.Pm02_5
		}
		if sd.Pms.Pm10_0 != nil {
			body.Pm10_0 = *sd.Pms.Pm10_0
		}
		if sd.Pms.Count00_30 != nil {
			body.Count00_30 = *sd.Pms.Count00_30
		}
		if sd.Pms.Count00_50 != nil {
			body.Count00_50 = *sd.Pms.Count00_50
		}
		if sd.Pms.Count01_00 != nil {
			body.Count01_00 = *sd.Pms.Count01_00
		}
		if sd.Pms.Count02_50 != nil {
			body.Count02_50 = *sd.Pms.Count02_50
		}
		if sd.Pms.Count05_00 != nil {
			body.Count05_00 = *sd.Pms.Count05_00
		}
		if sd.Pms.Count10_00 != nil {
			body.Count10_00 = *sd.Pms.Count10_00
		}
		if sd.Pms.CountSecs != nil {
			body.CountSecs = *sd.Pms.CountSecs
		}
	}

	if sd.Env != nil {
		if sd.Env.Temp != nil {
			body.Temp = sd.Env.Temp
		}
		if sd.Env.Humid != nil {
			body.Humid = sd.Env.Humid
		}
		if sd.Env.Press != nil {
			body.Press = sd.Env.Press
		}
	}

	if sd.Bat != nil {
		if sd.Bat.Voltage != nil {
			body.Voltage = sd.Bat.Voltage
		}
		if sd.Bat.Charging != nil {
			body.Charging = sd.Bat.Charging
		}
		if sd.Bat.Line != nil {
			body.USB = sd.Bat.Line
		}
	}

	if sd.Loc != nil {
		if sd.Loc.Lat != nil {
			event.WhereLat = float64(*sd.Loc.Lat)
		}
		if sd.Loc.Lon != nil {
			event.WhereLon = float64(*sd.Loc.Lon)
		}
		if sd.Loc.Lat != nil && sd.Loc.Lon != nil {
			event.WhereWhen = time.Now().UTC().Unix()
		}
	}

	if sd.Dev != nil {
		if sd.Dev.Indoors != nil {
			body.Indoors = sd.Dev.Indoors
		}
		if sd.Dev.Temp != nil {
			body.Temp = sd.Dev.Temp
		}
		if sd.Dev.Humid != nil {
			body.Humid = sd.Dev.Humid
		}
		if sd.Dev.Press != nil {
			body.Press = sd.Dev.Press
		}
		if sd.Dev.Rat != nil {
			event.Rat = *sd.Dev.Rat
		}
		if sd.Dev.Bars != nil {
			event.Bars = *sd.Dev.Bars
		}
	}

	event.When = time.Now().UTC().Unix()
	if sd.CapturedAt != nil {
		parsedTime, err := time.Parse(time.RFC3339, *sd.CapturedAt)
		if err == nil {
			event.When = parsedTime.Unix()
		}
	}

	if sd.DeviceContactName != "" || sd.DeviceContactOrg != "" || sd.DeviceContactRole != "" || sd.DeviceContactEmail != "" {
		details := map[string]interface{}{}
		if sd.DeviceContactName != "" {
			details["name"] = sd.DeviceContactName
		}
		if sd.DeviceContactOrg != "" {
			details["org"] = sd.DeviceContactOrg
		}
		if sd.DeviceContactRole != "" {
			details["role"] = sd.DeviceContactRole
		}
		if sd.DeviceContactEmail != "" {
			details["email"] = sd.DeviceContactEmail
		}
		event.Details = &details
	}

	event.DeviceUID = sd.DeviceUID
	event.DeviceSN = sd.DeviceSN
	event.NotefileID = "_air.qo"

	var bodyJSON []byte
	bodyJSON, err = json.Marshal(body)
	if err != nil {
		return
	}
	if string(bodyJSON) == "{}" {
		err = fmt.Errorf("no data")
		return
	}
	var bodyObj map[string]interface{}
	err = json.Unmarshal(bodyJSON, &bodyObj)
	if err != nil {
		return
	}
	event.Body = &bodyObj

	eventJSON, err = json.Marshal(event)

	deviceUID = event.DeviceUID
	return

}
