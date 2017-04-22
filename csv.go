// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// CSV file handling
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
    fd.WriteString("service_uploaded,when_captured,device,lnd_7318u,lnd_7318c,lnd_7128ec,loc_when_motion_began,loc_olc,loc_lat,loc_lon,loc_alt,bat_voltage,bat_charge,bat_current,env_temp,env_humid,env_press,enc_temp,enc_humid,enc_press,pms_pm01_0,pms_pm02_5,pms_pm10_0,pms_c00_30,pms_c00_50,pms_c01_00,pms_c02_50,pms_c05_00,pms_c10_00,pms_csecs,opc_pm01_0,opc_pm02_5,opc_pm10_0,opc_c00_38,opc_c00_54,opc_c01_00,opc_c02_10,opc_c05_00,opc_c10_00,opc_csecs,service_transport,gateway_lora_snr,service_handler,dev_test,dev_label,dev_uptime,dev_firmware,dev_cfgdev,dev_cfgsvc,dev_cfgttn,dev_cfggps,dev_cfgsen,dev_transmitted_bytes,dev_received_bytes,dev_comms_resets,dev_comms_failures,dev_comms_power_fails,dev_restarts,dev_motion_events,dev_oneshots,dev_oneshot_seconds,dev_iccid,dev_cpsi,dev_dfu,dev_free_memory,dev_ntp_count,dev_last_failure,dev_status,dev_module_lora,dev_module_fona,dev_err_opc,dev_err_pms,dev_err_bme0,dev_err_bme1,dev_err_lora,dev_err_fona,dev_err_geiger,dev_err_max01,dev_err_ugps,dev_err_twi,dev_err_twi_info,dev_err_lis,dev_err_spi,dev_err_con_lora,dev_err_con_fona,dev_err_con_wireless,dev_err_con_data,dev_err_con_service,dev_err_con_gateway,dev_comms_ant_fails,lnd_712u,lnd_78017w,dev_motion\r\n")

    // Done
    return fd, nil

}

// Done
func csvClose(fd *os.File) {
    fd.Close()
}

