// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Safecast API data structures
package main

// Loc is Device Location Data
type Loc struct {
	Lat *float32			`json:"loc_lat,omitempty"`
	Lon *float32			`json:"loc_lon,omitempty"`
	Alt *float32			`json:"loc_alt,omitempty"`
	MotionBegan *string		`json:"loc_when_motion_began,omitempty"`
	Olc *string				`json:"loc_olc,omitempty"`
}

// Env is Device Basic Environmental Data
type Env struct {
    Temp *float32			`json:"env_temp,omitempty"`
    Humid *float32			`json:"env_humid,omitempty"`
    Press *float32			`json:"env_press,omitempty"`
}

// Bat is Device Battery Performance Data
type Bat struct {
	Voltage *float32		`json:"bat_voltage,omitempty"`
    Current *float32		`json:"bat_current,omitempty"`
	Charge *float32			`json:"bat_charge,omitempty"`
}

// Lnd is support for LND Geiger Tubes
type Lnd struct {
	// Unshielded LND 7318
    U7318 *float32			`json:"lnd_7318u,omitempty"`
	// Shielded LND 7318
    C7318 *float32			`json:"lnd_7318c,omitempty"`
	// Energy-compensated LND 7128
    EC7128 *float32			`json:"lnd_7128ec,omitempty"`
	// Unshielded LND 712
    U712 *float32			`json:"lnd_712u,omitempty"`
	// Water-attenuated LND LND 78017
    W78017 *float32			`json:"lnd_78017w,omitempty"`
}

// Pms is for Plantower Air Sensor Data
type Pms struct {
    Pm01_0 *float32			`json:"pms_pm01_0,omitempty"`
    Pm02_5 *float32			`json:"pms_pm02_5,omitempty"`
    Pm10_0 *float32			`json:"pms_pm10_0,omitempty"`
    Std01_0 *float32		`json:"pms_std01_0,omitempty"`
    Std02_5 *float32		`json:"pms_std02_5,omitempty"`
    Std10_0 *float32		`json:"pms_std10_0,omitempty"`
    Count00_30 *uint32		`json:"pms_c00_30,omitempty"`
    Count00_50 *uint32		`json:"pms_c00_50,omitempty"`
    Count01_00 *uint32		`json:"pms_c01_00,omitempty"`
    Count02_50 *uint32		`json:"pms_c02_50,omitempty"`
    Count05_00 *uint32		`json:"pms_c05_00,omitempty"`
    Count10_00 *uint32		`json:"pms_c10_00,omitempty"`
    CountSecs *uint32		`json:"pms_csecs,omitempty"`
}

// Pms2 is for an auxiliary Plantower Air Sensor Data
type Pms2 struct {
    Pm01_0 *float32			`json:"pms2_pm01_0,omitempty"`
    Pm02_5 *float32			`json:"pms2_pm02_5,omitempty"`
    Pm10_0 *float32			`json:"pms2_pm10_0,omitempty"`
    Std01_0 *float32		`json:"pms2_std01_0,omitempty"`
    Std02_5 *float32		`json:"pms2_std02_5,omitempty"`
    Std10_0 *float32		`json:"pms2_std10_0,omitempty"`
    Count00_30 *uint32		`json:"pms2_c00_30,omitempty"`
    Count00_50 *uint32		`json:"pms2_c00_50,omitempty"`
    Count01_00 *uint32		`json:"pms2_c01_00,omitempty"`
    Count02_50 *uint32		`json:"pms2_c02_50,omitempty"`
    Count05_00 *uint32		`json:"pms2_c05_00,omitempty"`
    Count10_00 *uint32		`json:"pms2_c10_00,omitempty"`
    CountSecs *uint32		`json:"pms2_csecs,omitempty"`
}

// Opc is for Alphasense OPC-N2 Air Sensor Data
type Opc struct {
    Pm01_0 *float32			`json:"opc_pm01_0,omitempty"`
    Pm02_5 *float32			`json:"opc_pm02_5,omitempty"`
    Pm10_0 *float32			`json:"opc_pm10_0,omitempty"`
    Std01_0 *float32		`json:"opc_std01_0,omitempty"`
    Std02_5 *float32		`json:"opc_std02_5,omitempty"`
    Std10_0 *float32		`json:"opc_std10_0,omitempty"`
    Count00_38 *uint32		`json:"opc_c00_38,omitempty"`
    Count00_54 *uint32		`json:"opc_c00_54,omitempty"`
    Count01_00 *uint32		`json:"opc_c01_00,omitempty"`
    Count02_10 *uint32		`json:"opc_c02_10,omitempty"`
    Count05_00 *uint32		`json:"opc_c05_00,omitempty"`
    Count10_00 *uint32		`json:"opc_c10_00,omitempty"`
    CountSecs *uint32		`json:"opc_csecs,omitempty"`
}

