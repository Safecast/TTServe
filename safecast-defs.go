// Safecast API data structures

package main

type SafecastData struct {
    CapturedAt		string `json:"captured_at,omitempty"`   // 2016-02-20T14:02:25Z
    ChannelID		string `json:"channel_id,omitempty"`    // nil
    DeviceID		string `json:"device_id,omitempty"`     // 140
    DeviceTypeID	string `json:"devicetype_id,omitempty"` // nil
    Height			string `json:"height,omitempty"`        // 123
    ID				string `json:"id,omitempty"`            // 972298
    LocationName	string `json:"location_name,omitempty"` // nil
    OriginalID		string `json:"original_id,omitempty"`   // 972298
    SensorID		string `json:"sensor_id,omitempty"`     // nil
    StationID		string `json:"station_id,omitempty"`    // nil
    Unit			string `json:"unit,omitempty"`          // cpm
    UserID			string `json:"user_id,omitempty"`       // 304
    Value			string `json:"value,omitempty"`         // 36
    Latitude		string `json:"latitude,omitempty"`      // 37.0105
    Longitude		string `json:"longitude,omitempty"`     // 140.9253
    BatVoltage		string `json:"bat_voltage,omitempty"`   // 0-N volts
    BatSOC			string `json:"bat_soc,omitempty"`       // 0%-100%
    WirelessSNR		string `json:"wireless_snr,omitempty"`  // -127db to +127db
    EnvTemp			string `json:"env_temp,omitempty"`      // Degrees centigrade
    EnvHumid		string `json:"env_humid,omitempty"`     // Percent RH
    EnvPress		string `json:"env_press,omitempty"`     // Pascals
    PmsPm01_0		string `json:"pms_pm01_0,omitempty"`
    PmsPm02_5		string `json:"pms_pm02_5,omitempty"`
    PmsPm10_0		string `json:"pms_pm10_0,omitempty"`
    PmsC00_30		string `json:"pms_c00_30,omitempty"`
    PmsC00_50		string `json:"pms_c00_50,omitempty"`
    PmsC01_00		string `json:"pms_c01_00,omitempty"`
    PmsC02_50		string `json:"pms_c02_50,omitempty"`
    PmsC05_00		string `json:"pms_c05_00,omitempty"`
    PmsC10_00		string `json:"pms_c10_00,omitempty"`
    PmsCsecs		string `json:"pms_csecs,omitempty"`
    OpcPm01_0		string `json:"opc_pm01_0,omitempty"`
    OpcPm02_5		string `json:"opc_pm02_5,omitempty"`
    OpcPm10_0		string `json:"opc_pm10_0,omitempty"`
    OpcC00_38		string `json:"opc_c00_38,omitempty"`
    OpcC00_54		string `json:"opc_c00_54,omitempty"`
    OpcC01_00		string `json:"opc_c01_00,omitempty"`
    OpcC02_10		string `json:"opc_c02_10,omitempty"`
    OpcC05_00		string `json:"opc_c05_00,omitempty"`
    OpcC10_00		string `json:"opc_c10_00,omitempty"`
    OpcCsecs		string `json:"opc_csecs,omitempty"`
}

// These are strings used as the "unit" for the extended safecast uploads, and
// they should be maintained so that they are identical to the json: field names above.
const UnitStats string = "stats"
const UnitMessage string = "message"
const UnitCPM string = "cpm"
const UnitBatVoltage string = "bat_voltage"
const UnitBatSOC string = "bat_soc"
const UnitEnvTemp string = "env_temp"
const UnitEnvHumid string = "env_humid"
const UnitEnvPress string = "env_press"
const UnitWirelessSNR string = "wireless_snr"
const UnitPmsPm01_0 string = "pms_pm01_0"
const UnitPmsPm02_5 string = "pms_pm02_5"
const UnitPmsPm10_0 string = "pms_pm10_0"
const UnitPmsC00_30 string = "pms_c00_30"
const UnitPmsC00_50 string = "pms_c00_50"
const UnitPmsC01_00 string = "pms_c01_00"
const UnitPmsC02_50 string = "pms_c02_50"
const UnitPmsC05_00 string = "pms_c05_00"
const UnitPmsC10_00 string = "pms_c10_00"
const UnitPmsCsecs string = "pms_csecs"
const UnitOpcPm01_0 string = "opc_pm01_0"
const UnitOpcPm02_5 string = "opc_pm02_5"
const UnitOpcPm10_0 string = "opc_pm10_0"
const UnitOpcC00_38 string = "opc_c00_38"
const UnitOpcC00_54 string = "opc_c00_54"
const UnitOpcC01_00 string = "opc_c01_00"
const UnitOpcC02_10 string = "opc_c02_10"
const UnitOpcC05_00 string = "opc_c05_00"
const UnitOpcC10_00 string = "opc_c10_00"
const UnitOpcCsecs string = "opc_csecs"

//
