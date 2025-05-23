// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Handling of "value files", which are aggregations of information
// observed as messages are routed through the TTSERVE pipline.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	ttdata "github.com/Safecast/safecast-go"
)

// DeviceStatus is the data structure for the "Device Status" files
type DeviceStatus struct {
	ttdata.SafecastData `json:"current_values,omitempty"`
	LocationHistory     [5]ttdata.SafecastData `json:"location_history,omitempty"`
	GeigerHistory       [5]ttdata.SafecastData `json:"geiger_history,omitempty"`
	OpcHistory          [5]ttdata.SafecastData `json:"opc_history,omitempty"`
	PmsHistory          [5]ttdata.SafecastData `json:"pms_history,omitempty"`
	Pms2History         [5]ttdata.SafecastData `json:"pms2_history,omitempty"`
	IPInfo              IPInfoData             `json:"transport_ip_info,omitempty"`
}

// ReadDeviceStatus gets the current value
func ReadDeviceStatus(deviceUID string) (isAvail bool, isReset bool, sv DeviceStatus) {
	valueEmpty := DeviceStatus{}
	valueEmpty.DeviceUID = deviceUID

	// Generate the filename, which we'll use twice
	filename := GetDeviceStatusFilePath(deviceUID)

	// If the file doesn't exist, don't even try
	_, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// We did not reinitialize it - it's truly empty.
			return true, false, valueEmpty
		}
		return false, true, valueEmpty
	}

	// Try reading the file several times, now that we know it exists.
	// We retry just in case of file system errors on contention.
	for i := 0; i < 5; i++ {

		// Read the file and unmarshall if no error
		contents, errRead := os.ReadFile(filename)
		if errRead == nil {
			valueToRead := DeviceStatus{}
			errRead = json.Unmarshal(contents, &valueToRead)
			if errRead == nil {
				return true, false, valueToRead
			}
			// Malformed JSON can easily occur because of multiple concurrent
			// writers, and so this self-corrects the situation.
			fmt.Printf("*** %s appears to be corrupt *** (%s)\n", filename, errRead)
			return true, true, valueEmpty
		}
		err = errRead

		// Delay before trying again
		time.Sleep(5 * time.Second)

	}

	// Error
	if os.IsNotExist(err) {
		return true, true, valueEmpty
	}
	return false, true, valueEmpty

}