// Dev contains General Device Statistics
type Dev struct {
	Test *bool				`json:"dev_test,omitempty"`
	Motion *bool			`json:"dev_motion,omitempty"`
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
    MotionEvents *uint32	`json:"dev_motion_events,omitempty"`
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
    Temp *float32			`json:"dev_temp,omitempty"`
    Humid *float32			`json:"dev_humid,omitempty"`
    Press *float32			`json:"dev_press,omitempty"`
    ErrorsOpc *uint32		`json:"dev_err_opc,omitempty"`
    ErrorsPms *uint32		`json:"dev_err_pms,omitempty"`
    ErrorsPms2 *uint32		`json:"dev_err_pms2,omitempty"`
    ErrorsBme0 *uint32		`json:"dev_err_bme0,omitempty"`
    ErrorsBme1 *uint32		`json:"dev_err_bme1,omitempty"`
    ErrorsLora *uint32		`json:"dev_err_lora,omitempty"`
    ErrorsFona *uint32		`json:"dev_err_fona,omitempty"`
    ErrorsGeiger *uint32	`json:"dev_err_geiger,omitempty"`
    ErrorsMax01 *uint32		`json:"dev_err_max01,omitempty"`
    ErrorsUgps *uint32		`json:"dev_err_ugps,omitempty"`
    ErrorsTwi *uint32		`json:"dev_err_twi,omitempty"`
    ErrorsTwiInfo *string	`json:"dev_err_twi_info,omitempty"`
    ErrorsLis *uint32		`json:"dev_err_lis,omitempty"`
    ErrorsSpi *uint32		`json:"dev_err_spi,omitempty"`
    ErrorsConnectLora *uint32 `json:"dev_err_con_lora,omitempty"`
    ErrorsConnectFona *uint32 `json:"dev_err_con_fona,omitempty"`
    ErrorsConnectWireless *uint32 `json:"dev_err_con_wireless,omitempty"`
    ErrorsConnectData *uint32 `json:"dev_err_con_data,omitempty"`
    ErrorsConnectService *uint32 `json:"dev_err_con_service,omitempty"`
    ErrorsConnectGateway *uint32 `json:"dev_err_con_gateway,omitempty"`
    CommsAntFails *uint32	`json:"dev_comms_ant_fails,omitempty"`
    OvercurrentEvents *uint32	`json:"dev_overcurrent_events,omitempty"`
    ErrorsMtu *uint32		`json:"dev_err_mtu,omitempty"`
    Seqno *uint32			`json:"dev_seqno,omitempty"`
}

// Gateway is Lora home gateway-supplied metadata
type Gateway struct {
	ReceivedAt *string		`json:"gateway_received,omitempty"`
	SNR *float32			`json:"gateway_lora_snr,omitempty"`
	Lat *float32			`json:"gateway_loc_lat,omitempty"`
	Lon *float32			`json:"gateway_loc_lon,omitempty"`
	Alt *float32			`json:"gateway_loc_alt,omitempty"`
}

// Service contains service metadata
type Service struct {
	UploadedAt *string		`json:"service_uploaded,omitempty"`
    Transport *string		`json:"service_transport,omitempty"`
	HashMd5 *string			`json:"service_md5,omitempty"`
	Handler *string			`json:"service_handler,omitempty"`
}

// Note that this structure has been designed so that we could convert, at a later date,
// to a structured JSON out put by modifying these definitions by changing this of this form:
//    *Location `json:",omitempty"`
// to this form, using the data type as the fiel name and specifying a json field name..
//	  Location *Location `json:"location,omitempty"`

// SafecastData is our primary in-memory data structure for a Safecast message
type SafecastData struct {

	// The new device ID that will ultimatley replace DeviceID
	// because of the fact that DeviceID is only 32-bits and will eventually
	// have conflicts.
    DeviceURN *string		`json:"device_urn,omitempty"`

	// Data generated by the device itself and untouched in transit
    DeviceID *uint32		`json:"device,omitempty"`
    CapturedAt *string		`json:"when_captured,omitempty"`
	*Loc					`json:",omitempty"`
	*Env					`json:",omitempty"`
	*Bat					`json:",omitempty"`
	*Lnd					`json:",omitempty"`
	*Pms					`json:",omitempty"`
	*Pms2					`json:",omitempty"`
	*Opc					`json:",omitempty"`
	*Dev					`json:",omitempty"`

	// Metadata added as the above is being
	*Gateway				`json:",omitempty"`
	*Service				`json:",omitempty"`

}
