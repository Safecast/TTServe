// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Log file handling, whether JSON or CSV
package main

import (
    "os"
    "fmt"
    "time"
    "encoding/json"
)

// Construct path of a log file
func SafecastDeviceLogFilename(DeviceId string, Extension string) string {
    directory := SafecastDirectory()
    prefix := time.Now().UTC().Format("2006-01-")
    file := directory + TTDeviceLogPath + "/" + prefix + DeviceId + Extension
    return file
}

// Write to logs.
// Note that we don't do this with a goroutine because the serialization is helpful
// in log-ordering for buffered I/O messages where there are a huge batch of readings
// that are updated in sequence very quickly.
func SafecastWriteToLogs(UploadedAt string, sd SafecastData) {
	go SafecastLogToInflux(sd)
    go SafecastWriteDeviceStatus(UploadedAt, sd)
    go SafecastJSONDeviceLog(UploadedAt, sd)
    go SafecastCSVDeviceLog(UploadedAt, sd)
}

// Write the value to the log
func SafecastJSONDeviceLog(UploadedAt string, sd SafecastData) {

    file := SafecastDeviceLogFilename(fmt.Sprintf("%d", sd.DeviceId), ".json")

    // Open it
    fd, err := os.OpenFile(file, os.O_WRONLY|os.O_APPEND, 0666)
    if err != nil {

		// Don't attempt to create it if it already exists
	    _, err2 := os.Stat(file)
		if err2 == nil {
            fmt.Printf("Logging: Can't log to %s: %s\n", file, err)
			return
	    }
        if err2 == nil {
			if !os.IsNotExist(err2) {
	            fmt.Printf("Logging: Ignoring attempt to create %s: %s\n", file, err2)
				return
			}
	    }

        // Attempt to create the file because it doesn't already exist
        fd, err = os.OpenFile(file, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
        if err != nil {
            fmt.Printf("Logging: error creating %s: %s\n", file, err)
            return
        }

    }

    // Turn stats into a safe string writing
	if sd.Service == nil {
		var svc Service
		sd.Service = &svc
	}
    sd.Service.UploadedAt = &UploadedAt
    scJSON, _ := json.Marshal(sd)
    fd.WriteString(string(scJSON))
    fd.WriteString("\r\n,\r\n")

    // Close and exit
    fd.Close()

}

// Write the value to the log
func SafecastCSVDeviceLog(UploadedAt string, sd SafecastData) {

    // Extract the device number and form a filename
    file := SafecastDeviceLogFilename(fmt.Sprintf("%d", sd.DeviceId), ".csv")

    // Open it
    fd, err := os.OpenFile(file, os.O_WRONLY|os.O_APPEND, 0666)
    if err != nil {

		// Don't attempt to create it if it already exists
	    _, err2 := os.Stat(file)
		if err2 == nil {
            fmt.Printf("Logging: Can't log to %s: %s\n", file, err)
			return
	    }
        if err2 == nil {
			if !os.IsNotExist(err2) {
	            fmt.Printf("Logging: Ignoring attempt to create %s: %s\n", file, err2)
				return
			}
	    }

        // Attempt to create the file because it doesn't already exist
        fd, err = os.OpenFile(file, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
        if err != nil {
            fmt.Printf("Logging: error creating file %s: %s\n", file, err)
            return
        }

        // Write the header
        fd.WriteString("Uploaded,Captured,Device ID,Stats,Uptime,7318U,7318C,7128EC,Latitude,Longitude,Altitude,Bat V,Bat SOC,Bat I,SNR,Temp C,Humid %,Press Pa,PMS PM 1.0,PMS PM 2.5,PMS PM 10.0,PMS # 0.3,PMS # 0.5,PMS # 1.0,PMS # 2.5,PMS # 5.0,PMS # 10.0,PMS # Secs,OPC PM 1.0,OPC PM 2.5,OPC PM 10.0,OPC # 0.38,OPC # 0.54,OPC # 1.0,OPC # 2.1,OPC # 5.0,OPC # 10.0,OPC # Secs\r\n")

    }

    // Turn stats into a safe string for CSV
    stats := "\""
    if sd.Dev != nil {
        if sd.Dev.UptimeMinutes != nil {
            stats += fmt.Sprintf("Uptime:%d ", *sd.Dev.UptimeMinutes)
        }
        if sd.Dev.AppVersion != nil {
            stats += fmt.Sprintf("AppVersion:%s ", *sd.Dev.AppVersion)
        }
        if sd.Dev.DeviceParams != nil {
            stats += fmt.Sprintf("DevParams:%s ", *sd.Dev.DeviceParams)
        }
        if sd.Dev.GpsParams != nil {
            stats += fmt.Sprintf("GpsParams:%s ", *sd.Dev.GpsParams)
        }
        if sd.Dev.ServiceParams != nil {
            stats += fmt.Sprintf("ServiceParams:%s ", *sd.Dev.ServiceParams)
        }
        if sd.Dev.TtnParams != nil {
            stats += fmt.Sprintf("TtnParams:%s ", *sd.Dev.TtnParams)
        }
        if sd.Dev.SensorParams != nil {
            stats += fmt.Sprintf("SensorParams:%s ", *sd.Dev.SensorParams)
        }
        if sd.Dev.TransmittedBytes != nil {
            stats += fmt.Sprintf("Sent:%d ", *sd.Dev.TransmittedBytes)
        }
        if sd.Dev.ReceivedBytes != nil {
            stats += fmt.Sprintf("Rcvd:%d ", *sd.Dev.ReceivedBytes)
        }
        if sd.Dev.CommsResets != nil {
            stats += fmt.Sprintf("CommsResets:%d ", *sd.Dev.CommsResets)
        }
        if sd.Dev.CommsFails != nil {
            stats += fmt.Sprintf("CommsFails:%d ", *sd.Dev.CommsFails)
        }
        if sd.Dev.CommsPowerFails != nil {
            stats += fmt.Sprintf("CommsPowerFails:%d ", *sd.Dev.CommsPowerFails)
        }
        if sd.Dev.DeviceRestarts != nil {
            stats += fmt.Sprintf("Restarts:%d ", *sd.Dev.DeviceRestarts)
        }
        if sd.Dev.Motiondrops != nil {
            stats += fmt.Sprintf("Motiondrops:%d ", *sd.Dev.Motiondrops)
        }
        if sd.Dev.Oneshots != nil {
            stats += fmt.Sprintf("Oneshots:%d ", *sd.Dev.Oneshots)
        }
        if sd.Dev.OneshotSeconds != nil {
            stats += fmt.Sprintf("OneshotSecs:%d ", *sd.Dev.OneshotSeconds)
        }
        if sd.Dev.Iccid != nil {
            stats += fmt.Sprintf("Iccid:%s ", *sd.Dev.Iccid)
        }
        if sd.Dev.Cpsi != nil {
            stats += fmt.Sprintf("Cpsi:%s ", *sd.Dev.Cpsi)
        }
        if sd.Dev.Dfu != nil {
            stats += fmt.Sprintf("DFU:%s ", *sd.Dev.Dfu)
        }
        if sd.Dev.ModuleLora != nil {
            stats += fmt.Sprintf("DFU:%s ", *sd.Dev.ModuleLora)
        }
        if sd.Dev.ModuleFona != nil {
            stats += fmt.Sprintf("DFU:%s ", *sd.Dev.ModuleFona)
        }
        if sd.Dev.DeviceLabel != nil {
            stats += fmt.Sprintf("Label:%s ", *sd.Dev.DeviceLabel)
        }
        if sd.Dev.FreeMem != nil {
            stats += fmt.Sprintf("FreeMem:%d ", *sd.Dev.FreeMem)
        }
        if sd.Dev.NTPCount != nil {
            stats += fmt.Sprintf("NTPCount:%d ", *sd.Dev.NTPCount)
        }
        if sd.Dev.LastFailure != nil {
            stats += fmt.Sprintf("LastFailure:%s ", *sd.Dev.LastFailure)
        }
        if sd.Dev.Status != nil {
            stats += fmt.Sprintf("Status:%s ", *sd.Dev.Status)
        }
    }
    stats = stats + "\""

    // Write the stuff
    s := ""

    // Convert the times to something that can be parsed by Excel
    zTime := ""
    if sd.Service != nil && sd.Service.UploadedAt != nil {
        zTime = fmt.Sprintf("%s", *sd.Service.UploadedAt)
    } else if UploadedAt != "" {
        zTime = UploadedAt
    }
    t, err := time.Parse("2006-01-02T15:04:05Z", zTime)
    if err == nil {
        zTime = t.UTC().Format("2006-01-02 15:04:05")
    }
    s += zTime

    s += ","
    if sd.CapturedAt != nil {
        t, err = time.Parse("2006-01-02T15:04:05Z", *sd.CapturedAt)
        if err == nil {
            s += t.UTC().Format("2006-01-02 15:04:05")
        } else {
            s += *sd.CapturedAt
        }
    }

    s = s + fmt.Sprintf(",%d", sd.DeviceId)
    s = s + fmt.Sprintf(",%s", stats)
    s = s + fmt.Sprintf(",%s", "")          // Value
    if sd.Lnd == nil {
        s += ",,,"
    } else {
        if sd.U7318 != nil {
            s = s + fmt.Sprintf(",%f", *sd.U7318)
        } else {
            s += ","
        }
        if sd.C7318 != nil {
            s = s + fmt.Sprintf(",%f", *sd.C7318)
        } else {
            s += ","
        }
        if sd.EC7128 != nil {
            s = s + fmt.Sprintf(",%f", *sd.EC7128)
        } else {
            s += ","
        }
    }
    if sd.Loc == nil {
        s += ",,,"
    } else {
        s = s + fmt.Sprintf(",%f", sd.Loc.Lat)
        s = s + fmt.Sprintf(",%f", sd.Loc.Lon)
        if sd.Loc.Alt != nil {
            s = s + fmt.Sprintf(",%f", *sd.Loc.Alt)
        } else {
            s += ","
        }
    }
    if sd.Bat == nil {
        s += ",,,"
    } else {
        if sd.Bat.Voltage == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Bat.Voltage)
        }
        if sd.Bat.Charge == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Bat.Charge)
        }
        if sd.Bat.Current == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Bat.Current)
        }
    }
    if sd.Gateway == nil {
        s += ","
    } else {
        if sd.Gateway.SNR == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Gateway.SNR)
        }
    }
    if sd.Env == nil {
        s += ",,,"
    } else {
        if sd.Env.Temp == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Env.Temp)
        }
        if sd.Env.Humid == nil {
            s += ","
        } else {
            s = s + fmt.Sprintf(",%f", *sd.Env.Humid)
        }
        if sd.Env.Press == nil {
            s += ","
        } else {
            s = s + fmt.Sprintf(",%f", *sd.Env.Press)
        }
    }
    if sd.Pms == nil {
        s += ",,,,,,,,,,"
    } else {
        if sd.Pms.Pm01_0 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Pms.Pm01_0)
        }
        if sd.Pms.Pm02_5 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Pms.Pm02_5)
        }
        if sd.Pms.Pm10_0 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Pms.Pm10_0)
        }
        if sd.Pms.Count00_30 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Pms.Count00_30)
        }
        if sd.Pms.Count00_50 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Pms.Count00_50)
        }
        if sd.Pms.Count01_00 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Pms.Count01_00)
        }
        if sd.Pms.Count02_50 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Pms.Count02_50)
        }
        if sd.Pms.Count05_00 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Pms.Count05_00)
        }
        if sd.Pms.Count10_00 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Pms.Count10_00)
        }
        if sd.Pms.CountSecs == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Pms.CountSecs)
        }
    }
    if sd.Opc == nil {
        s += ",,,,,,,,,,"
    } else {
        if sd.Opc.Pm01_0 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Opc.Pm01_0)
        }
        if sd.Opc.Pm02_5 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Opc.Pm02_5)
        }
        if sd.Opc.Pm10_0 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Opc.Pm10_0)
        }
        if sd.Opc.Count00_38 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Opc.Count00_38)
        }
        if sd.Opc.Count00_54 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Opc.Count00_54)
        }
        if sd.Opc.Count01_00 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Opc.Count01_00)
        }
        if sd.Opc.Count02_10 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Opc.Count02_10)
        }
        if sd.Opc.Count05_00 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Opc.Count05_00)
        }
        if sd.Opc.Count10_00 == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Opc.Count10_00)
        }
        if sd.Opc.CountSecs == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%d", *sd.Opc.CountSecs)
        }
    }
    s = s + "\r\n"

    fd.WriteString(s)

    // Close and exit
    fd.Close()

}
