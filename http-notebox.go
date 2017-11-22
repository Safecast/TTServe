// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the routing from a notebox
package main

import (
    "io/ioutil"
    "net/http"
    "fmt"
    "encoding/json"
	"crypto/md5"
)

// Handle inbound HTTP requests from Notebox's via the Notehub reporter task
func inboundWebNoteboxHandler(rw http.ResponseWriter, req *http.Request) {
    var body []byte
    var err error

    // Remember when it was uploaded to us
    UploadedAt := NowInUTC()

    // Get the remote address, and only add this to the count if it's likely from
    // the internal HTTP load balancer.
    remoteAddr, isReal := getRequestorIPv4(req)
    if !isReal {
        remoteAddr = "internal address"
    }

    // Read the body as a byte array
    body, err = ioutil.ReadAll(req.Body)
    if err != nil {
        stats.Count.HTTP++
        fmt.Printf("Error reading HTTP request body: \n%v\n", req)
        return

    }

    // Parse it into an array of SafecastData structures
    set := []SafecastData{}
    err = json.Unmarshal(body, &set)
    if err != nil {
        fmt.Printf("*** %s cannot parse received this from %s: %s\n%s\n***\n", UploadedAt, remoteAddr, err, string(body))
        return
    }

    // Process each reading individually
    for _, sd := range set {

        err = ReformatFromNotebox(UploadedAt, &sd)
        if err != nil {
            fmt.Printf("*** cannot format incoming data from notebox: %sn", UploadedAt, remoteAddr, err)
        }

        // Report where we got it from, and when we got it
        var svc Service
        svc.UploadedAt = &UploadedAt
        transportStr := "notebox:" + remoteAddr
        svc.Transport = &transportStr
        sd.Service = &svc

        // If the data doesn't have anything useful in it, optimize it completely away.  This is
        // observed to happen for Safecast Air from time to time
        if sd.Opc == nil && sd.Pms == nil && sd.Env == nil && sd.Lnd == nil && sd.Bat == nil && sd.Dev == nil {
            fmt.Printf("%s *** Ignoring because message contains no data\n", LogTime())
            return
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

    }

    // A real request
    stats.Count.HTTP++

}

// ReformatFromNotebox reformats to our standard normalized data format
func ReformatFromNotebox(uploadedAt string, sd *SafecastData) (err error) {

    // Mark it as a test measurement
    if sd.Dev == nil {
        var dev Dev
        sd.Dev = &dev
    }
	isTest := true
    sd.Dev.Test = &isTest

	// Convert from a device name to device number
	if sd.DeviceUID == nil {
		err = fmt.Errorf("missing device UID")
		return;
	}
	hash := md5.Sum([]byte(*sd.DeviceUID))
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
	sd.DeviceUID = nil
	sd.DeviceID = &deviceID

	// Done
	return
	
}