// WriteDeviceStatus saves the last value in a file
func WriteDeviceStatus(sc ttdata.SafecastData) {
	var ChangedLoc = false
	var ChangedPms = false
	var ChangedPms2 = false
	var ChangedOpc = false
	var ChangedGeiger = false
	var value DeviceStatus

	// Delay a random amount just in case we get called very quickly
	// with two sequential values by the same device.  While no guarantee,
	// this reduces the chance that we will overwrite each other.
	// This happens ALL THE TIME when there are multiple LoRa gateways
	// that receive and upload the same message from the same device,
	// and are typically received by different TTSERVE instances because
	// of load balancing.  This simply reduces the possibility of
	// file corruption due to multiple concurrent writers.  (The corruption
	// is self-correcting, but it's still good to avoid.)
	sleepSeconds := Random(0, 30)
	time.Sleep(time.Duration(sleepSeconds) * time.Second)

	// Use the supplied upload time as our modification time
	if sc.Service == nil {
		var svc ttdata.Service
		sc.Service = &svc
	}

	// Read the current value, or a blank value structure if it's blank.
	// If the value isn't available it's because of a nonrecoverable  error.
	// If it was reset, try waiting around a bit until it is fixed.
	for i := 0; i < 4; i++ {
		isAvail, isReset, rvalue := ReadDeviceStatus(sc.DeviceUID)
		value = rvalue
		if isAvail && !isReset {
			break
		}
		time.Sleep(time.Duration(Random(1, 6)) * time.Second)
	}

	// Update the identity-related fields that are unconditionally specified
	value.DeviceUID = sc.DeviceUID
	value.DeviceClass = sc.DeviceClass
	value.DeviceSN = sc.DeviceSN
	value.DeviceID = sc.DeviceID
	value.DeviceContactName = sc.DeviceContactName
	value.DeviceContactOrg = sc.DeviceContactOrg
	value.DeviceContactRole = sc.DeviceContactRole
	value.DeviceContactEmail = sc.DeviceContactEmail

	// Copy extra info from the sheet
	if value.DeviceID != 0 || value.DeviceSN != "" {
		si, err := sheetDeviceInfo(value.DeviceID, value.DeviceSN)
		if err == nil {
			if si.Custodian != "" || si.CustodianContact != "" {
				if si.Custodian != "" {
					value.DeviceContactName = si.Custodian
				}
				if si.CustodianContact != "" {
					value.DeviceContactEmail = si.CustodianContact
				}
			}
			if si.Dashboard != "" {
				if value.Dev == nil {
					value.Dev = &(ttdata.Dev{})
				}
				value.Dev.Dashboard = &si.Dashboard
			}
		}
	}

	// Generate a synthetic dashboard url
	notePrefix := "note:dev:"
	if strings.HasPrefix(value.DeviceUID, notePrefix) {
		if value.Dev == nil {
			value.Dev = &(ttdata.Dev{})
		}
		if value.Dev.Dashboard == nil {
			s := ""
			value.Dev.Dashboard = &s
		}
		if *value.Dev.Dashboard == "" {
			url := "http://qrgo.org/ID/" + strings.TrimPrefix(value.DeviceUID, notePrefix)
			value.Dev.Dashboard = &url
		}
	}

	// Update the current values, but only if modified
	if sc.Service != nil && sc.Service.UploadedAt != nil {
		if value.Service == nil {
			var svc ttdata.Service
			value.Service = &svc
		}
		value.Service.UploadedAt = sc.Service.UploadedAt
	}
	if sc.CapturedAt != nil {
		value.CapturedAt = sc.CapturedAt
	}
	if sc.Bat != nil {
		var bat ttdata.Bat
		if value.Bat == nil {
			value.Bat = &bat
		}
		if sc.Voltage != nil {
			value.Bat.Voltage = sc.Bat.Voltage
		}
		if sc.Current != nil {
			value.Bat.Current = sc.Bat.Current
		}
		if sc.Charge != nil {
			value.Bat.Charge = sc.Bat.Charge
		}
	}
	if sc.Env != nil {
		var env ttdata.Env
		if value.Env == nil {
			value.Env = &env
		}
		if sc.Env.Temp != nil {
			value.Env.Temp = sc.Env.Temp
		}
		if sc.Env.Humid != nil {
			value.Env.Humid = sc.Env.Humid
		}
		if sc.Env.Press != nil {
			value.Env.Press = sc.Env.Press
		}
	}
	if sc.Gateway != nil {
		var gate ttdata.Gateway
		if value.Gateway == nil {
			value.Gateway = &gate
		}
		if sc.Gateway.SNR != nil {
			value.Gateway.SNR = sc.Gateway.SNR
		}
		if sc.Gateway.ReceivedAt != nil {
			value.Gateway.ReceivedAt = sc.Gateway.ReceivedAt
		}
		if sc.Gateway.Lat != nil {
			value.Gateway.Lat = sc.Gateway.Lat
		}
		if sc.Gateway.Lon != nil {
			value.Gateway.Lon = sc.Gateway.Lon
		}
		if sc.Gateway.Alt != nil {
			value.Gateway.Alt = sc.Gateway.Alt
		}
	}
	if sc.Service != nil {
		var svc ttdata.Service
		if value.Service == nil {
			value.Service = &svc
		}
		if sc.Service.Transport != nil {
			value.Service.Transport = sc.Service.Transport
		}
	}
	if sc.Loc != nil && sc.Loc.Lat != nil && sc.Loc.Lon != nil {
		var loc ttdata.Loc
		if value.Loc == nil {
			value.Loc = &loc
		}
		var lat, lon float64
		if value.Loc.Lat == nil {
			value.Loc.Lat = &lat
		}
		if value.Loc.Lon == nil {
			value.Loc.Lon = &lon
		}
		var locname string
		if value.Loc.LocName == nil {
			value.Loc.LocName = &locname
		}
		// Don't overwrite a good lat/lon with an unsupplied lat/lon
		if *sc.Loc.Lat != 0.0 || *sc.Loc.Lon != 0.0 {
			value.Loc.Lat = sc.Loc.Lat
			value.Loc.Lon = sc.Loc.Lon
			value.Loc.LocName = sc.Loc.LocName
			value.Loc.LocCountry = sc.Loc.LocCountry
			value.Loc.LocZone = sc.Loc.LocZone
			if *value.Loc.Lat != *sc.Loc.Lat || *value.Loc.Lon != *sc.Loc.Lon {
				ChangedLoc = true
			}
			if sc.Loc.LocName != nil && *value.Loc.LocName != *sc.Loc.LocName {
				ChangedLoc = true
			}
		}
	}
	if sc.Pms != nil {
		var pms ttdata.Pms
		if value.Pms == nil {
			value.Pms = &pms
		}
		if sc.Pms.Pm01_0 != nil && (value.Pms.Pm01_0 == nil || *value.Pms.Pm01_0 != *sc.Pms.Pm01_0) {
			value.Pms.Pm01_0 = sc.Pms.Pm01_0
			ChangedPms = true
		}
		if sc.Pms.Pm02_5 != nil && (value.Pms.Pm02_5 == nil || *value.Pms.Pm02_5 != *sc.Pms.Pm02_5) {
			value.Pms.Pm02_5 = sc.Pms.Pm02_5
			ChangedPms = true
		}
		if sc.Pms.Pm10_0 != nil && (value.Pms.Pm10_0 == nil || *value.Pms.Pm10_0 != *sc.Pms.Pm10_0) {
			value.Pms.Pm10_0 = sc.Pms.Pm10_0
			ChangedPms = true
		}
		if sc.Pms.Std01_0 != nil && (value.Pms.Std01_0 == nil || *value.Pms.Std01_0 != *sc.Pms.Std01_0) {
			value.Pms.Std01_0 = sc.Pms.Std01_0
			ChangedPms = true
		}
		if sc.Pms.Std02_5 != nil && (value.Pms.Std02_5 == nil || *value.Pms.Std02_5 != *sc.Pms.Std02_5) {
			value.Pms.Std02_5 = sc.Pms.Std02_5
			ChangedPms = true
		}
		if sc.Pms.Std10_0 != nil && (value.Pms.Std10_0 == nil || *value.Pms.Std10_0 != *sc.Pms.Std10_0) {
			value.Pms.Std10_0 = sc.Pms.Std10_0
			ChangedPms = true
		}
		if sc.Pms.Count00_30 != nil && (value.Pms.Count00_30 == nil || *value.Pms.Count00_30 != *sc.Pms.Count00_30) {
			value.Pms.Count00_30 = sc.Pms.Count00_30
			ChangedPms = true
		}
		if sc.Pms.Count00_50 != nil && (value.Pms.Count00_50 == nil || *value.Pms.Count00_50 != *sc.Pms.Count00_50) {
			value.Pms.Count00_50 = sc.Pms.Count00_50
			ChangedPms = true
		}
		if sc.Pms.Count01_00 != nil && (value.Pms.Count01_00 == nil || *value.Pms.Count01_00 != *sc.Pms.Count01_00) {
			value.Pms.Count01_00 = sc.Pms.Count01_00
			ChangedPms = true
		}
		if sc.Pms.Count02_50 != nil && (value.Pms.Count02_50 == nil || *value.Pms.Count02_50 != *sc.Pms.Count02_50) {
			value.Pms.Count02_50 = sc.Pms.Count02_50
			ChangedPms = true
		}
		if sc.Pms.Count05_00 != nil && (value.Pms.Count05_00 == nil || *value.Pms.Count05_00 != *sc.Pms.Count05_00) {
			value.Pms.Count05_00 = sc.Pms.Count05_00
			ChangedPms = true
		}
		if sc.Pms.Count10_00 != nil && (value.Pms.Count10_00 == nil || *value.Pms.Count10_00 != *sc.Pms.Count10_00) {
			value.Pms.Count10_00 = sc.Pms.Count10_00
			ChangedPms = true
		}
		if sc.Pms.CountSecs != nil && (value.Pms.CountSecs == nil || *value.Pms.CountSecs != *sc.Pms.CountSecs) {
			value.Pms.CountSecs = sc.Pms.CountSecs
			ChangedPms = true
		}
	}
	if sc.Pms2 != nil {
		var pms2 ttdata.Pms2
		if value.Pms2 == nil {
			value.Pms2 = &pms2
		}
		if sc.Pms2.Pm01_0 != nil && (value.Pms2.Pm01_0 == nil || *value.Pms2.Pm01_0 != *sc.Pms2.Pm01_0) {
			value.Pms2.Pm01_0 = sc.Pms2.Pm01_0
			ChangedPms2 = true
		}
		if sc.Pms2.Pm02_5 != nil && (value.Pms2.Pm02_5 == nil || *value.Pms2.Pm02_5 != *sc.Pms2.Pm02_5) {
			value.Pms2.Pm02_5 = sc.Pms2.Pm02_5
			ChangedPms2 = true
		}
		if sc.Pms2.Pm10_0 != nil && (value.Pms2.Pm10_0 == nil || *value.Pms2.Pm10_0 != *sc.Pms2.Pm10_0) {
			value.Pms2.Pm10_0 = sc.Pms2.Pm10_0
			ChangedPms2 = true
		}
		if sc.Pms2.Std01_0 != nil && (value.Pms2.Std01_0 == nil || *value.Pms2.Std01_0 != *sc.Pms2.Std01_0) {
			value.Pms2.Std01_0 = sc.Pms2.Std01_0
			ChangedPms2 = true
		}
		if sc.Pms2.Std02_5 != nil && (value.Pms2.Std02_5 == nil || *value.Pms2.Std02_5 != *sc.Pms2.Std02_5) {
			value.Pms2.Std02_5 = sc.Pms2.Std02_5
			ChangedPms2 = true
		}
		if sc.Pms2.Std10_0 != nil && (value.Pms2.Std10_0 == nil || *value.Pms2.Std10_0 != *sc.Pms2.Std10_0) {
			value.Pms2.Std10_0 = sc.Pms2.Std10_0
			ChangedPms2 = true
		}
		if sc.Pms2.Count00_30 != nil && (value.Pms2.Count00_30 == nil || *value.Pms2.Count00_30 != *sc.Pms2.Count00_30) {
			value.Pms2.Count00_30 = sc.Pms2.Count00_30
			ChangedPms2 = true
		}
		if sc.Pms2.Count00_50 != nil && (value.Pms2.Count00_50 == nil || *value.Pms2.Count00_50 != *sc.Pms2.Count00_50) {
			value.Pms2.Count00_50 = sc.Pms2.Count00_50
			ChangedPms2 = true
		}
		if sc.Pms2.Count01_00 != nil && (value.Pms2.Count01_00 == nil || *value.Pms2.Count01_00 != *sc.Pms2.Count01_00) {
			value.Pms2.Count01_00 = sc.Pms2.Count01_00
			ChangedPms2 = true
		}
		if sc.Pms2.Count02_50 != nil && (value.Pms2.Count02_50 == nil || *value.Pms2.Count02_50 != *sc.Pms2.Count02_50) {
			value.Pms2.Count02_50 = sc.Pms2.Count02_50
			ChangedPms2 = true
		}
		if sc.Pms2.Count05_00 != nil && (value.Pms2.Count05_00 == nil || *value.Pms2.Count05_00 != *sc.Pms2.Count05_00) {
			value.Pms2.Count05_00 = sc.Pms2.Count05_00
			ChangedPms2 = true
		}
		if sc.Pms2.Count10_00 != nil && (value.Pms2.Count10_00 == nil || *value.Pms2.Count10_00 != *sc.Pms2.Count10_00) {
			value.Pms2.Count10_00 = sc.Pms2.Count10_00
			ChangedPms2 = true
		}
		if sc.Pms2.CountSecs != nil && (value.Pms2.CountSecs == nil || *value.Pms2.CountSecs != *sc.Pms2.CountSecs) {
			value.Pms2.CountSecs = sc.Pms2.CountSecs
			ChangedPms2 = true
		}
	}
	if sc.Opc != nil {
		var opc ttdata.Opc
		if value.Opc == nil {
			value.Opc = &opc
		}
		if sc.Opc.Pm01_0 != nil && (value.Opc.Pm01_0 == nil || *value.Opc.Pm01_0 != *sc.Opc.Pm01_0) {
			value.Opc.Pm01_0 = sc.Opc.Pm01_0
			ChangedOpc = true
		}
		if sc.Opc.Pm02_5 != nil && (value.Opc.Pm02_5 == nil || *value.Opc.Pm02_5 != *sc.Opc.Pm02_5) {
			value.Opc.Pm02_5 = sc.Opc.Pm02_5
			ChangedOpc = true
		}
		if sc.Opc.Pm10_0 != nil && (value.Opc.Pm10_0 == nil || *value.Opc.Pm10_0 != *sc.Opc.Pm10_0) {
			value.Opc.Pm10_0 = sc.Opc.Pm10_0
			ChangedOpc = true
		}
		if sc.Opc.Std01_0 != nil && (value.Opc.Std01_0 == nil || *value.Opc.Std01_0 != *sc.Opc.Std01_0) {
			value.Opc.Std01_0 = sc.Opc.Std01_0
			ChangedOpc = true
		}
		if sc.Opc.Std02_5 != nil && (value.Opc.Std02_5 == nil || *value.Opc.Std02_5 != *sc.Opc.Std02_5) {
			value.Opc.Std02_5 = sc.Opc.Std02_5
			ChangedOpc = true
		}
		if sc.Opc.Std10_0 != nil && (value.Opc.Std10_0 == nil || *value.Opc.Std10_0 != *sc.Opc.Std10_0) {
			value.Opc.Std10_0 = sc.Opc.Std10_0
			ChangedOpc = true
		}
		if sc.Opc.Count00_38 != nil && (value.Opc.Count00_38 == nil || *value.Opc.Count00_38 != *sc.Opc.Count00_38) {
			value.Opc.Count00_38 = sc.Opc.Count00_38
			ChangedOpc = true
		}
		if sc.Opc.Count00_54 != nil && (value.Opc.Count00_54 == nil || *value.Opc.Count00_54 != *sc.Opc.Count00_54) {
			value.Opc.Count00_54 = sc.Opc.Count00_54
			ChangedOpc = true
		}
		if sc.Opc.Count01_00 != nil && (value.Opc.Count01_00 == nil || *value.Opc.Count01_00 != *sc.Opc.Count01_00) {
			value.Opc.Count01_00 = sc.Opc.Count01_00
			ChangedOpc = true
		}
		if sc.Opc.Count02_10 != nil && (value.Opc.Count02_10 == nil || *value.Opc.Count02_10 != *sc.Opc.Count02_10) {
			value.Opc.Count02_10 = sc.Opc.Count02_10
			ChangedOpc = true
		}
		if sc.Opc.Count05_00 != nil && (value.Opc.Count05_00 == nil || *value.Opc.Count05_00 != *sc.Opc.Count05_00) {
			value.Opc.Count05_00 = sc.Opc.Count05_00
			ChangedOpc = true
		}
		if sc.Opc.Count10_00 != nil && (value.Opc.Count10_00 == nil || *value.Opc.Count10_00 != *sc.Opc.Count10_00) {
			value.Opc.Count10_00 = sc.Opc.Count10_00
			ChangedOpc = true
		}
		if sc.Opc.CountSecs != nil && (value.Opc.CountSecs == nil || *value.Opc.CountSecs != *sc.Opc.CountSecs) {
			value.Opc.CountSecs = sc.Opc.CountSecs
			ChangedOpc = true
		}
	}
	if sc.Lnd != nil {
		var lnd ttdata.Lnd
		if value.Lnd == nil {
			value.Lnd = &lnd
		}
		if sc.Lnd.U7318 != nil {
			var val float64
			if value.Lnd.U7318 == nil {
				value.Lnd.U7318 = &val
			}
			if *value.Lnd.U7318 != *sc.Lnd.U7318 {
				value.Lnd.U7318 = sc.Lnd.U7318
				ChangedGeiger = true
			}
		}
		if sc.Lnd.C7318 != nil {
			var val float64
			if value.Lnd.C7318 == nil {
				value.Lnd.C7318 = &val
			}
			if *value.Lnd.C7318 != *sc.Lnd.C7318 {
				value.Lnd.C7318 = sc.Lnd.C7318
				ChangedGeiger = true
			}
		}
		if sc.Lnd.EC7128 != nil {
			var val float64
			if value.Lnd.EC7128 == nil {
				value.Lnd.EC7128 = &val
			}
			if *value.Lnd.EC7128 != *sc.Lnd.EC7128 {
				value.Lnd.EC7128 = sc.Lnd.EC7128
				ChangedGeiger = true
			}
		}
		if sc.Lnd.U712 != nil {
			var val float64
			if value.Lnd.U712 == nil {
				value.Lnd.U712 = &val
			}
			if *value.Lnd.U712 != *sc.Lnd.U712 {
				value.Lnd.U712 = sc.Lnd.U712
				ChangedGeiger = true
			}
		}
		if sc.Lnd.W78017 != nil {
			var val float64
			if value.Lnd.W78017 == nil {
				value.Lnd.W78017 = &val
			}
			if *value.Lnd.W78017 != *sc.Lnd.W78017 {
				value.Lnd.W78017 = sc.Lnd.W78017
				ChangedGeiger = true
			}
		}
	}
	if sc.Dev != nil {
		var dev ttdata.Dev
		if value.Dev == nil {
			value.Dev = &dev
		}
		if sc.Dev.Test != nil {
			value.Dev.Test = sc.Dev.Test
		}
		if sc.Dev.Motion != nil {
			value.Dev.Motion = sc.Dev.Motion
		}
		if sc.Dev.DeviceLabel != nil {
			value.Dev.DeviceLabel = sc.Dev.DeviceLabel
		}
		if sc.Dev.UptimeMinutes != nil {
			value.Dev.UptimeMinutes = sc.Dev.UptimeMinutes
		}
		if sc.Dev.AppVersion != nil {
			value.Dev.AppVersion = sc.Dev.AppVersion
		}
		if sc.Dev.DeviceParams != nil {
			value.Dev.DeviceParams = sc.Dev.DeviceParams
		}
		if sc.Dev.ServiceParams != nil {
			value.Dev.ServiceParams = sc.Dev.ServiceParams
		}
		if sc.Dev.TtnParams != nil {
			value.Dev.TtnParams = sc.Dev.TtnParams
		}
		if sc.Dev.GpsParams != nil {
			value.Dev.GpsParams = sc.Dev.GpsParams
		}
		if sc.Dev.SensorParams != nil {
			value.Dev.SensorParams = sc.Dev.SensorParams
		}
		if sc.Dev.TransmittedBytes != nil {
			value.Dev.TransmittedBytes = sc.Dev.TransmittedBytes
		}
		if sc.Dev.ReceivedBytes != nil {
			value.Dev.ReceivedBytes = sc.Dev.ReceivedBytes
		}
		if sc.Dev.CommsResets != nil {
			value.Dev.CommsResets = sc.Dev.CommsResets
		}
		if sc.Dev.CommsFails != nil {
			value.Dev.CommsFails = sc.Dev.CommsFails
		}
		if sc.Dev.CommsPowerFails != nil {
			value.Dev.CommsPowerFails = sc.Dev.CommsPowerFails
		}
		if sc.Dev.DeviceRestarts != nil {
			value.Dev.DeviceRestarts = sc.Dev.DeviceRestarts
		}
		if sc.Dev.MotionEvents != nil {
			value.Dev.MotionEvents = sc.Dev.MotionEvents
		}
		if sc.Dev.Oneshots != nil {
			value.Dev.Oneshots = sc.Dev.Oneshots
		}
		if sc.Dev.OneshotSeconds != nil {
			value.Dev.OneshotSeconds = sc.Dev.OneshotSeconds
		}
		if sc.Dev.Iccid != nil {
			value.Dev.Iccid = sc.Dev.Iccid
		}
		if sc.Dev.Cpsi != nil {
			value.Dev.Cpsi = sc.Dev.Cpsi
		}
		if sc.Dev.Dfu != nil {
			value.Dev.Dfu = sc.Dev.Dfu
		}
		if sc.Dev.FreeMem != nil {
			value.Dev.FreeMem = sc.Dev.FreeMem
		}
		if sc.Dev.NTPCount != nil {
			value.Dev.NTPCount = sc.Dev.NTPCount
		}
		if sc.Dev.LastFailure != nil {
			value.Dev.LastFailure = sc.Dev.LastFailure
		}
		if sc.Dev.Status != nil {
			value.Dev.Status = sc.Dev.Status
		}
		if sc.Dev.ModuleLora != nil {
			value.Dev.ModuleLora = sc.Dev.ModuleLora
		}
		if sc.Dev.ModuleFona != nil {
			value.Dev.ModuleFona = sc.Dev.ModuleFona
		}
		if sc.Dev.Temp != nil {
			value.Dev.Temp = sc.Dev.Temp
		}
		if sc.Dev.Humid != nil {
			value.Dev.Humid = sc.Dev.Humid
		}
		if sc.Dev.Press != nil {
			value.Dev.Press = sc.Dev.Press
		}
		if sc.Dev.Rat != nil {
			value.Dev.Rat = sc.Dev.Rat
		}
		if sc.Dev.Bars != nil {
			value.Dev.Bars = sc.Dev.Bars
		}
		if sc.Dev.CommsAntFails != nil {
			value.Dev.CommsAntFails = sc.Dev.CommsAntFails
		}
		if sc.Dev.OvercurrentEvents != nil {
			value.Dev.OvercurrentEvents = sc.Dev.OvercurrentEvents
		}
		if sc.Dev.Seqno != nil {
			value.Dev.Seqno = sc.Dev.Seqno
		}
		if sc.Dev.Moved != nil {
			value.Dev.Moved = sc.Dev.Moved
		}
		if sc.Dev.Orientation != nil {
			value.Dev.Orientation = sc.Dev.Orientation
		}
		if sc.Dev.Dashboard != nil {
			value.Dev.Dashboard = sc.Dev.Dashboard
		}

		// Maximize error values that are supplied
		if sc.Dev.ErrorsOpc != nil {
			if value.Dev.ErrorsOpc == nil ||
				(value.Dev.ErrorsOpc != nil && *sc.Dev.ErrorsOpc > *value.Dev.ErrorsOpc) {
				value.Dev.ErrorsOpc = sc.Dev.ErrorsOpc
			}
		}
		if sc.Dev.ErrorsPms != nil {
			if value.Dev.ErrorsPms == nil ||
				(value.Dev.ErrorsPms != nil && *sc.Dev.ErrorsPms > *value.Dev.ErrorsPms) {
				value.Dev.ErrorsPms = sc.Dev.ErrorsPms
			}
		}
		if sc.Dev.ErrorsPms2 != nil {
			if value.Dev.ErrorsPms2 == nil ||
				(value.Dev.ErrorsPms2 != nil && *sc.Dev.ErrorsPms2 > *value.Dev.ErrorsPms2) {
				value.Dev.ErrorsPms2 = sc.Dev.ErrorsPms2
			}
		}
		if sc.Dev.ErrorsBme0 != nil {
			if value.Dev.ErrorsBme0 == nil ||
				(value.Dev.ErrorsBme0 != nil && *sc.Dev.ErrorsBme0 > *value.Dev.ErrorsBme0) {
				value.Dev.ErrorsBme0 = sc.Dev.ErrorsBme0
			}
		}
		if sc.Dev.ErrorsBme1 != nil {
			if value.Dev.ErrorsBme1 == nil ||
				(value.Dev.ErrorsBme1 != nil && *sc.Dev.ErrorsBme1 > *value.Dev.ErrorsBme1) {
				value.Dev.ErrorsBme1 = sc.Dev.ErrorsBme1
			}
		}
		if sc.Dev.ErrorsLora != nil {
			if value.Dev.ErrorsLora == nil ||
				(value.Dev.ErrorsLora != nil && *sc.Dev.ErrorsLora > *value.Dev.ErrorsLora) {
				value.Dev.ErrorsLora = sc.Dev.ErrorsLora
			}
		}
		if sc.Dev.ErrorsFona != nil {
			if value.Dev.ErrorsFona == nil ||
				(value.Dev.ErrorsFona != nil && *sc.Dev.ErrorsFona > *value.Dev.ErrorsFona) {
				value.Dev.ErrorsFona = sc.Dev.ErrorsFona
			}
		}
		if sc.Dev.ErrorsGeiger != nil {
			if value.Dev.ErrorsGeiger == nil ||
				(value.Dev.ErrorsGeiger != nil && *sc.Dev.ErrorsGeiger > *value.Dev.ErrorsGeiger) {
				value.Dev.ErrorsGeiger = sc.Dev.ErrorsGeiger
			}
		}
		if sc.Dev.ErrorsMax01 != nil {
			if value.Dev.ErrorsMax01 == nil ||
				(value.Dev.ErrorsMax01 != nil && *sc.Dev.ErrorsMax01 > *value.Dev.ErrorsMax01) {
				value.Dev.ErrorsMax01 = sc.Dev.ErrorsMax01
			}
		}
		if sc.Dev.ErrorsUgps != nil {
			if value.Dev.ErrorsUgps == nil ||
				(value.Dev.ErrorsUgps != nil && *sc.Dev.ErrorsUgps > *value.Dev.ErrorsUgps) {
				value.Dev.ErrorsUgps = sc.Dev.ErrorsUgps
			}
		}
		if sc.Dev.ErrorsTwi != nil {
			if value.Dev.ErrorsTwi == nil ||
				(value.Dev.ErrorsTwi != nil && *sc.Dev.ErrorsTwi > *value.Dev.ErrorsTwi) {
				value.Dev.ErrorsTwi = sc.Dev.ErrorsTwi
			}
		}
		if sc.Dev.ErrorsTwiInfo != nil {
			value.Dev.ErrorsTwiInfo = sc.Dev.ErrorsTwiInfo
		}
		if sc.Dev.ErrorsLis != nil {
			if value.Dev.ErrorsLis == nil ||
				(value.Dev.ErrorsLis != nil && *sc.Dev.ErrorsLis > *value.Dev.ErrorsLis) {
				value.Dev.ErrorsLis = sc.Dev.ErrorsLis
			}
		}
		if sc.Dev.ErrorsSpi != nil {
			if value.Dev.ErrorsSpi == nil ||
				(value.Dev.ErrorsSpi != nil && *sc.Dev.ErrorsSpi > *value.Dev.ErrorsSpi) {
				value.Dev.ErrorsSpi = sc.Dev.ErrorsSpi
			}
		}
		if sc.Dev.ErrorsMtu != nil {
			if value.Dev.ErrorsMtu == nil ||
				(value.Dev.ErrorsMtu != nil && *sc.Dev.ErrorsMtu > *value.Dev.ErrorsMtu) {
				value.Dev.ErrorsMtu = sc.Dev.ErrorsMtu
			}
		}
		if sc.Dev.ErrorsConnectLora != nil {
			if value.Dev.ErrorsConnectLora == nil ||
				(value.Dev.ErrorsConnectLora != nil && *sc.Dev.ErrorsConnectLora > *value.Dev.ErrorsConnectLora) {
				value.Dev.ErrorsConnectLora = sc.Dev.ErrorsConnectLora
			}
		}
		if sc.Dev.ErrorsConnectFona != nil {
			if value.Dev.ErrorsConnectFona == nil ||
				(value.Dev.ErrorsConnectFona != nil && *sc.Dev.ErrorsConnectFona > *value.Dev.ErrorsConnectFona) {
				value.Dev.ErrorsConnectFona = sc.Dev.ErrorsConnectFona
			}
		}
		if sc.Dev.ErrorsConnectWireless != nil {
			if value.Dev.ErrorsConnectWireless == nil ||
				(value.Dev.ErrorsConnectWireless != nil && *sc.Dev.ErrorsConnectWireless > *value.Dev.ErrorsConnectWireless) {
				value.Dev.ErrorsConnectWireless = sc.Dev.ErrorsConnectWireless
			}
		}
		if sc.Dev.ErrorsConnectGateway != nil {
			if value.Dev.ErrorsConnectGateway == nil ||
				(value.Dev.ErrorsConnectGateway != nil && *sc.Dev.ErrorsConnectGateway > *value.Dev.ErrorsConnectGateway) {
				value.Dev.ErrorsConnectGateway = sc.Dev.ErrorsConnectGateway
			}
		}
		if sc.Dev.ErrorsConnectData != nil {
			if value.Dev.ErrorsConnectData == nil ||
				(value.Dev.ErrorsConnectData != nil && *sc.Dev.ErrorsConnectData > *value.Dev.ErrorsConnectData) {
				value.Dev.ErrorsConnectData = sc.Dev.ErrorsConnectData
			}
		}
		if sc.Dev.ErrorsConnectService != nil {
			if value.Dev.ErrorsConnectService == nil ||
				(value.Dev.ErrorsConnectService != nil && *sc.Dev.ErrorsConnectService > *value.Dev.ErrorsConnectService) {
				value.Dev.ErrorsConnectService = sc.Dev.ErrorsConnectService
			}
		}

	}

	// Calculate a time of the shuffle, allowing for the fact that our preferred time
	// CapturedAt may not be available.
	now := NowInUTC()
	ShuffledAt := &now
	if value.CapturedAt != nil {
		ShuffledAt = value.CapturedAt
	}

	// Shuffle
	if ChangedLoc {
		for i := len(value.LocationHistory) - 1; i > 0; i-- {
			value.LocationHistory[i] = value.LocationHistory[i-1]
		}
		new := ttdata.SafecastData{}
		new.DeviceID = value.DeviceID
		new.DeviceUID = value.DeviceUID
		new.DeviceSN = value.DeviceSN
		new.CapturedAt = ShuffledAt
		new.Loc = value.Loc
		value.LocationHistory[0] = new
	}

	// Shuffle
	if ChangedPms {
		for i := len(value.PmsHistory) - 1; i > 0; i-- {
			value.PmsHistory[i] = value.PmsHistory[i-1]
		}
		new := ttdata.SafecastData{}
		new.DeviceID = value.DeviceID
		new.DeviceUID = value.DeviceUID
		new.DeviceSN = value.DeviceSN
		new.CapturedAt = ShuffledAt
		new.Pms = value.Pms
		value.PmsHistory[0] = new
	}

	// Shuffle
	if ChangedPms2 {
		for i := len(value.Pms2History) - 1; i > 0; i-- {
			value.Pms2History[i] = value.Pms2History[i-1]
		}
		new := ttdata.SafecastData{}
		new.DeviceID = value.DeviceID
		new.DeviceUID = value.DeviceUID
		new.DeviceSN = value.DeviceSN
		new.CapturedAt = ShuffledAt
		new.Pms2 = value.Pms2
		value.Pms2History[0] = new
	}

	// Shuffle
	if ChangedOpc {
		for i := len(value.OpcHistory) - 1; i > 0; i-- {
			value.OpcHistory[i] = value.OpcHistory[i-1]
		}
		new := ttdata.SafecastData{}
		new.DeviceID = value.DeviceID
		new.DeviceUID = value.DeviceUID
		new.DeviceSN = value.DeviceSN
		new.CapturedAt = ShuffledAt
		new.Opc = value.Opc
		value.OpcHistory[0] = new
	}

	// Shuffle
	if ChangedGeiger {
		for i := len(value.GeigerHistory) - 1; i > 0; i-- {
			value.GeigerHistory[i] = value.GeigerHistory[i-1]
		}
		new := ttdata.SafecastData{}
		new.DeviceID = value.DeviceID
		new.DeviceUID = value.DeviceUID
		new.DeviceSN = value.DeviceSN
		new.CapturedAt = ShuffledAt
		new.Lnd = value.Lnd
		value.GeigerHistory[0] = new
	}

	// If the current transport has an IP address, try to
	// get the IP info

	if value.Service != nil && value.Service.Transport != nil {
		ipInfo := IPInfoData{}
		Str1 := strings.Split(*value.Service.Transport, ":")
		IP := Str1[len(Str1)-1]
		Str2 := strings.Split(IP, ".")
		isValidIP := len(Str1) > 1 && len(Str2) == 4
		if isValidIP {
			response, err := http.Get("http://ip-api.com/json/" + IP)
			if err == nil {
				defer response.Body.Close()
				contents, err := io.ReadAll(response.Body)
				if err == nil {
					var info IPInfoData
					err = json.Unmarshal(contents, &info)
					if err == nil {
						ipInfo = info
					}
				}
			}
		}
		value.IPInfo = ipInfo
	}

	// Write it to the file until it's written correctly, to allow for concurrency
	filename := GetDeviceStatusFilePath(sc.DeviceUID)
	valueJSON, _ := json.MarshalIndent(value, "", "    ")

	for {

		// Write the value
		fd, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
		if err != nil {
			fmt.Printf("*** Unable to write %s: %v\n", filename, err)
			break
		}
		fd.WriteString(string(valueJSON))
		fd.Close()

		// Delay, to increase the chance that we will catch a concurrent update/overwrite
		time.Sleep(time.Duration(Random(1, 6)) * time.Second)

		// Do an integrity check, and re-write the value if necessary
		_, isEmpty, _ := ReadDeviceStatus(sc.DeviceUID)
		if !isEmpty {
			break
		}
	}

}

