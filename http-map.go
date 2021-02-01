// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the "/device/<device_urn>" HTTP topic
package main

import (
	"fmt"
	"io"
	"net/http"
)

// Handle inbound HTTP requests to redirect to a map page for the class of this device
func inboundWebMAPHandler(rw http.ResponseWriter, req *http.Request) {
	stats.Count.HTTP++

	// Extract the deviceUID
	deviceUID := req.RequestURI[len(TTServerTopicMAP):]

	// Read the device status
	isAvail, isReset, ds := ReadDeviceStatus(deviceUID)
	if !isAvail || isReset {
		io.WriteString(rw, "device is not recognized")
		return
	}

	// Construct a redirect URL based upon the class of the device
	url := ""
	switch ds.DeviceClass {

	case "geigiecast":
		fallthrough
	case "pointcast":
		fallthrough
	case "safecast-air":
		fallthrough
	case "ngeigie":
		fallthrough
	case "":
		url = "https://map.safecast.org"

	case "product:org.airnote.solar.rad.v1":
		fallthrough
	case "product:org.airnote.solar.air.v1":
		fallthrough
	case "product:com.blues.airnote":
		fallthrough
	case "product:org.airnote.solar.v1":
		url = "https://grafana.safecast.cc/d/t_Z6DlbGz/safecast-all-airnotes?orgId=1"

	default:
		io.WriteString(rw, fmt.Sprintf("class %s for device %s is not recognized", ds.DeviceClass, deviceUID))
		return

	}

	// Perform the redirect
	http.Redirect(rw, req, url, http.StatusTemporaryRedirect)

}
