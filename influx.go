// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Influx-related
package main

import (
    "fmt"
    "time"
    influx "github.com/influxdata/influxdb/client/v2"
)

const (
    SafecastDb = "safecast"
    SafecastDataPoint = "data"
)

// Get client config parameters
func InfluxConfig() influx.HTTPConfig {
    var clcfg influx.HTTPConfig
    clcfg.Addr = fmt.Sprintf("https://%s:8086", ServiceConfig.InfluxHost)
    clcfg.Username = ServiceConfig.InfluxUsername
    clcfg.Password = ServiceConfig.InfluxPassword
    return clcfg
}

// Log the specific safecast data point to influx
func SafecastLogToInflux(sd SafecastData) bool {

    // Open the client
    cl, clerr := influx.NewHTTPClient(InfluxConfig())
    if clerr == nil {
        defer cl.Close()
    } else {
        fmt.Printf("Influx connect error: %v\n", clerr)
        return false
    }

    // Create a new batch
    bpcfg := influx.BatchPointsConfig{}
    bpcfg.Database = SafecastDb
    bp, bperr := influx.NewBatchPoints(bpcfg)
    if bperr != nil {
        fmt.Printf("Influx batch points creation error: %v\n", bperr)
        return false
    }

    // Create the tags and fields structures from which a point will be made
    var setMeasurementTime bool = false
    var measurementTime time.Time
    tags := map[string]string{}
    fields := map[string]interface{}{}

    // Extract each safecast field into its influx equivalent

    if sd.DeviceId != nil {
        fields["device"] = *sd.DeviceId
        tags["device_str"] = fmt.Sprintf("%d", *sd.DeviceId)
    }

    if sd.CapturedAt != nil {
        fields["when_captured"] = *sd.CapturedAt
        t, e := time.Parse("2006-01-02T15:04:05Z", *sd.CapturedAt)
        if e == nil {
            fields["when_captured_num"] = t.UnixNano()
            setMeasurementTime = true
            measurementTime = t
        }
    }

    if sd.Loc != nil {
        if sd.Loc.Lat != nil {
            fields["loc_lat"] = *sd.Loc.Lat
        }
        if sd.Loc.Lon != nil {
            fields["loc_lon"] = *sd.Loc.Lon
        }
        if sd.Loc.Alt != nil {
            fields["loc_alt"] = *sd.Loc.Alt
        }
        if sd.Loc.MotionBegan != nil {
            fields["loc_when_motion_began"] = *sd.Loc.MotionBegan
            t, e := time.Parse("2006-01-02T15:04:05Z", *sd.Loc.MotionBegan)
            if e == nil {
                fields["loc_when_motion_began_num"] = t.UnixNano()
            }
        }
        if sd.Loc.Olc != nil {
            tags["loc_olc"] = *sd.Loc.Olc
        }
    }

    if sd.Env != nil {
        if sd.Env.Temp != nil {
            fields["env_temp"] = *sd.Env.Temp
        }
        if sd.Env.Humid != nil {
            fields["env_humid"] = *sd.Env.Humid
        }
        if sd.Env.Press != nil {
            fields["env_press"] = *sd.Env.Press
        }
    }

    if sd.Bat != nil {
        if sd.Bat.Voltage != nil {
            fields["bat_voltage"] = *sd.Bat.Voltage
        }
        if sd.Bat.Current != nil {
            fields["bat_current"] = *sd.Bat.Current
        }
        if sd.Bat.Charge != nil {
            fields["bat_charge"] = *sd.Bat.Charge
        }
    }

    if sd.Lnd != nil {
        if sd.Lnd.U7318 != nil {
            fields["lnd_7318u"] = *sd.Lnd.U7318
        }
        if sd.Lnd.C7318 != nil {
            fields["lnd_7318c"] = *sd.Lnd.C7318
        }
        if sd.Lnd.EC7128 != nil {
            fields["lnd_7128ec"] = *sd.Lnd.EC7128
        }
    }

    if sd.Pms != nil {
        if sd.Pms.Pm01_0 != nil {
            fields["pms_pm01_0"] = *sd.Pms.Pm01_0
        }
        if sd.Pms.Pm02_5 != nil {
            fields["pms_pm02_5"] = *sd.Pms.Pm02_5
        }
        if sd.Pms.Pm10_0 != nil {
            fields["pms_pm10_0"] = *sd.Pms.Pm10_0
        }
        if sd.Pms.Count00_30 != nil {
            fields["pms_c00_30"] = *sd.Pms.Count00_30
        }
        if sd.Pms.Count00_50 != nil {
            fields["pms_c00_50"] = *sd.Pms.Count00_50
        }
        if sd.Pms.Count01_00 != nil {
            fields["pms_c01_00"] = *sd.Pms.Count01_00
        }
        if sd.Pms.Count02_50 != nil {
            fields["pms_c02_50"] = *sd.Pms.Count02_50
        }
        if sd.Pms.Count05_00 != nil {
            fields["pms_c05_00"] = *sd.Pms.Count05_00
        }
        if sd.Pms.Count10_00 != nil {
            fields["pms_c10_00"] = *sd.Pms.Count10_00
        }
        if sd.Pms.CountSecs != nil {
            fields["pms_csecs"] = *sd.Pms.CountSecs
        }
    }

    if sd.Opc != nil {
        if sd.Opc.Pm01_0 != nil {
            fields["opc_pm01_0"] = *sd.Opc.Pm01_0
        }
        if sd.Opc.Pm02_5 != nil {
            fields["opc_pm02_5"] = *sd.Opc.Pm02_5
        }
        if sd.Opc.Pm10_0 != nil {
            fields["opc_pm10_0"] = *sd.Opc.Pm10_0
        }
        if sd.Opc.Count00_38 != nil {
            fields["opc_c00_38"] = *sd.Opc.Count00_38
        }
        if sd.Opc.Count00_54 != nil {
            fields["opc_c00_54"] = *sd.Opc.Count00_54
        }
        if sd.Opc.Count01_00 != nil {
            fields["opc_c01_00"] = *sd.Opc.Count01_00
        }
        if sd.Opc.Count02_10 != nil {
            fields["opc_c02_10"] = *sd.Opc.Count02_10
        }
        if sd.Opc.Count05_00 != nil {
            fields["opc_c05_00"] = *sd.Opc.Count05_00
        }
        if sd.Opc.Count10_00 != nil {
            fields["opc_c10_00"] = *sd.Opc.Count10_00
        }
        if sd.Opc.CountSecs != nil {
            fields["opc_csecs"] = *sd.Opc.CountSecs
        }
    }

    if sd.Dev != nil {
        if sd.Dev.Test != nil {
            fields["dev_test"] = *sd.Dev.Test
        }
        if sd.Dev.DeviceLabel != nil {
            tags["dev_label"] = *sd.Dev.DeviceLabel
        }
        if sd.Dev.UptimeMinutes != nil {
            fields["dev_uptime"] = *sd.Dev.UptimeMinutes
        }
        if sd.Dev.AppVersion != nil {
            tags["dev_firmware"] = *sd.Dev.AppVersion
        }
        if sd.Dev.DeviceParams != nil {
            fields["dev_cfgdev"] = *sd.Dev.DeviceParams
        }
        if sd.Dev.ServiceParams != nil {
            fields["dev_cfgsvc"] = *sd.Dev.ServiceParams
        }
        if sd.Dev.TtnParams != nil {
            fields["dev_cfgttn"] = *sd.Dev.TtnParams
        }
        if sd.Dev.GpsParams != nil {
            fields["dev_cfggps"] = *sd.Dev.GpsParams
        }
        if sd.Dev.SensorParams != nil {
            fields["dev_cfgsen"] = *sd.Dev.SensorParams
        }
        if sd.Dev.TransmittedBytes != nil {
            fields["dev_transmitted_bytes"] = *sd.Dev.TransmittedBytes
        }
        if sd.Dev.ReceivedBytes != nil {
            fields["dev_received_bytes"] = *sd.Dev.ReceivedBytes
        }
        if sd.Dev.CommsResets != nil {
            fields["dev_comms_resets"] = *sd.Dev.CommsResets
        }
        if sd.Dev.CommsFails != nil {
            fields["dev_comms_failures"] = *sd.Dev.CommsFails
        }
        if sd.Dev.CommsPowerFails != nil {
            fields["dev_comms_power_fails"] = *sd.Dev.CommsPowerFails
        }
        if sd.Dev.DeviceRestarts != nil {
            fields["dev_restarts"] = *sd.Dev.DeviceRestarts
        }
        if sd.Dev.Motiondrops != nil {
            fields["dev_motiondrops"] = *sd.Dev.Motiondrops
        }
        if sd.Dev.Oneshots != nil {
            fields["dev_oneshots"] = *sd.Dev.Oneshots
        }
        if sd.Dev.OneshotSeconds != nil {
            fields["dev_oneshot_seconds"] = *sd.Dev.OneshotSeconds
        }
        if sd.Dev.Iccid != nil {
            fields["dev_iccid"] = *sd.Dev.Iccid
        }
        if sd.Dev.Cpsi != nil {
            fields["dev_cpsi"] = *sd.Dev.Cpsi
        }
        if sd.Dev.Dfu != nil {
            fields["dev_dfu"] = *sd.Dev.Dfu
        }
        if sd.Dev.FreeMem != nil {
            fields["dev_free_memory"] = *sd.Dev.FreeMem
        }
        if sd.Dev.NTPCount != nil {
            fields["dev_ntp_count"] = *sd.Dev.NTPCount
        }
        if sd.Dev.LastFailure != nil {
            fields["dev_last_failure"] = *sd.Dev.LastFailure
        }
        if sd.Dev.Status != nil {
            fields["dev_status"] = *sd.Dev.Status
        }
        if sd.Dev.ModuleLora != nil {
            tags["dev_module_lora"] = *sd.Dev.ModuleLora
        }
        if sd.Dev.ModuleFona != nil {
            tags["dev_module_fona"] = *sd.Dev.ModuleFona
        }
        if sd.Dev.Temp != nil {
            fields["dev_temp"] = *sd.Dev.Temp
        }
        if sd.Dev.Humid != nil {
            fields["dev_humid"] = *sd.Dev.Humid
        }
        if sd.Dev.Press != nil {
            fields["dev_press"] = *sd.Dev.Press
        }
        if sd.Dev.ErrorsOpc != nil {
            fields["dev_err_opc"] = *sd.Dev.ErrorsOpc
        }
        if sd.Dev.ErrorsPms != nil {
            fields["dev_err_pms"] = *sd.Dev.ErrorsPms
        }
        if sd.Dev.ErrorsBme0 != nil {
            fields["dev_err_bme0"] = *sd.Dev.ErrorsBme0
        }
        if sd.Dev.ErrorsBme1 != nil {
            fields["dev_err_bme1"] = *sd.Dev.ErrorsBme1
        }
        if sd.Dev.ErrorsLora != nil {
            fields["dev_err_lora"] = *sd.Dev.ErrorsLora
        }
        if sd.Dev.ErrorsFona != nil {
            fields["dev_err_fona"] = *sd.Dev.ErrorsFona
        }
        if sd.Dev.ErrorsGeiger != nil {
            fields["dev_err_geiger"] = *sd.Dev.ErrorsGeiger
        }
        if sd.Dev.ErrorsMax01 != nil {
            fields["dev_err_max01"] = *sd.Dev.ErrorsMax01
        }
        if sd.Dev.ErrorsUgps != nil {
            fields["dev_err_ugps"] = *sd.Dev.ErrorsUgps
        }
        if sd.Dev.ErrorsTwi != nil {
            fields["dev_err_twi"] = *sd.Dev.ErrorsTwi
        }
        if sd.Dev.ErrorsTwiInfo != nil {
            fields["dev_err_twi_info"] = *sd.Dev.ErrorsTwiInfo
        }
        if sd.Dev.ErrorsLis != nil {
            fields["dev_err_lis"] = *sd.Dev.ErrorsLis
        }
        if sd.Dev.ErrorsSpi != nil {
            fields["dev_err_spi"] = *sd.Dev.ErrorsSpi
        }
        if sd.Dev.ErrorsConnectLora != nil {
            fields["dev_err_con_lora"] = *sd.Dev.ErrorsConnectLora
        }
        if sd.Dev.ErrorsConnectFona != nil {
            fields["dev_err_con_fona"] = *sd.Dev.ErrorsConnectFona
        }
        if sd.Dev.ErrorsConnectWireless != nil {
            fields["dev_err_con_wireless"] = *sd.Dev.ErrorsConnectWireless
        }
        if sd.Dev.ErrorsConnectData != nil {
            fields["dev_err_con_data"] = *sd.Dev.ErrorsConnectData
        }
        if sd.Dev.ErrorsConnectService != nil {
            fields["dev_err_con_service"] = *sd.Dev.ErrorsConnectService
        }
        if sd.Dev.ErrorsConnectGateway != nil {
            fields["dev_err_con_gateway"] = *sd.Dev.ErrorsConnectGateway
        }

    }

    if sd.Gateway != nil {
        if sd.Gateway.ReceivedAt != nil {
            fields["gateway_received"] = *sd.Gateway.ReceivedAt
            t, e := time.Parse("2006-01-02T15:04:05Z", *sd.Gateway.ReceivedAt)
            if e == nil {
                fields["gateway_received_num"] = t.UnixNano()
            }
        }
        if sd.Gateway.SNR != nil {
            fields["gateway_lora_snr"] = *sd.Gateway.SNR
        }
        if sd.Gateway.Lat != nil {
            fields["gateway_loc_lat"] = *sd.Gateway.Lat
        }
        if sd.Gateway.Lon != nil {
            fields["gateway_loc_lon"] = *sd.Gateway.Lon
        }
        if sd.Gateway.Alt != nil {
            fields["gateway_loc_alt"] = *sd.Gateway.Alt
        }
    }

    if sd.Service != nil {
        if sd.Service.UploadedAt != nil {
            fields["service_uploaded"] = *sd.Service.UploadedAt
            t, e := time.Parse("2006-01-02T15:04:05Z", *sd.Service.UploadedAt)
            if e == nil {
                fields["service_uploaded_num"] = t.UnixNano()
                if !setMeasurementTime {
                    setMeasurementTime = true
                    measurementTime = t
                }
            }
        }
        if sd.Service.Transport != nil {
            tags["service_transport"] = *sd.Service.Transport
        }
        if sd.Service.HashMd5 != nil {
            fields["service_md5"] = *sd.Service.HashMd5
        }
        if sd.Service.Handler != nil {
            tags["service_handler"] = *sd.Service.Handler
        }
    }

    // Make a new point
    var mperr error
    var pt *influx.Point
    if setMeasurementTime {
        pt, mperr = influx.NewPoint(SafecastDataPoint, tags, fields, measurementTime)
    } else {
        pt, mperr = influx.NewPoint(SafecastDataPoint, tags, fields)
    }
    if mperr != nil {
        fmt.Printf("Influx point creation error: %v\n", mperr)
        return false
    }

    // Debug
    if (false) {
        fmt.Printf("*** Influx:\n%v\n", pt)
    }

    // Add the point to the batch
    bp.AddPoint(pt)

    // Write the batch
    wrerr := cl.Write(bp)
    if wrerr != nil {
        fmt.Printf("Influx write error: %v\n", wrerr)
        return false
    }

    // Done
    return true

}

