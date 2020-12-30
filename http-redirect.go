// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound support for the HTTP V1 safecast topic
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

// Debugging
const redirectDebug bool = false

// Handle inbound HTTP requests from the Teletype Gateway
func inboundWebRedirectHandler(rw http.ResponseWriter, req *http.Request) {
	var sdV1 *SafecastDataV1
	var sdV1Emit *SafecastDataV1ToEmit
	var body []byte
	var err error

	// Remember when it was uploaded to us
	UploadedAt := NowInUTC()

	// Process the request URI, looking for things that will indicate "dev"
	method := req.Method
	if method == "" {
		method = "GET"
	}

	// See if this is a test measurement
	isTestMeasurement := strings.Contains(req.RequestURI, "test")

	// Get the remote address, and only add this to the count if it's likely from
	// the internal HTTP load balancer.
	_, isReal, abusive := getRequestorIPv4(req)
	if abusive {
		return
	}

	// If this is a GET (a V1 Pointcast 3G), convert RequestURI into valid json
	RequestURI := req.RequestURI
	if method == "GET" {
		// Before: /scripts/shorttest.php?api_key=q1LKu7RQ8s5pmyxunnDW&lat=34.4883&lon=136.165&cpm=0&id=100031&alt=535
		//  After: {"unit":"cpm","latitude":"34.4883","longitude":"136.165","value":"0","device_id":"100031","height":"535"}
		str1 := strings.SplitN(RequestURI, "&", 2)
		RequestURI = str1[0]
		if len(str1) == 1 {
			body = []byte("")
		} else {
			str2 := str1[len(str1)-1]
			str3 := "unit=cpm&" + str2
			str4 := strings.Replace(str3, "lat=", "latitude=", 1)
			str5 := strings.Replace(str4, "lon=", "longitude=", 1)
			str6 := strings.Replace(str5, "alt=", "height=", 1)
			str7 := strings.Replace(str6, "cpm=", "value=", 1)
			str8 := strings.Replace(str7, "id=", "device_id=", 1)
			str9 := strings.Replace(str8, "typ=", "devicetype_id=", 1)
			str10 := strings.Replace(str9, "=", "\":\"", -1)
			str11 := strings.Replace(str10, "&", "\",\"", -1)
			body = []byte("{\"" + str11 + "\"}")
		}

	} else {

		// Read the body as a byte array
		body, err = ioutil.ReadAll(req.Body)
		if err != nil {
			stats.Count.HTTP++
			fmt.Printf("Error reading HTTP request body: \n%v\n", req)
			return

		}
	}

	// Clean up the json.  Specifically, Device ID 100049 puts a newline into
	// the devietype_id string literal, which is choked on by the JSON parser
	cleanBodyStr := string(body)
	cleanBodyStr = strings.Replace(cleanBodyStr, "\n", "", -1)
	cleanBodyStr = strings.Replace(cleanBodyStr, "\r", "", -1)
	cleanBodyStr = strings.Replace(cleanBodyStr, "\\r", "", -1)
	cleanBodyStr = strings.Replace(cleanBodyStr, "\":\" ", "\":\"", -1)
	cleanBody := []byte(cleanBodyStr)

	// Decode the request with custom marshaling
	sdV1, sdV1Emit, err = SafecastV1Decode(bytes.NewReader(cleanBody))
	if err != nil {
		stats.Count.HTTP++

		// Eliminate a bit of the noise caused by load balancer health checks
		if isReal && req.RequestURI != "/" && req.RequestURI != "/favicon.ico" {

			// See if this is nothing but a device ID
			deviceUID := req.RequestURI[len("/"):]
			file := GetDeviceStatusFilePath(deviceUID)
			contents, err := ioutil.ReadFile(file)
			if err == nil {
				GenerateDeviceSummaryWebPage(rw, contents)
				return
			}
			io.WriteString(rw, fmt.Sprintf("Unknown device: %s\n", deviceUID))
			return

		}

		io.WriteString(rw, fmt.Sprintf("Live Free or Die.\n"))
		return

	}

	// A real request
	stats.Count.HTTP++

	// Fill in the minimum defaults
	if sdV1.Unit == nil {
		s := "cpm"
		sdV1.Unit = &s
		sdV1Emit.Unit = &s
	}
	if sdV1.Value == nil {
		f64 := float64(0)
		sdV1.Value = &f64
		str := fmt.Sprintf("%f", f64)
		sdV1Emit.Value = &str
	}
	if sdV1.CapturedAt == nil {
		capturedAt := NowInUTC()
		sdV1.CapturedAt = &capturedAt
		sdV1Emit.CapturedAt = &capturedAt
	}

	// Debugging on 2017-06-24 with Rob; feel free to delete after 2017-07-01 if it's still here
	if false {
		if sdV1.DeviceID != nil {
			devicetype, _, _ := SafecastV1DeviceType(*sdV1.DeviceID)
			if devicetype == "safecast-air" {
				fmt.Printf("*** DeviceID %d %t %s\n", *sdV1.DeviceID, isTestMeasurement, req.RequestURI)
			}
		}
	}

	// Convert it to text to emit
	sdV1EmitJSON, _ := json.Marshal(sdV1Emit)

	// If debugging, display it
	if redirectDebug {
		fmt.Printf("\n\n*** Redirect %s test:%v %s\n", method, isTestMeasurement, req.RequestURI)
		fmt.Printf("*** Redirect received:\n%s\n", string(cleanBody))
		fmt.Printf("*** Redirect decoded to V1:\n%s\n", sdV1EmitJSON)
	}

	// For backward compatibility,post it to V1 with an URL that is preserved.  Also do normal post
	_, result := SafecastV1Upload(sdV1EmitJSON, RequestURI, isTestMeasurement, *sdV1.Unit, fmt.Sprintf("%.3f", *sdV1.Value))

	// Send a reply to Pointcast saying that the request was processed acceptably.
	// If we fail to do this, Pointcast goes into an infinite reboot loop with comms errors
	// due to GetMeasurementReply() returning 0.
	io.WriteString(rw, result)

	// Convert to current data format
	deviceID, deviceType, sd := SafecastReformatFromV1(sdV1, isTestMeasurement)
	if deviceID == 0 {
		requestor, _, abusive := getRequestorIPv4(req)
		if abusive {
			return
		}
		transportStr := deviceType + ":" + requestor
		fmt.Printf("\n%s ** Ignoring message with DeviceID == 0 from %s:\n%s\n", LogTime(), transportStr, string(cleanBody))
		return
	}

	// If debugging, display it
	if redirectDebug {
		scJSON, _ := json.Marshal(sd)
		fmt.Printf("*** Redirect reformatted to V2:\n%s\n\n\n", scJSON)
	}

	// Report where we got it from, and when we got it
	var svc Service
	svc.UploadedAt = &UploadedAt
	requestor, _, abusive := getRequestorIPv4(req)
	if abusive {
		return
	}
	transportStr := deviceType + ":" + requestor
	svc.Transport = &transportStr
	sd.Service = &svc

	fmt.Printf("\n%s Received payload for %d from %s\n", LogTime(), sd.DeviceUID, transportStr)
	fmt.Printf("%s\n", cleanBody)

	// If the data doesn't have anything useful in it, optimize it completely away.  This is
	// observed to happen for Safecast Air from time to time
	if sd.Opc == nil && sd.Pms == nil && sd.Pms2 == nil && sd.Env == nil && sd.Lnd == nil && sd.Bat == nil && sd.Dev == nil {
		fmt.Printf("%s *** Ignoring because message contains no data\n", LogTime())
		return
	}

	// Generate the CRC of the original device data
	hash := HashSafecastData(sd)
	sd.Service.HashMd5 = &hash

	// Add info about the server instance that actually did the upload
	sd.Service.Handler = &TTServeInstanceID

	// Post to V2
	Upload(sd)
	WriteToLogs(sd)
	stats.Count.HTTPRedirect++

}
