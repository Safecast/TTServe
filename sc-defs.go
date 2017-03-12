// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Safecast API data structures
package main

// Service metadata
type Service struct {
	UploadedAt *string		`json:"service_uploaded,omitempty"`
    Transport *string		`json:"service_transport,omitempty"`
	HashMd5 *string			`json:"service_md5,omitempty"`
	Handler *string			`json:"service_handler,omitempty"`
}

// Gateway-supplied metadata
type Gateway struct {
	SNR *float32			`json:"gateway_lora_snr,omitempty"`
	ReceivedAt *string		`json:"gateway_received,omitempty"`
	Lat *float32			`json:"gateway_loc_lat,omitempty"`
	Lon *float32			`json:"gateway_loc_lon,omitempty"`
	Alt *float32			`json:"gateway_loc_alt,omitempty"`
}

// Device Location Data - Lat and Lon required, Alt is optional
type Loc struct {
	Lat float32				`json:"loc_lat"`
	Lon float32				`json:"loc_lon"`
	Alt *float32			`json:"loc_alt,omitempty"`
	Motion bool				`json:"loc_motion,omitempty"`
}

// Device Battery Performance Data - all are optional
type Bat struct {
	Voltage *float32		`json:"bat_voltage,omitempty"`
    Current *float32		`json:"bat_current,omitempty"`
	Charge *float32			`json:"bat_charge,omitempty"`
}

// Device Basic Environmental Data - all are optional
type Env struct {
    Temp *float32			`json:"env_temp,omitempty"`
    Humid *float32			`json:"env_humid,omitempty"`
    Press *float32			`json:"env_press,omitempty"`
}

// LND Geiger Tubes - both are optional
type Lnd struct {
	// Unshielded LND 7318
    U7318 *float32			`json:"lnd_7318u,omitempty"`
	// Shielded LND 7318
    C7318 *float32			`json:"lnd_7318c,omitempty"`
	// Energy-compensated LND 7128
    EC7128 *float32			`json:"lnd_7128ec,omitempty"`
}

// Plantower Air Sensor Data - PM all are optional
type Pms struct {
    Pm01_0 *float32			`json:"pms_pm01_0,omitempty"`
    Pm02_5 *float32			`json:"pms_pm02_5,omitempty"`
    Pm10_0 *float32			`json:"pms_pm10_0,omitempty"`
    Count00_30 *uint32		`json:"pms_c00_30,omitempty"`
    Count00_50 *uint32		`json:"pms_c00_50,omitempty"`
    Count01_00 *uint32		`json:"pms_c01_00,omitempty"`
    Count02_50 *uint32		`json:"pms_c02_50,omitempty"`
    Count05_00 *uint32		`json:"pms_c05_00,omitempty"`
    Count10_00 *uint32		`json:"pms_c10_00,omitempty"`
    CountSecs *uint32		`json:"pms_csecs,omitempty"`
}

// Alphasense OPC-N2 Air Sensor Data - all are optional
type Opc struct {
    Pm01_0 *float32			`json:"opc_pm01_0,omitempty"`
    Pm02_5 *float32			`json:"opc_pm02_5,omitempty"`
    Pm10_0 *float32			`json:"opc_pm10_0,omitempty"`
    Count00_38 *uint32		`json:"opc_c00_38,omitempty"`
    Count00_54 *uint32		`json:"opc_c00_54,omitempty"`
    Count01_00 *uint32		`json:"opc_c01_00,omitempty"`
    Count02_10 *uint32		`json:"opc_c02_10,omitempty"`
    Count05_00 *uint32		`json:"opc_c05_00,omitempty"`
    Count10_00 *uint32		`json:"opc_c10_00,omitempty"`
    CountSecs *uint32		`json:"opc_csecs,omitempty"`
}

// General Device Statistics - All Optional
type Dev struct {
    DeviceLabel *string		`json:"dev_label,omitempty"`
    UptimeMinutes *uint32	`json:"dev_uptime,omitempty"`
    AppVersion *string		`json:"dev_firmware,omitempty"`
    DeviceParams *string	`json:"dev_cfgdev,omitempty"`
    ServiceParams *string	`json:"dev_cfgsvc,omitempty"`
    TtnParams *string		`json:"dev_cfgttn,omitempty"`
    GpsParams *string		`json:"dev_cfggps,omitempty"`
    SensorParams *string	`json:"dev_cfgsen,omitempty"`
    TransmittedBytes *uint32 `json:"dev_transmitted_bytes,omitempty"`
    ReceivedBytes *uint32	`json:"dev_received_bytes,omitempty"`
    CommsResets *uint32		`json:"dev_comms_resets,omitempty"`
    CommsFails *uint32		`json:"dev_comms_failures,omitempty"`
    CommsPowerFails *uint32	`json:"dev_comms_power_fails,omitempty"`
    DeviceRestarts *uint32	`json:"dev_restarts,omitempty"`
    Motiondrops *uint32		`json:"dev_motiondrops,omitempty"`
    Oneshots *uint32		`json:"dev_oneshots,omitempty"`
    OneshotSeconds *uint32	`json:"dev_oneshot_seconds,omitempty"`
    Iccid *string			`json:"dev_iccid,omitempty"`
    Cpsi *string			`json:"dev_cpsi,omitempty"`
    Dfu *string				`json:"dev_dfu,omitempty"`
    FreeMem *uint32			`json:"dev_free_memory,omitempty"`
    NTPCount *uint32		`json:"dev_ntp_count,omitempty"`
	LastFailure *string		`json:"dev_last_failure,omitempty"`
	Status *string			`json:"dev_status,omitempty"`
	ModuleLora *string		`json:"dev_module_lora,omitempty"`
	ModuleFona *string		`json:"dev_module_fona,omitempty"`
}

// Note that this structure has been designed so that we could convert, at a later date,
// to a structured JSON out put by modifying these definitions by changing this of this form:
//    *Location `json:",omitempty"`
// to this form, using the data type as the fiel name and specifying a json field name..
//	  Location *Location `json:"location,omitempty"`

// Toggle the commment between these two lines to change flat/structured output
type SafecastData struct {

	// Data generated by the device itself and untouched in transit
    DeviceId uint64			`json:"device"`
    CapturedAt *string		`json:"when_captured,omitempty"`
	*Loc					`json:",omitempty"`
	*Env					`json:",omitempty"`
	*Bat					`json:",omitempty"`
	*Lnd					`json:",omitempty"`
	*Pms					`json:",omitempty"`
	*Opc					`json:",omitempty"`
	*Dev					`json:",omitempty"`

	// Metadata added as the above is being
	*Gateway				`json:",omitempty"`
	*Service				`json:",omitempty"`

}