// Perform a query, returning either an URL to results or an error message
func InfluxQuery(the_user string, the_query string) (success bool, result string) {

    // Open the client
    cl, clerr := influx.NewHTTPClient(InfluxConfig())
    if clerr == nil {
        defer cl.Close()
    } else {
        return false, fmt.Sprintf("Influx connect error: %v", clerr)
    }

    // Perform the query
    response, qerr := cl.Query(influx.NewQuery("SELECT "+the_query, SafecastDb, "ns"))
    if qerr != nil {
        return false, fmt.Sprintf("Influx query error: %v", qerr)
    }

    // Exit if an err
    if response.Error() != nil {
        return false, fmt.Sprintf("Influx query response error: %v", response.Error())
    }

    // Iterate over all results
    for _, result := range response.Results {
		// Ignore this
        fmt.Printf("%d Messages:\n", len(result.Messages))
        for i, m := range result.Messages {
            fmt.Printf("%d: Level:'%s' Text:'%s'\n", i, m.Level, m.Text)
        }
		// These are sets of results with a name
        fmt.Printf("%d Sets:\n", len(result.Series))
        for i, r := range result.Series {
			// Set name is 'data', put this in column 0
            fmt.Printf("%d: Name:'%s' Tags:'%d' Cols:'%d' Rows:'%d'\n", i, r.Name, len(r.Tags), len(r.Columns), len(r.Values))
			// No tags - don't even know what to do with
            fmt.Printf("%d Tags:\n", len(r.Tags))
            for k, v := range r.Tags {
                fmt.Printf("'%s':'%s'\n", k, v)
            }
			// 86 columns, and each v is the column name
            fmt.Printf("%d Columns:\n", len(r.Columns))
            for i, v := range r.Columns {
                fmt.Printf("%d: '%s'\n", i, v)
            }
			// Rows of results
			fmt.Printf("%d Rows:\n", len(r.Values))
			for i, v := range r.Values {
				fmt.Printf("%d: %d cols'\n", i, len(v))
				for k, cell := range v {
					fmt.Printf("%d: %v'\n", k, cell)
				}
			}
        }

    }

    // const TTInfluxQueryPath = "/influx-query"
    return true, the_query

}
