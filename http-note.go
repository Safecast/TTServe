// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the routing from a note
package main

import (
    "io/ioutil"
    "net/http"
    "fmt"
    "encoding/json"
	"crypto/md5"
    "github.com/google/open-location-code/go"
)
	
// Handle inbound HTTP requests from Note's via the Notehub reporter task
func inboundWebNoteHandler(rw http.ResponseWriter, req *http.Request) {
    var body []byte
    var err error

    // Get the remote address, and only add this to the count if it's likely from
    // the internal HTTP load balancer.
    remoteAddr, isReal := getRequestorIPv4(req)
    if !isReal {
        remoteAddr = "internal address"
    }
    transportStr := "note:" + remoteAddr
    uploadedAt := NowInUTC()

    // Read the body as a byte array
    body, err = ioutil.ReadAll(req.Body)
    if err != nil {
        stats.Count.HTTP++
        return

    }

	// Exit if it's there's nothing there
	if len(body) == 0 {
		return
	}
	
	// Unmarshal into a notehub Event structure
    // Parse it into an array of SafecastData structures
    e := NoteEvent{}
    err = json.Unmarshal(body, &e)

	fmt.Printf("***************************************\n");
	fmt.Printf("***************************************\n");
	fmt.Printf("***************************************\n");
	fmt.Printf("from %s at %s\n", transportStr, uploadedAt)
	fmt.Printf("err: %s\n", err)
	fmt.Printf("%s\n", string(body))
	fmt.Printf("***************************************\n");
	fmt.Printf("***************************************\n");
	fmt.Printf("***************************************\n");

/*

    // Remember when it was uploaded to us

    // Process each reading individually
	uploaded := 0
    for _, sd := range set {

        err = ReformatFromNote(uploadedAt, &sd)
        if err != nil {
            emitError(rw, fmt.Errorf("cannot format incoming data from note: %s", err))
			return
        }

        // Report where we got it from, and when we got it
        var svc Service
        svc.UploadedAt = &uploadedAt
        svc.Transport = &transportStr
        sd.Service = &svc

        // If the data doesn't have anything useful in it, optimize it completely away.  This happens
		// with data points that have nothing to do with Safecast but are stored in the note DB
        if sd.Opc == nil && sd.Pms == nil && sd.Pms2 == nil && sd.Env == nil && sd.Lnd == nil && sd.Bat == nil {
            fmt.Printf("%s *** Ignoring because message contains no data\n", LogTime())
			continue
        }

        // Generate the CRC of the original device data
        hash := HashSafecastData(sd)
        sd.Service.HashMd5 = &hash

        // Add info about the server instance that actually did the upload
        sd.Service.Handler = &TTServeInstanceID

        // Debug
        fmt.Printf("\n%s Received payload for %d from %s\n", LogTime(), *sd.DeviceID, transportStr)
        scJSON, _ := json.Marshal(sd)
        fmt.Printf("%s\n", scJSON)

        // Post to V2
        Upload(sd)
        WriteToLogs(sd)
        stats.Count.HTTPRedirect++
		uploaded++

    }
*/
	
    // Count the request
    stats.Count.HTTP++

}

// ReformatFromNote reformats to our standard normalized data format
func ReformatFromNote(uploadedAt string, sd *SafecastData) (err error) {

    // Mark it as a test measurement
    if sd.Dev == nil {
        var dev Dev
        sd.Dev = &dev
    }
	isTest := true
    sd.Dev.Test = &isTest

	// Convert from a device name to device number
	if sd.DeviceURN == nil {
		err = fmt.Errorf("missing device URN")
		return;
	}
	hash := md5.Sum([]byte(*sd.DeviceURN))
	var deviceID uint32 = 0
	for i:=0; i<len(hash); i++ {
		x := uint32(hash[i]) << (uint(i) % 4)
		deviceID = deviceID ^ x
	}

	// Reserve the low 2^20 addresses for fixed allocation as per Rob agreement
	// (see ttnode/src/io.c)
    if (deviceID < 1048576) {
        deviceID = ^deviceID;
	}
	sd.DeviceID = &deviceID

	// Convert olc to lat/lon
	if sd.Loc != nil && sd.Loc.Olc != nil {
		ca, err2 := olc.Decode(*sd.Loc.Olc)
		if err2 != nil {
			err = err2
			return
		}
		lat64, lon64 := ca.Center()
		lat32 := float32(lat64)
		lon32 := float32(lon64)
		sd.Loc.Lat = &lat32
		sd.Loc.Lon = &lon32
	}

	// Done
	return
	
}
