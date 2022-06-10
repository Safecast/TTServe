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

// Types of redirect page
const (
	deviceDashboard = iota
	deviceMap
	deviceProfile
)

// Handle inbound HTTP requests to redirect to the appropriate page
func inboundWebIDHandler(rw http.ResponseWriter, req *http.Request) {
	webPageRedirectHandler(rw, req, req.RequestURI[len(TTServerTopicDashboard):], deviceDashboard)
}
func inboundWebDashboardHandler(rw http.ResponseWriter, req *http.Request) {
	webPageRedirectHandler(rw, req, req.RequestURI[len(TTServerTopicDashboard):], deviceDashboard)
}
func inboundWebMapHandler(rw http.ResponseWriter, req *http.Request) {
	webPageRedirectHandler(rw, req, req.RequestURI[len(TTServerTopicMap):], deviceMap)
}
func inboundWebProfileHandler(rw http.ResponseWriter, req *http.Request) {
	webPageRedirectHandler(rw, req, req.RequestURI[len(TTServerTopicProfile):], deviceProfile)
}

// Handle inbound HTTP requests to redirect to a certain page
func webPageRedirectHandler(rw http.ResponseWriter, req *http.Request, deviceUID string, pageType int) {
	stats.Count.HTTP++

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
		switch pageType {
		case deviceDashboard:
			url = "https://grafana.safecast.cc/d/DFSxrOLWk/safecast-device-details?orgId=1&from=now-7d&to=now&refresh=15m&var-device_urn=" + deviceUID
		case deviceMap:
			url = "https://map.safecast.org"
		case deviceProfile:
			url = "https://api.safecast.org/en-US/device_stories/" + deviceUID
		}

	case "product:net.ozzie.ray:radnote":
		fallthrough
	case "product:org.airnote.solar.rad.v1":
		switch pageType {
		case deviceDashboard:
			url = "https://grafana.safecast.cc/d/ndnJJuYMk/safecast-radnote?orgId=1&var-device_urn=" + deviceUID
		case deviceMap:
			url = "https://grafana.safecast.cc/d/t_Z6DlbGz/safecast-all-airnotes?orgId=1"
		case deviceProfile:
			url = "https://api.safecast.org/en-US/device_stories/" + deviceUID
		}

	case "product:org.airnote.solar.air.v1":
		fallthrough
	case "product:com.blues.airnote":
		fallthrough
	case "product:org.airnote.solar.v1":
		switch pageType {
		case deviceDashboard:
			url = "https://grafana.safecast.cc/d/7wsttvxGk/airnote-device-details?orgId=1&var-device_urn=" + deviceUID
		case deviceMap:
			url = "https://grafana.safecast.cc/d/t_Z6DlbGz/safecast-all-airnotes?orgId=1"
		case deviceProfile:
			url = "https://api.safecast.org/en-US/device_stories/" + deviceUID
		}

	default:
		io.WriteString(rw, fmt.Sprintf("class %s for device %s is not recognized", ds.DeviceClass, deviceUID))
		return

	}

	// Perform the redirect
	http.Redirect(rw, req, url, http.StatusTemporaryRedirect)

}
