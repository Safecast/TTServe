// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Log file handling, whether JSON or CSV
package main

import (
    "os"
	"time"
    "fmt"
)

func csvOpen(filename string) (*os.File, error) {

    // Open it
    fd, err := os.OpenFile(filename, os.O_WRONLY|os.O_APPEND, 0666)

	// Exit if no error
	if err == nil {
		return fd, nil
	}

    // Don't attempt to create it if it already exists
    _, err2 := os.Stat(filename)
    if err2 == nil {
        return nil, err
    }
    if !os.IsNotExist(err2) {
        return nil, err2
    }

	// Create the new dataset
	return csvNew(filename)

}

// Create a new dataset
func csvNew(filename string) (*os.File, error) {

    fd, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
    if err != nil {
        return nil, err
    }

    // Write the header
    fd.WriteString("service_uploaded,when_captured,device,dev_uptime,lnd_7318u,lnd_7318c,lnd_7128ec,loc_lat,loc_lon,loc_alt,bat_voltage,bat_charge,bat_current,gateway_lora_snr,env_temp,env_humid,env_press,enc_temp,enc_humid,enc_press,pms_pm01_0,pms_pm02_5,pms_pm10_0,pms_c00_30,pms_c00_50,pms_c01_00,pms_c02_50,pms_c05_00,pms_c10_00,pms_csecs,opc_pm01_0,opc_pm02_5,opc_pm10_0,opc_c00_38,opc_c00_54,opc_c01_00,opc_c02_10,opc_c05_00,opc_c10_00,opc_csecs,STATS\r\n")

	// Done
	return fd, nil

}

// Done
func csvClose(fd *os.File) {
	fd.Close()
}

// Append a measurement to the dataset
func csvAppend(fd *os.File, sd *SafecastData) {

    // Write the stuff
    s := ""

    // Convert the times to something that can be parsed by Excel
    zTime := ""
    if sd.Service != nil && sd.Service.UploadedAt != nil {
        zTime = fmt.Sprintf("%s", *sd.Service.UploadedAt)
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

    s = s + fmt.Sprintf(",%d", *sd.DeviceId)
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
        if sd.Loc.Lat != nil {
            s = s + fmt.Sprintf(",%f", *sd.Loc.Lat)
        } else {
            s += ","
        }
        if sd.Loc.Lon != nil {
            s = s + fmt.Sprintf(",%f", *sd.Loc.Lon)
        } else {
            s += ","
        }
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
    if sd.Dev == nil {
        s += ",,,"
    } else {
        if sd.Dev.Temp == nil {
            s += ","
        } else {
            s += fmt.Sprintf(",%f", *sd.Dev.Temp)
        }
        if sd.Dev.Humid == nil {
            s += ","
        } else {
            s = s + fmt.Sprintf(",%f", *sd.Dev.Humid)
        }
        if sd.Dev.Press == nil {
            s += ","
        } else {
            s = s + fmt.Sprintf(",%f", *sd.Dev.Press)
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
    s = s + fmt.Sprintf(",%s", stats)
    s = s + "\r\n"

    fd.WriteString(s)

}
