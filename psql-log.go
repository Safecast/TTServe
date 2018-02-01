// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Log to PostreSQL, and process queries
package main

import (
    "os"
    "fmt"
    "time"
    "encoding/json"
)

// Database info
const dbTable = "data"

// Default CSV columns - order etc not required, but these were defined to be identical to the format
// that was previously used by the pre-PSQL report format, which is helpful if anyone happened to develop
// spreadsheets that made assumptions about what is in which column.
const defaultCols = ".value.service_uploaded;.value.when_captured;.value.device;.value.lnd_7318u;.value.lnd_7318c;.value.lnd_7128ec;.value.loc_when_motion_began;.value.loc_olc;.value.loc_lat;.value.loc_lon;.value.loc_alt;.value.bat_voltage;.value.bat_charge;.value.bat_current;.value.env_temp;.value.env_humid;.value.env_press;.value.enc_temp;.value.enc_humid;.value.enc_press;.value.pms_pm01_0;.value.pms_std01_0;.value.pms_pm02_5;.value.pms_std02_5;.value.pms_pm10_0;.value.pms_std10_0;.value.pms_c00_30;.value.pms_c00_50;.value.pms_c01_00;.value.pms_c02_50;.value.pms_c05_00;.value.pms_c10_00;.value.pms_csecs;.value.opc_pm01_0;.value.opc_std01_0;.value.opc_pm02_5;.value.opc_std02_5;.value.opc_pm10_0;.value.opc_std10_0;.value.opc_c00_38;.value.opc_c00_54;.value.opc_c01_00;.value.opc_c02_10;.value.opc_c05_00;.value.opc_c10_00;.value.opc_csecs;.value.service_transport;.value.gateway_lora_snr;.value.service_handler;.value.dev_test;.value.dev_label;.value.dev_uptime;.value.dev_firmware;.value.dev_cfgdev;.value.dev_cfgsvc;.value.dev_cfgttn;.value.dev_cfggps;.value.dev_cfgsen;.value.dev_transmitted_bytes;.value.dev_received_bytes;.value.dev_comms_resets;.value.dev_comms_failures;.value.dev_comms_power_fails;.value.dev_restarts;.value.dev_motion_events;.value.dev_oneshots;.value.dev_oneshot_seconds;.value.dev_iccid;.value.dev_cpsi;.value.dev_dfu;.value.dev_free_memory;.value.dev_ntp_count;.value.dev_last_failure;.value.dev_status;.value.dev_module_lora;.value.dev_module_fona;.value.dev_err_opc;.value.dev_err_pms;.value.dev_err_bme0;.value.dev_err_bme1;.value.dev_err_lora;.value.dev_err_fona;.value.dev_err_geiger;.value.dev_err_max01;.value.dev_err_ugps;.value.dev_err_twi;.value.dev_err_twi_info;.value.dev_err_lis;.value.dev_err_spi;.value.dev_err_con_lora;.value.dev_err_con_fona;.value.dev_err_con_wireless;.value.dev_err_con_data;.value.dev_err_con_service;.value.dev_err_con_gateway;.value.dev_comms_ant_fails;.value.lnd_712u;.value.lnd_78017w;.value.dev_motion;.value.dev_overcurrent_events;.value.dev_err_mtu;.value.dev_seqno"

// Query the log to a file
func logQuery(qstr string, isCSV bool, user string) (numResults int, url string, filename string, err error) {

    // Bail if the data table isn't provisioned
    err = dbValidateTable(dbTable, true)
    if err != nil {
        return
    }

    // Unmarshal the text into a JSON query
    q := DbQuery{}
    err = json.Unmarshal([]byte(qstr), &q)
    if err != nil {
        err = fmt.Errorf("query format not recognized: %s: %s\n%v\n", qstr, err, []byte(qstr))
		return
    }

	// Validate the format
    if (isCSV && q.Format == "") {
        q.Format = "csv"
    } else {
        q.Format = "json"
    }
	switch (q.Format) {
	case "csv":
		isCSV = true
	case "json":
		isCSV = false
	default:
		err = fmt.Errorf("unrecognized query format: %s", q.Format)
		return
	}
	
	// If no columns specified, allow it in JSON (which dumps the whole thing), but not in CSV
	if (q.Columns == "") {
		if isCSV {
			q.Columns = defaultCols
		} else {
			q.Columns = ".value"
		}
	}

    // Build a PSQL query
    sqlQuery, qerr := dbBuildQuery(dbTable, &q)
    if qerr != nil {
		err = fmt.Errorf("cannot build query: %s\n", qerr)
        return 
    }

    // Generate the filename
    file := time.Now().UTC().Format("2006-01-02-15-04-05") + "-" + user
    file = file + "." + q.Format
    url = fmt.Sprintf("http://%s%s%s", TTServerHTTPAddress, TTServerTopicQueryResults, file)
    filename = TTQueryPath + "/" + file

    // Create the output file
    var fd *os.File
    if isCSV {
        fd, err = csvNew(SafecastDirectory() + filename)
    } else {
        fd, err = jsonNew(SafecastDirectory() + filename)
    }
    if err != nil {
        err = fmt.Errorf("cannot create file %s: %s", file, err)
		return
    }

    var response string
    numResults, _, response, err = dbQueryToWriter(fd, sqlQuery, false, &q)
    if err != nil {
        fmt.Printf("QueryWriter error: %s\n", err)
    }
    fmt.Printf("QueryWriter response: %s file %s\n", response, file)

    // Done, after writing the footer
    if isCSV {
        csvClose(fd)
    } else {
        jsonClose(fd)
    }

    return 
}

// LogToDb logs the specific safecast data point to the database
func LogToDb(sd SafecastData) bool {

    // Bail if the data table isn't provisioned
    err := dbValidateTable(dbTable, true)
    if err != nil {
        fmt.Printf("error opening table '%s': %s\n", dbTable, err)
        return false
    }

    // Add the object with a unique ID
    err = dbAddObject(dbTable, "", sd)
    if err != nil {
        fmt.Printf("db table error adding record to '%s': %s\n", dbTable, err)
        return false
    }

    // Done
    return true

}
