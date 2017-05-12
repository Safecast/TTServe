// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Influx-related
package main

import (
    "os"
    "fmt"
    "time"
    "encoding/json"
    influx "github.com/influxdata/influxdb/client/v2"
)

const quoteTextInCSV bool = false

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

    if sd.CapturedAt != nil {
        fields["when_captured"] = *sd.CapturedAt
        t, e := time.Parse("2006-01-02T15:04:05Z", *sd.CapturedAt)
        if e == nil {
            fields["when_captured_num"] = t.UnixNano()
            setMeasurementTime = true
            measurementTime = t
        }
    }

    if sd.DeviceId != nil {
        tags["device_str"] = fmt.Sprintf("%d", *sd.DeviceId)
        fields["device"] = *sd.DeviceId
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
            fields["loc_olc"] = *sd.Loc.Olc
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
        if sd.Lnd.U712 != nil {
            fields["lnd_712u"] = *sd.Lnd.U712
        }
        if sd.Lnd.W78017 != nil {
            fields["lnd_78017w"] = *sd.Lnd.W78017
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
        if sd.Pms.Std01_0 != nil {
            fields["pms_std01_0"] = *sd.Pms.Std01_0
        }
        if sd.Pms.Std02_5 != nil {
            fields["pms_std02_5"] = *sd.Pms.Std02_5
        }
        if sd.Pms.Std10_0 != nil {
            fields["pms_std10_0"] = *sd.Pms.Std10_0
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
        if sd.Opc.Std01_0 != nil {
            fields["opc_std01_0"] = *sd.Opc.Std01_0
        }
        if sd.Opc.Std02_5 != nil {
            fields["opc_std02_5"] = *sd.Opc.Std02_5
        }
        if sd.Opc.Std10_0 != nil {
            fields["opc_std10_0"] = *sd.Opc.Std10_0
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
        if sd.Dev.Motion != nil {
            fields["dev_motion"] = *sd.Dev.Motion
        }
        if sd.Dev.DeviceLabel != nil {
            fields["dev_label"] = *sd.Dev.DeviceLabel
        }
        if sd.Dev.UptimeMinutes != nil {
            fields["dev_uptime"] = *sd.Dev.UptimeMinutes
        }
        if sd.Dev.AppVersion != nil {
            fields["dev_firmware"] = *sd.Dev.AppVersion
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
        if sd.Dev.OvercurrentEvents != nil {
            fields["dev_overcurrent_events"] = *sd.Dev.OvercurrentEvents
        }
        if sd.Dev.CommsAntFails != nil {
            fields["dev_comms_ant_fails"] = *sd.Dev.CommsAntFails
        }
        if sd.Dev.DeviceRestarts != nil {
            fields["dev_restarts"] = *sd.Dev.DeviceRestarts
        }
        if sd.Dev.MotionEvents != nil {
            fields["dev_motion_events"] = *sd.Dev.MotionEvents
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
            fields["dev_module_lora"] = *sd.Dev.ModuleLora
        }
        if sd.Dev.ModuleFona != nil {
            fields["dev_module_fona"] = *sd.Dev.ModuleFona
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
        if sd.Dev.ErrorsMtu != nil {
            fields["dev_err_mtu"] = *sd.Dev.ErrorsMtu
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
            fields["service_transport"] = *sd.Service.Transport
        }
        if sd.Service.HashMd5 != nil {
            fields["service_md5"] = *sd.Service.HashMd5
        }
        if sd.Service.Handler != nil {
            fields["service_handler"] = *sd.Service.Handler
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

// Just a debug function that traverses a Response, which took me forever to figure out
func InfluxResultsToFile(response *influx.Response, fCSV bool, fd *os.File) int {
	buf := make([]byte, 8192)
    fDebug := false
    fDebugMax := false
	results := 0
	firstrow := true
	
    for _, result := range response.Results {
        // Ignore this
        if fDebug {
            fmt.Printf("%d Messages:\n", len(result.Messages))
            for i, m := range result.Messages {
                fmt.Printf("%d: Level:'%s' Text:'%s'\n", i, m.Level, m.Text)
            }
        }
        // These are sets of results with a name
        if fDebug {
            fmt.Printf("%d Sets:\n", len(result.Series))
        }
        for i, r := range result.Series {
            if fDebugMax {
                // Set name is 'data', put this in column 0
                fmt.Printf("%d: Name:'%s' Tags:'%d' Cols:'%d' Rows:'%d'\n", i, r.Name, len(r.Tags), len(r.Columns), len(r.Values))
            }
            // Partial, or not
            if fDebugMax {
                fmt.Printf("%d: PARTIAL = %t\n", i, r.Partial)
            }
            // No tags - don't even know what to do with
            if fDebugMax {
                fmt.Printf("%d Tags:\n", len(r.Tags))
                for k, v := range r.Tags {
                    fmt.Printf("'%s':'%s'\n", k, v)
                }
            }
            // 86 columns, and each v is the column name
            if fDebugMax {
                fmt.Printf("%d Columns:\n", len(r.Columns))
                for i, v := range r.Columns {
                    fmt.Printf("%d: '%s'\n", i, v)
                }
            }
            // Rows of results
            if fDebugMax {
                fmt.Printf("%d Rows:\n", len(r.Values))
            }
            for i, v := range r.Values {
                if fDebugMax {
                    fmt.Printf("%d: %d cols\n", i, len(v))
                }
                // Initialize JSON data structure
				buflen := 0
				buflen += copy(buf[buflen:], "{")
                first := true
                // Iterate over cells in the row
                for k, cell := range v {
                    if cell == nil {
                        if fDebugMax {
                            fmt.Printf("%d: NIL\n", k)
                        }
                    } else {
                        colname := ""
                        rowval := ""
                        dbgval := ""
                        if k < len(r.Columns) {
                            colname = r.Columns[k]
                        }
                        switch cell := cell.(type) {
                        default:
                            rowval = fmt.Sprintf("\"%s\":\"%v\"", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: unknown type %T", k, cell)
                            }
                        case json.Number:
                            rowval = fmt.Sprintf("\"%s\":%v", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: json.Number", k)
                            }
                        case string:
                            rowval = fmt.Sprintf("\"%s\":\"%s\"", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: string", k)
                            }
                        case bool:
                            rowval = fmt.Sprintf("\"%s\":%t", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: bool", k)
                            }
                        case *bool:
                            rowval = fmt.Sprintf("\"%s\":%t", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: *bool", k)
                            }
                        case int:
                            rowval = fmt.Sprintf("\"%s\":%d", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: int", k)
                            }
                        case int8:
                            rowval = fmt.Sprintf("\"%s\":%d", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: int8", k)
                            }
                        case int16:
                            rowval = fmt.Sprintf("\"%s\":%d", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: int16", k)
                            }
                        case int32:
                            rowval = fmt.Sprintf("\"%s\":%d", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: int32", k)
                            }
                        case int64:
                            rowval = fmt.Sprintf("\"%s\":%d", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: int64", k)
                            }
                        case *int:
                            rowval = fmt.Sprintf("\"%s\":%d", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: *int", k)
                            }
                        case *int8:
                            rowval = fmt.Sprintf("\"%s\":%d", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: *int8", k)
                            }
                        case *int16:
                            rowval = fmt.Sprintf("\"%s\":%d", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: *int16", k)
                            }
                        case *int32:
                            rowval = fmt.Sprintf("\"%s\":%d", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: *int32", k)
                            }
                        case *int64:
                            rowval = fmt.Sprintf("\"%s\":%d", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: *int64", k)
                            }
                        case uint:
                            rowval = fmt.Sprintf("\"%s\":%u", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: uint", k)
                            }
                        case uint8:
                            rowval = fmt.Sprintf("\"%s\":%u", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: uint8", k)
                            }
                        case uint16:
                            rowval = fmt.Sprintf("\"%s\":%u", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: uint16", k)
                            }
                        case uint32:
                            rowval = fmt.Sprintf("\"%s\":%u", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: uint32", k)
                            }
                        case uint64:
                            rowval = fmt.Sprintf("\"%s\":%u", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: uint64", k)
                            }
                        case *uint:
                            rowval = fmt.Sprintf("\"%s\":%u", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: *uint", k)
                            }
                        case *uint8:
                            rowval = fmt.Sprintf("\"%s\":%u", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: *uint8", k)
                            }
                        case *uint16:
                            rowval = fmt.Sprintf("\"%s\":%u", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: *uint16", k)
                            }
                        case *uint32:
                            rowval = fmt.Sprintf("\"%s\":%u", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: *uint32", k)
                            }
                        case *uint64:
                            rowval = fmt.Sprintf("\"%s\":%u", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: *uint64", k)
                            }
                        case float32:
                            rowval = fmt.Sprintf("\"%s\":%f", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: float32", k)
                            }
                        case float64:
                            rowval = fmt.Sprintf("\"%s\":%f", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: float64", k)
                            }
                        case *float32:
                            rowval = fmt.Sprintf("\"%s\":%f", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: *float32", k)
                            }
                        case *float64:
                            rowval = fmt.Sprintf("\"%s\":%f", colname, cell)
                            if fDebugMax {
                                dbgval = fmt.Sprintf("%d: *float64", k)
                            }
                        }
                        if fDebugMax {
                            fmt.Printf("%s %s\n", dbgval, rowval)
                        }
                        if colname != "" {
                            if first {
                                first = false
                            } else {
								buflen += copy(buf[buflen:], ",")
							}
							buflen += copy(buf[buflen:], rowval)
                        }
                    }

                }

                // End the JSON structure
				buflen += copy(buf[buflen:], "}")

                // Unmarshal it to Safecast data
                sd := SafecastData{}
                err := json.Unmarshal(buf[0:buflen], &sd)
                if err != nil {
                    fmt.Printf("\nError unmarshaling %s:\n%s\n", err, string(buf[0:buflen]))
                } else {

					// Append a row to the file
					if fCSV {
						csvAppend(fd, &sd, firstrow)
					} else {
						jsonAppend(fd, &sd, firstrow)
					}
					firstrow = false
					
					// Bump the number of successful results
					results++

                }

            }
        }
        if fDebug {
			fmt.Printf("** %d Results so far **\n", results)
		}
    }

	return results
	
}

// Perform a query, returning either an URL to results or an error message
func InfluxQuery(the_user string, the_device string, the_query string, is_csv bool) (success bool, numresults int, result string, resultfilename string) {

	// Request for influx query
	fmt.Printf("\n*** %s requested query '%s'\n", the_user, the_query)
	
    // Remap unicode characters (such as single quotes) to ASCII equivalents
    the_query = RemapCommonUnicodeToASCII(the_query)

    // Open the client
    cl, clerr := influx.NewHTTPClient(InfluxConfig())
    if clerr == nil {
        defer cl.Close()
    } else {
        return false, 0, fmt.Sprintf("%v", clerr), ""
    }

    // Perform the query
    q := influx.NewQuery("SELECT " + the_query, SafecastDb, "ns")
    q.Chunked = true
    q.ChunkSize = 100
    response, qerr := cl.Query(q)
    if qerr != nil {
        return false, 0, fmt.Sprintf("%v", qerr), ""
    }

    // Exit if an err
    if response.Error() != nil {
        return false, 0, fmt.Sprintf("%v", response.Error()), ""
    }

	// Generate the filename
    file := time.Now().UTC().Format("2006-01-02") + "-" + the_device + "-" + the_user
	if is_csv {
		file = file + ".csv"
	} else {
		file = file + ".json"
	}
    filename := SafecastDirectory() + TTInfluxQueryPath + "/"  + file

    // Create the output file
	var fd *os.File
	var err error
	if is_csv {
		fd, err = csvNew(filename)
	} else {
		fd, err = jsonNew(filename)
	}
    if err != nil {
        return false, 0, fmt.Sprintf("cannot create file: %s", err), ""
    }

    // Convert to CSV
    rows := InfluxResultsToFile(response, is_csv, fd)

	// Close the file
	if is_csv {
		csvClose(fd)
	} else {
		jsonClose(fd)
	}

	// Exit if no results
    if rows == 0 {
        return false, 0, "No results.", ""
    }

    // Return the URL to the file
    url := fmt.Sprintf("http://%s%s%s", TTServerHTTPAddress, TTServerTopicQueryResults, file)
    return true, rows, url, filename

}
