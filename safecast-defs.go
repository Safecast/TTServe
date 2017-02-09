// Safecast API data structures

package main

type SafecastDataV1 struct {
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
    BatCurrent		string `json:"bat_current,omitempty"`   // -N to +N amps
    WirelessSNR		string `json:"wireless_snr,omitempty"`	// gateway signal strength
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
    Cpm0			string `json:"cpm0,omitempty"`
    Cpm1			string `json:"cpm1,omitempty"`
    Transport		string `json:"transport,omitempty"`
}

type SafecastDataV2 struct {
	UploadedAt		string `json:"uploaded_at,omitempty"`
    CapturedAt		string `json:"captured_at,omitempty"`
    DeviceID		uint32 `json:"device_id,omitempty"`
    Height			float32 `json:"height,omitempty"`
    Latitude		float32 `json:"latitude,omitempty"`
    Longitude		float32 `json:"longitude,omitempty"`
    BatVoltage		float32 `json:"bat_voltage,omitempty"`
    BatSOC			float32 `json:"bat_soc,omitempty"`
    BatCurrent		float32 `json:"bat_current,omitempty"`
    WirelessSNR		float32 `json:"lora_snr,omitempty"`
    EnvTemp			float32 `json:"env_temp,omitempty"`
    EnvHumid		float32 `json:"env_humid,omitempty"`
    EnvPress		float32 `json:"env_press,omitempty"`
    PmsPm01_0		float32 `json:"pms_pm01_0,omitempty"`
    PmsPm02_5		float32 `json:"pms_pm02_5,omitempty"`
    PmsPm10_0		float32 `json:"pms_pm10_0,omitempty"`
    PmsC00_30		uint32 `json:"pms_c00_30,omitempty"`
    PmsC00_50		uint32 `json:"pms_c00_50,omitempty"`
    PmsC01_00		uint32 `json:"pms_c01_00,omitempty"`
    PmsC02_50		uint32 `json:"pms_c02_50,omitempty"`
    PmsC05_00		uint32 `json:"pms_c05_00,omitempty"`
    PmsC10_00		uint32 `json:"pms_c10_00,omitempty"`
    PmsCsecs		uint32 `json:"pms_csecs,omitempty"`
    OpcPm01_0		float32 `json:"opc_pm01_0,omitempty"`
    OpcPm02_5		float32 `json:"opc_pm02_5,omitempty"`
    OpcPm10_0		float32 `json:"opc_pm10_0,omitempty"`
    OpcC00_38		uint32 `json:"opc_c00_38,omitempty"`
    OpcC00_54		uint32 `json:"opc_c00_54,omitempty"`
    OpcC01_00		uint32 `json:"opc_c01_00,omitempty"`
    OpcC02_10		uint32 `json:"opc_c02_10,omitempty"`
    OpcC05_00		uint32 `json:"opc_c05_00,omitempty"`
    OpcC10_00		uint32 `json:"opc_c10_00,omitempty"`
    OpcCsecs		uint32 `json:"opc_csecs,omitempty"`
    Cpm0			float32 `json:"lndp_cpm,omitempty"`
    Cpm1			float32 `json:"lndc_cpm,omitempty"`
    Transport		string `json:"transport,omitempty"`
    StatsUptimeMinutes    uint32 `json:"uptime_min,omitempty"`
    StatsAppVersion       string `json:"version,omitempty"`
    StatsDeviceParams     string `json:"config,omitempty"`
    StatsTransmittedBytes uint32 `json:"transmitted_bytes,omitempty"`
    StatsReceivedBytes    uint32 `json:"received_bytes,omitempty"`
    StatsCommsResets      uint32 `json:"comms_resets,omitempty"`
    StatsCommsFails       uint32 `json:"comms_failures,omitempty"`
    StatsCommsPowerFails  uint32 `json:"comms_power_fails,omitempty"`
    StatsDeviceRestarts   uint32 `json:"restarts,omitempty"`
    StatsMotiondrops      uint32 `json:"motiondrops,omitempty"`
    StatsOneshots         uint32 `json:"oneshots,omitempty"`
    StatsOneshotSeconds   uint32 `json:"oneshot_seconds,omitempty"`
    StatsIccid            string `json:"iccid,omitempty"`
    StatsCpsi             string `json:"cpsi,omitempty"`
    StatsDfu              string `json:"dfu,omitempty"`
    StatsDeviceInfo       string `json:"label,omitempty"`
    StatsFreeMem          uint32 `json:"free_memory,omitempty"`
    StatsNTPCount         uint32 `json:"ntp_count,omitempty"`
	StatsLastFailure	  string `json:"last_failure,omitempty"`
	StatsStatus			  string `json:"status,omitempty"`
    Message				  string `json:"message,omitempty"`
}


type SafecastValue struct {
	SafecastDataV2			`json:"current_values,omitempty"`
	LocationHistory			[5]SafecastDataV2 `json:"location_history,omitempty"`
	GeigerHistory			[5]SafecastDataV2 `json:"geiger_history,omitempty"`
	OpcHistory				[5]SafecastDataV2 `json:"opc_history,omitempty"`
	PmsHistory				[5]SafecastDataV2 `json:"pms_history,omitempty"`
    TransportHistory		[5]SafecastDataV2 `json:"transport_history,omitempty"`
}

//