// GetDeviceStatusSummary gets a summary of a device
func GetDeviceStatusSummary(DeviceUID string) (label string, gps string, Lat float64, Lon float64, s string) {

	// Read the "last known interesting data value" from the status file, which might include GPS
	_, _, value := ReadDeviceStatus(DeviceUID)

	// If this device maps to a safecast device, look up its spreadsheet info
	var info string
	if value.DeviceID != 0 {
		value.DeviceSN, info = sheetDeviceIDToSN(value.DeviceID)
	}
	label = value.DeviceSN
	if info != "" {
		if label != "" {
			label += " "
		}
		label += fmt.Sprintf("(%s)", info)
	}

	// Get location
	if value.Loc != nil && value.Loc.Lat != nil && value.Loc.Lon != nil {
		Lat = *value.Loc.Lat
		Lon = *value.Loc.Lon
		gps = fmt.Sprintf("<http://maps.google.com/maps?z=12&t=m&q=loc:%f+%f|gps>", *value.Loc.Lat, *value.Loc.Lon)
	}

	// Build the summary from the value
	needSpace := false
	if value.Bat != nil && value.Bat.Voltage != nil {
		needSpace = true
		s += fmt.Sprintf("%.1fv", *value.Bat.Voltage)
	}

	if value.Lnd != nil {
		rad := ""
		if value.Lnd.U7318 != nil {
			rad += fmt.Sprintf("%.0f", *value.Lnd.U7318)
		}
		if value.Lnd.C7318 != nil {
			if rad != "" {
				rad += "|"
			}
			rad += fmt.Sprintf("%.0f", *value.Lnd.C7318)
		}
		if value.Lnd.EC7128 != nil {
			if rad != "" {
				rad += "|"
			}
			rad += fmt.Sprintf("%.0f", *value.Lnd.EC7128)
		}
		if value.Lnd.U712 != nil {
			if rad != "" {
				rad += "|"
			}
			rad += fmt.Sprintf("%.0f", *value.Lnd.U712)
		}
		if value.Lnd.W78017 != nil {
			if rad != "" {
				rad += "|"
			}
			rad += fmt.Sprintf("%.0f", *value.Lnd.W78017)
		}
		if rad != "" {
			if needSpace {
				s += " "
				needSpace = false
			}
			s += rad + "cpm"
			needSpace = true
		}
	}
	air := ""
	if value.Opc != nil {
		if value.Opc.Pm01_0 != nil && value.Opc.Pm02_5 != nil && value.Opc.Pm10_0 != nil {
			air += fmt.Sprintf("%.1f|%.1f|%.1fug/m3", *value.Opc.Pm01_0, *value.Opc.Pm02_5, *value.Opc.Pm10_0)
		}
	} else if value.Pms != nil {
		if value.Pms.Pm01_0 != nil && value.Pms.Pm02_5 != nil && value.Pms.Pm10_0 != nil {
			air += fmt.Sprintf("%.1f|%.1f|%.1fug/m3", *value.Pms.Pm01_0, *value.Pms.Pm02_5, *value.Pms.Pm10_0)
		}
	} else if value.Pms2 != nil {
		if value.Pms2.Pm01_0 != nil && value.Pms2.Pm02_5 != nil && value.Pms2.Pm10_0 != nil {
			air += fmt.Sprintf("%.1f|%.1f|%.1fug/m3", *value.Pms2.Pm01_0, *value.Pms2.Pm02_5, *value.Pms2.Pm10_0)
		}
	}
	if air != "" {
		if needSpace {
			s += " "
			needSpace = false
		}
		s += air
		needSpace = true
	}

	// Done
	return

}

// DeviceUIDFilename cleans a device UID for use in a filename
func DeviceUIDFilename(DeviceUID string) (clean string) {
	if DeviceUID == "" {
		DeviceUID = "unknown"
	}
	clean = strings.ReplaceAll(DeviceUID, ":", "-")
	clean = strings.ReplaceAll(clean, ".", "-")
	return
}

// GetDeviceStatusFilePath generates a device status filename in the local file store
func GetDeviceStatusFilePath(DeviceUID string) string {

	return SafecastDirectory() + TTDeviceStatusPath + "/" + DeviceUIDFilename(DeviceUID) + ".json"

}