// Append a measurement to the dataset
func csvAppend(fd *os.File, sd *SafecastData, first bool) {

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
		// Handle Safecast Air's misformatted captured at dates
		if err != nil {
	        t, err = time.Parse("2006-1-2T15:4:5Z", *sd.CapturedAt)
		}
		// If no error, reformat it
        if err == nil {
            s += t.UTC().Format("2006-01-02 15:04:05")
        } else {
            s += *sd.CapturedAt
        }
    }

    s = s + fmt.Sprintf(",%d", *sd.DeviceId)

    if sd.Lnd != nil && sd.Lnd.U7318 != nil {
        s = s + fmt.Sprintf(",%f", *sd.Lnd.U7318)
    } else {
        s += ","
    }
    if sd.Lnd != nil && sd.Lnd.C7318 != nil {
        s = s + fmt.Sprintf(",%f", *sd.Lnd.C7318)
    } else {
        s += ","
    }
    if sd.Lnd != nil && sd.Lnd.EC7128 != nil {
        s = s + fmt.Sprintf(",%f", *sd.Lnd.EC7128)
    } else {
        s += ","
    }

    if sd.Loc != nil && sd.Loc.MotionBegan != nil {
        t, err = time.Parse("2006-01-02T15:04:05Z", *sd.Loc.MotionBegan)
        if err == nil {
            s += t.UTC().Format("2006-01-02 15:04:05")
        } else {
            s += *sd.Loc.MotionBegan
        }
    } else {
        s += ","
    }
    if sd.Loc != nil && sd.Loc.Olc != nil {
        s = s + fmt.Sprintf(",%s", *sd.Loc.Olc)
    } else {
        s += ","
    }
    if sd.Loc != nil && sd.Loc.Lat != nil {
        s = s + fmt.Sprintf(",%f", *sd.Loc.Lat)
    } else {
        s += ","
    }
    if sd.Loc != nil && sd.Loc.Lon != nil {
        s = s + fmt.Sprintf(",%f", *sd.Loc.Lon)
    } else {
        s += ","
    }
    if sd.Loc != nil && sd.Loc.Alt != nil {
        s = s + fmt.Sprintf(",%f", *sd.Loc.Alt)
    } else {
        s += ","
    }

    if sd.Bat != nil && sd.Bat.Voltage != nil {
        s += fmt.Sprintf(",%f", *sd.Bat.Voltage)
    } else {
        s += ","
    }
    if sd.Bat != nil && sd.Bat.Charge != nil {
        s += fmt.Sprintf(",%f", *sd.Bat.Charge)
    } else {
        s += ","
    }
    if sd.Bat != nil && sd.Bat.Current != nil {
        s += fmt.Sprintf(",%f", *sd.Bat.Current)
    } else {
        s += ","
    }

    if sd.Env != nil && sd.Env.Temp != nil {
        s += fmt.Sprintf(",%f", *sd.Env.Temp)
    } else {
        s += ","
    }
    if sd.Env != nil && sd.Env.Humid != nil {
        s = s + fmt.Sprintf(",%f", *sd.Env.Humid)
    } else {
        s += ","
    }
    if sd.Env != nil && sd.Env.Press != nil {
        s = s + fmt.Sprintf(",%f", *sd.Env.Press)
    } else {
        s += ","
    }

    if sd.Dev != nil && sd.Dev.Temp != nil {
        s += fmt.Sprintf(",%f", *sd.Dev.Temp)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.Humid != nil {
        s = s + fmt.Sprintf(",%f", *sd.Dev.Humid)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.Press != nil {
        s = s + fmt.Sprintf(",%f", *sd.Dev.Press)
    } else {
        s += ","
    }

    if sd.Pms != nil && sd.Pms.Pm01_0 != nil {
        s += fmt.Sprintf(",%f", *sd.Pms.Pm01_0)
    } else {
        s += ","
    }
    if sd.Pms != nil && sd.Pms.Pm02_5 != nil {
        s += fmt.Sprintf(",%f", *sd.Pms.Pm02_5)
    } else {
        s += ","
    }
    if sd.Pms != nil && sd.Pms.Pm10_0 != nil {
        s += fmt.Sprintf(",%f", *sd.Pms.Pm10_0)
    } else {
        s += ","
    }
    if sd.Pms != nil && sd.Pms.Count00_30 != nil {
        s += fmt.Sprintf(",%d", *sd.Pms.Count00_30)
    } else {
        s += ","
    }
    if sd.Pms != nil && sd.Pms.Count00_50 != nil {
        s += fmt.Sprintf(",%d", *sd.Pms.Count00_50)
    } else {
        s += ","
    }
    if sd.Pms != nil && sd.Pms.Count01_00 != nil {
        s += fmt.Sprintf(",%d", *sd.Pms.Count01_00)
    } else {
        s += ","
    }
    if sd.Pms != nil && sd.Pms.Count02_50 != nil {
        s += fmt.Sprintf(",%d", *sd.Pms.Count02_50)
    } else {
        s += ","
    }
    if sd.Pms != nil && sd.Pms.Count05_00 != nil {
        s += fmt.Sprintf(",%d", *sd.Pms.Count05_00)
    } else {
        s += ","
    }
    if sd.Pms != nil && sd.Pms.Count10_00 != nil {
        s += fmt.Sprintf(",%d", *sd.Pms.Count10_00)
    } else {
        s += ","
    }
    if sd.Pms != nil && sd.Pms.CountSecs != nil {
        s += fmt.Sprintf(",%d", *sd.Pms.CountSecs)
    } else {
        s += ","
    }

    if sd.Opc != nil && sd.Opc.Pm01_0 != nil {
        s += fmt.Sprintf(",%f", *sd.Opc.Pm01_0)
    } else {
        s += ","
    }
    if sd.Opc != nil && sd.Opc.Pm02_5 != nil {
        s += fmt.Sprintf(",%f", *sd.Opc.Pm02_5)
    } else {
        s += ","
    }
    if sd.Opc != nil && sd.Opc.Pm10_0 != nil {
        s += fmt.Sprintf(",%f", *sd.Opc.Pm10_0)
    } else {
        s += ","
    }
    if sd.Opc != nil && sd.Opc.Count00_38 != nil {
        s += fmt.Sprintf(",%d", *sd.Opc.Count00_38)
    } else {
        s += ","
    }
    if sd.Opc != nil && sd.Opc.Count00_54 != nil {
        s += fmt.Sprintf(",%d", *sd.Opc.Count00_54)
    } else {
        s += ","
    }
    if sd.Opc != nil && sd.Opc.Count01_00 != nil {
        s += fmt.Sprintf(",%d", *sd.Opc.Count01_00)
    } else {
        s += ","
    }
    if sd.Opc != nil && sd.Opc.Count02_10 != nil {
        s += fmt.Sprintf(",%d", *sd.Opc.Count02_10)
    } else {
        s += ","
    }
    if sd.Opc != nil && sd.Opc.Count05_00 != nil {
        s += fmt.Sprintf(",%d", *sd.Opc.Count05_00)
    } else {
        s += ","
    }
    if sd.Opc != nil && sd.Opc.Count10_00 != nil {
        s += fmt.Sprintf(",%d", *sd.Opc.Count10_00)
    } else {
        s += ","
    }
    if sd.Opc != nil && sd.Opc.CountSecs != nil {
        s += fmt.Sprintf(",%d", *sd.Opc.CountSecs)
    } else {
        s += ","
    }

    // Service metadata
    if sd.Service != nil && sd.Service.Transport != nil {
        s = s + fmt.Sprintf(",%s", *sd.Service.Transport)
    } else {
        s += ","
    }
    if sd.Gateway != nil && sd.Gateway.SNR != nil {
        s += fmt.Sprintf(",%f", *sd.Gateway.SNR)
    } else {
        s += ","
    }
    if sd.Service != nil && sd.Service.Handler != nil {
        s = s + fmt.Sprintf(",%s", *sd.Service.Handler)
    } else {
        s += ","
    }

    // Turn stats into a safe string for CSV
    if sd.Dev != nil && sd.Dev.Test != nil {
        s += fmt.Sprintf(",%t", *sd.Dev.Test)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.DeviceLabel != nil {
        s += fmt.Sprintf(",%s", *sd.Dev.DeviceLabel)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.UptimeMinutes != nil {
        s += fmt.Sprintf(",%d", *sd.Dev.UptimeMinutes)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.AppVersion != nil {
        s += fmt.Sprintf(",%s", *sd.Dev.AppVersion)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.DeviceParams != nil {
        s += fmt.Sprintf(",%s", *sd.Dev.DeviceParams)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.ServiceParams != nil {
        s += fmt.Sprintf(",%s", *sd.Dev.ServiceParams)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.TtnParams != nil {
        s += fmt.Sprintf(",%s", *sd.Dev.TtnParams)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.GpsParams != nil {
        s += fmt.Sprintf(",%s", *sd.Dev.GpsParams)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.SensorParams != nil {
        s += fmt.Sprintf(",%s", *sd.Dev.SensorParams)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.TransmittedBytes != nil {
        s += fmt.Sprintf(",%d", *sd.Dev.TransmittedBytes)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.ReceivedBytes != nil {
        s += fmt.Sprintf(",%d", *sd.Dev.ReceivedBytes)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.CommsResets != nil {
        s += fmt.Sprintf(",%d", *sd.Dev.CommsResets)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.CommsFails != nil {
        s += fmt.Sprintf(",%d", *sd.Dev.CommsFails)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.CommsPowerFails != nil {
        s += fmt.Sprintf(",%d", *sd.Dev.CommsPowerFails)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.DeviceRestarts != nil {
        s += fmt.Sprintf(",%d", *sd.Dev.DeviceRestarts)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.MotionEvents != nil {
        s += fmt.Sprintf(",%d", *sd.Dev.MotionEvents)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.Oneshots != nil {
        s += fmt.Sprintf(",%d", *sd.Dev.Oneshots)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.OneshotSeconds != nil {
        s += fmt.Sprintf(",%d", *sd.Dev.OneshotSeconds)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.Iccid != nil {
        s += fmt.Sprintf(",\"SIM %s\"", *sd.Dev.Iccid)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.Cpsi != nil {
        s += fmt.Sprintf(",\"%s\"", *sd.Dev.Cpsi)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.Dfu != nil {
        s += fmt.Sprintf(",%s", *sd.Dev.Dfu)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.FreeMem != nil {
        s += fmt.Sprintf(",%d", *sd.Dev.FreeMem)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.NTPCount != nil {
        s += fmt.Sprintf(",%d", *sd.Dev.NTPCount)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.LastFailure != nil {
        s += fmt.Sprintf(",%s", *sd.Dev.LastFailure)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.Status != nil {
        s += fmt.Sprintf(",%s", *sd.Dev.Status)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.ModuleLora != nil {
        s += fmt.Sprintf(",%s", *sd.Dev.ModuleLora)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.ModuleFona != nil {
        s += fmt.Sprintf(",%s", *sd.Dev.ModuleFona)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.ErrorsOpc != nil {
		s += fmt.Sprintf(",%d", *sd.Dev.ErrorsOpc)
	} else {
		s += ","
	}
    if sd.Dev != nil && sd.Dev.ErrorsPms != nil {
		s += fmt.Sprintf(",%d", *sd.Dev.ErrorsPms)
	} else {
		s += ","
	}
    if sd.Dev != nil && sd.Dev.ErrorsBme0 != nil {
		s += fmt.Sprintf(",%d", *sd.Dev.ErrorsBme0)
	} else {
		s += ","
	}
    if sd.Dev != nil && sd.Dev.ErrorsBme1 != nil {
		s += fmt.Sprintf(",%d", *sd.Dev.ErrorsBme1)
	} else {
		s += ","
	}
    if sd.Dev != nil && sd.Dev.ErrorsLora != nil {
		s += fmt.Sprintf(",%d", *sd.Dev.ErrorsLora)
	} else {
		s += ","
	}
    if sd.Dev != nil && sd.Dev.ErrorsFona != nil {
		s += fmt.Sprintf(",%d", *sd.Dev.ErrorsFona)
	} else {
		s += ","
	}
    if sd.Dev != nil && sd.Dev.ErrorsGeiger != nil {
		s += fmt.Sprintf(",%d", *sd.Dev.ErrorsGeiger)
	} else {
		s += ","
	}
    if sd.Dev != nil && sd.Dev.ErrorsMax01 != nil {
		s += fmt.Sprintf(",%d", *sd.Dev.ErrorsMax01)
	} else {
		s += ","
	}
    if sd.Dev != nil && sd.Dev.ErrorsUgps != nil {
		s += fmt.Sprintf(",%d", *sd.Dev.ErrorsUgps)
	} else {
		s += ","
	}
    if sd.Dev != nil && sd.Dev.ErrorsTwi != nil {
		s += fmt.Sprintf(",%d", *sd.Dev.ErrorsTwi)
	} else {
		s += ","
	}
    if sd.Dev != nil && sd.Dev.ErrorsTwiInfo != nil {
		s += fmt.Sprintf(",%s", *sd.Dev.ErrorsTwiInfo)
	} else {
		s += ","
	}
    if sd.Dev != nil && sd.Dev.ErrorsLis != nil {
		s += fmt.Sprintf(",%d", *sd.Dev.ErrorsLis)
	} else {
		s += ","
	}
    if sd.Dev != nil && sd.Dev.ErrorsSpi != nil {
		s += fmt.Sprintf(",%d", *sd.Dev.ErrorsSpi)
	} else {
		s += ","
	}
    if sd.Dev != nil && sd.Dev.ErrorsConnectLora != nil {
		s += fmt.Sprintf(",%d", *sd.Dev.ErrorsConnectLora)
	} else {
		s += ","
	}
    if sd.Dev != nil && sd.Dev.ErrorsConnectFona != nil {
		s += fmt.Sprintf(",%d", *sd.Dev.ErrorsConnectFona)
	} else {
		s += ","
	}
    if sd.Dev != nil && sd.Dev.ErrorsConnectWireless != nil {
		s += fmt.Sprintf(",%d", *sd.Dev.ErrorsConnectWireless)
	} else {
		s += ","
	}
    if sd.Dev != nil && sd.Dev.ErrorsConnectData != nil {
		s += fmt.Sprintf(",%d", *sd.Dev.ErrorsConnectData)
	} else {
		s += ","
	}
    if sd.Dev != nil && sd.Dev.ErrorsConnectService != nil {
		s += fmt.Sprintf(",%d", *sd.Dev.ErrorsConnectService)
	} else {
		s += ","
	}
    if sd.Dev != nil && sd.Dev.ErrorsConnectGateway != nil {
		s += fmt.Sprintf(",%d", *sd.Dev.ErrorsConnectGateway)
	} else {
		s += ","
	}
    if sd.Dev != nil && sd.Dev.CommsAntFails != nil {
        s += fmt.Sprintf(",%d", *sd.Dev.CommsAntFails)
    } else {
        s += ","
    }

    if sd.Lnd != nil && sd.Lnd.U712 != nil {
        s = s + fmt.Sprintf(",%f", *sd.Lnd.U712)
    } else {
        s += ","
    }
    if sd.Lnd != nil && sd.Lnd.W78017 != nil {
        s = s + fmt.Sprintf(",%f", *sd.Lnd.W78017)
    } else {
        s += ","
    }
    if sd.Dev != nil && sd.Dev.Motion != nil {
        s += fmt.Sprintf(",%t", *sd.Dev.Motion)
    } else {
        s += ","
    }

    s = s + "\r\n"

    fd.WriteString(s)

}
