// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the routing from a note
package main

import (
    "fmt"
	"time"
    "io/ioutil"
    "net/http"
    "encoding/json"
	"crypto/md5"
)

// Schemas for the different file types
type sensorINA struct {
	Voltage float32		`json:"voltage,omitempty"`
    Current float32		`json:"current,omitempty"`
}
type sensorBME struct {
	Temperature float32	`json:"temp,omitempty"`
    Humidity float32	`json:"humid,omitempty"`
    Pressure float32	`json:"press,omitempty"`
}
type sensorRAD struct {
	CPM float32			`json:"cpm,omitempty"`
    Seconds uint32		`json:"secs,omitempty"`
}
type sensorAIR struct {
	Pm01_0 float32		`json:"pm01_0,omitempty"`
	Pm02_5 float32		`json:"pm02_5,omitempty"`
	Pm10_0 float32		`json:"pm10_0,omitempty"`
	Count00_30 uint32	`json:"c00_30,omitempty"`
	Count00_50 uint32	`json:"c00_50,omitempty"`
	Count01_00 uint32	`json:"c01_00,omitempty"`
	Count02_50 uint32	`json:"c02_50,omitempty"`
	Count05_00 uint32	`json:"c05_00,omitempty"`
	Count10_00 uint32	`json:"c10_00,omitempty"`
    CountSecs uint32	`json:"csecs,omitempty"`
}
	
// Handle inbound HTTP requests from Note's via the Notehub reporter task
func inboundWebNoteHandler(rw http.ResponseWriter, req *http.Request) {
    var body []byte
    var err error
	
    // Count the request
    stats.Count.HTTP++

    // Get the remote address, and only add this to the count if it's likely from
    // the internal HTTP load balancer.
    remoteAddr, isReal := getRequestorIPv4(req)
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
    e := NoteEvent{}
    err = json.Unmarshal(body, &e)
	if err != nil {
		return
	}

	// Convert to Safecast data, and exit if failure
    sd, err := noteToSD(e, transportStr)
    if err != nil {
		fmt.Printf("NOTE ignored: %s\n%s\n", err, body);
		return
    }

	// Display info about it
    fmt.Printf("\n%s Received payload for %d %s from %s in %s\n", LogTime(), sd.DeviceID, *sd.DeviceURN, transportStr,
		e.TowerLocation+" "+e.TowerCountry)

	fmt.Printf("%s TRANSFORMED INTO:\n", body);
	var sdJSON []byte
	sdJSON, err = json.Marshal(sd)
	if err == nil {
		fmt.Printf("%s\n", sdJSON)
	}

	// Send it to Safecast
	go SendToSafecast(sd)

}

// ReformatFromNote reformats to our standard normalized data format
func noteToSD(e NoteEvent, transport string) (sd SafecastData, err error) {

    // Mark it as a test measurement
	isTest := true
    var dev Dev
    sd.Dev = &dev
    sd.Dev.Test = &isTest

	// Generate device name
	deviceURN := "note:" + e.DeviceUID
	sd.DeviceURN = &deviceURN

	// Generate backward-compatible safecast Device ID, reserving the low 2^20 addresses
	// for fixed allocation as per Rob agreement (see ttnode/src/io.c)
	hash := md5.Sum([]byte(*sd.DeviceURN))
	var deviceID uint32 = 0
	for i:=0; i<len(hash); i++ {
		x := uint32(hash[i]) << (uint(i) % 4)
		deviceID = deviceID ^ x
	}
    if (deviceID < 1048576) {
        deviceID = ^deviceID;
	}
	sd.DeviceID = &deviceID

	// Service-related
    var svc Service
    sd.Service = &svc
    svc.Transport = &transport
    uploadedAt := NowInUTC()		// OZZIE should be in message
	svc.UploadedAt = &uploadedAt

	// When captured on the device
	if e.When != 0 {
		capturedAt := time.Unix(e.When, 0).Format("2006-01-02T15:04:05Z")
		sd.CapturedAt = &capturedAt
	}

	// Where captured
	if e.Where != "" {
	    var loc Loc
	    sd.Loc = &loc
		var lat, lon float32
		lat = float32(e.WhereLat)
		lon = float32(e.WhereLon)
		sd.Loc.Lat = &lat
		sd.Loc.Lon = &lon
		sd.Loc.Olc = &e.Where
	}

	// If there's no body, bail
	if e.Body == nil {
		err = fmt.Errorf("no sensor data")
		return
	}
	var sensorJSON []byte
	sensorJSON, err = json.Marshal(e.Body)
	if err != nil {
		return
	}

	// Decompose the body with a per-notefile schema
	switch e.NotefileID {

	case "bat-ina219.qo":
	    s := sensorINA{}
	    err = json.Unmarshal(sensorJSON, &s)
		if err != nil {
			return
		}
	    var bat Bat
		bat.Voltage = &s.Voltage
		bat.Current = &s.Current
	    sd.Bat = &bat
		
	case "air-bme280.qo":
	    s := sensorBME{}
	    err = json.Unmarshal(sensorJSON, &s)
		if err != nil {
			return
		}
	    var env Env
		env.Temp = &s.Temperature
		env.Humid = &s.Humidity
		env.Press = &s.Pressure
	    sd.Env = &env
		
	case "rad0-lnd7318u.qo":
		s := sensorRAD{}
	    err = json.Unmarshal(sensorJSON, &s)
		if err != nil {
			return
		}
	    var lnd Lnd
	    lnd.C7318 = &s.CPM
		sd.Lnd = &lnd

	case "rad1-lnd7128ec.qo":
		s := sensorRAD{}
	    err = json.Unmarshal(sensorJSON, &s)
		if err != nil {
			return
		}
	    var lnd Lnd
	    lnd.EC7128 = &s.CPM
		sd.Lnd = &lnd

	case "aq0-pms5003.qo":
		s := sensorAIR{}
	    err = json.Unmarshal(sensorJSON, &s)
		if err != nil {
			return
		}
	    var pms Pms
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
		sd.Pms = &pms

	case "aq1-pms5003.qo":
		s := sensorAIR{}
	    err = json.Unmarshal(sensorJSON, &s)
		if err != nil {
			return
		}
	    var pms Pms2
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
		sd.Pms2 = &pms

	default:
		err = fmt.Errorf("no sensor data in file %s", e.NotefileID)
		return

	}

	// Done
	return
	
}
