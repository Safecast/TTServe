// Safecast "value file" handling
package main

import (
    "os"
    "net/http"
    "fmt"
    "io/ioutil"
    "strings"
    "encoding/json"
)

// The data structure for the "Value" files
type SafecastValue struct {
    SafecastData            `json:"current_values,omitempty"`
    LocationHistory         [5]SafecastData `json:"location_history,omitempty"`
    GeigerHistory           [5]SafecastData `json:"geiger_history,omitempty"`
    OpcHistory              [5]SafecastData `json:"opc_history,omitempty"`
    PmsHistory              [5]SafecastData `json:"pms_history,omitempty"`
    IPInfo                  IPInfoData      `json:"transport_ip_info,omitempty"`
}

// Get the current value
func SafecastReadValue(deviceID uint32) (isAvail bool, sv SafecastValue) {
    value := SafecastValue{}

    // Generate the filename, which we'll use twice
    filename := SafecastDirectory() + TTServerValuePath + "/" + fmt.Sprintf("%d", deviceID) + ".json"

    // Read the file if it exists
    file, err := ioutil.ReadFile(filename)
    if err != nil {
        value = SafecastValue{}
        value.DeviceID = uint64(deviceID);
        return false, value
    }

    // Read it as JSON
    err = json.Unmarshal(file, &value)
    if err != nil {
        value = SafecastValue{}
        value.DeviceID = uint64(deviceID);
        return false, value
    }

    // Got it
    return true, value

}

// Save the last value in a file
func SafecastWriteValue(UploadedAt string, sc SafecastData) {
    var ChangedLocation = false
    var ChangedPms = false
    var ChangedOpc = false
    var ChangedGeiger = false

    // Use the supplied upload time as our modification time
    sc.UploadedAt = &UploadedAt

    // Read the current value, or a blank value structure if it's blank
    _, value := SafecastReadValue(uint32(sc.DeviceID))

    // Update the current values, but only if modified
    if sc.UploadedAt != nil {
        value.UploadedAt = sc.UploadedAt
    }
    if sc.CapturedAt != nil {
        value.CapturedAt = sc.CapturedAt
    }
    if sc.Bat != nil {
        var bat Bat
        if value.Bat == nil {
            value.Bat = &bat
        }
        if sc.Voltage != nil {
            value.Bat.Voltage = sc.Bat.Voltage
        }
        if sc.Current != nil {
            value.Bat.Current = sc.Bat.Current
        }
        if sc.Charge != nil {
            value.Bat.Charge = sc.Bat.Charge
        }
    }
    if sc.Env != nil {
        var env Env
        if value.Env == nil {
            value.Env = &env
        }
        if sc.Temp != nil {
            value.Env.Temp = sc.Env.Temp
        }
        if sc.Humid != nil {
            value.Env.Humid = sc.Env.Humid
        }
        if sc.Press != nil {
            value.Env.Press = sc.Env.Press
        }
    }
    if sc.Net != nil {
        var net Net
        if value.Net == nil {
            value.Net = &net
        }
        if sc.SNR != nil {
            value.Net.SNR = sc.Net.SNR
        }
        if sc.Transport != nil {
            value.Net.Transport = sc.Net.Transport
        }
    }
    if sc.Loc != nil {
        var loc Loc
        if (value.Loc == nil) {
            value.Loc = &loc
        }
        if (value.Loc.Lat != sc.Loc.Lat || value.Loc.Lon != sc.Loc.Lon) {
            ChangedLocation = true
        }
        value.Loc = sc.Loc
    }
    if sc.Pms != nil {
        var pms Pms
        if (value.Pms == nil) {
            value.Pms = &pms
        }
        if sc.Pms.Pm01_0 != nil {
            value.Pms.Pm01_0 = sc.Pms.Pm01_0
        }
        if sc.Pms.Pm02_5 != nil {
            value.Pms.Pm02_5 = sc.Pms.Pm02_5
        }
        if sc.Pms.Pm10_0 != nil {
            value.Pms.Pm10_0 = sc.Pms.Pm10_0
        }
        if sc.Pms.CountSecs != nil {
            value.Pms.Count00_30 = sc.Pms.Count00_30
            value.Pms.Count00_50 = sc.Pms.Count00_50
            value.Pms.Count01_00 = sc.Pms.Count01_00
            value.Pms.Count02_50 = sc.Pms.Count02_50
            value.Pms.Count05_00 = sc.Pms.Count05_00
            value.Pms.Count10_00 = sc.Pms.Count10_00
            value.Pms.CountSecs = sc.Pms.CountSecs
        }
        ChangedPms = true
    }
    if sc.Opc != nil {
        var opc Opc
        if (value.Opc == nil) {
            value.Opc = &opc
        }
        if sc.Opc.Pm01_0 != nil {
            value.Opc.Pm01_0 = sc.Opc.Pm01_0
        }
        if sc.Opc.Pm02_5 != nil {
            value.Opc.Pm02_5 = sc.Opc.Pm02_5
        }
        if sc.Opc.Pm10_0 != nil {
            value.Opc.Pm10_0 = sc.Opc.Pm10_0
        }
        if sc.Opc.CountSecs != nil {
            value.Opc.Count00_38 = sc.Opc.Count00_38
            value.Opc.Count00_54 = sc.Opc.Count00_54
            value.Opc.Count01_00 = sc.Opc.Count01_00
            value.Opc.Count02_10 = sc.Opc.Count02_10
            value.Opc.Count05_00 = sc.Opc.Count05_00
            value.Opc.Count10_00 = sc.Opc.Count10_00
            value.Opc.CountSecs = sc.Opc.CountSecs
        }
        ChangedOpc = true
    }
    if sc.Lnd != nil {
        var lnd Lnd
        if value.Lnd == nil {
            value.Lnd = &lnd
        }
        if sc.Lnd.U7318 != nil {
            var val float32
            if value.Lnd.U7318 == nil {
                value.Lnd.U7318 = &val
            }
            if (*value.Lnd.U7318 != *sc.Lnd.U7318) {
                ChangedGeiger = true
            }
            value.Lnd.U7318 = sc.Lnd.U7318
        }
        if sc.Lnd.C7318 != nil {
            var val float32
            if value.Lnd.C7318 == nil {
                value.Lnd.C7318 = &val
            }
            if (*value.Lnd.C7318 != *sc.Lnd.C7318) {
                ChangedGeiger = true
            }
            value.Lnd.C7318 = sc.Lnd.C7318
        }
        if sc.Lnd.EC7128 != nil {
            var val float32
            if value.Lnd.EC7128 == nil {
                value.Lnd.EC7128 = &val
            }
            if (*value.Lnd.EC7128 != *sc.Lnd.EC7128) {
                ChangedGeiger = true
            }
            value.Lnd.EC7128 = sc.Lnd.EC7128
        }
    }
    if sc.Dev != nil {
        var dev Dev
        if value.Dev == nil {
            value.Dev = &dev
        }
        if sc.Dev.UptimeMinutes != nil {
            value.Dev.UptimeMinutes = sc.Dev.UptimeMinutes
        }
        if sc.Dev.AppVersion != nil {
            value.Dev.AppVersion = sc.Dev.AppVersion
        }
        if sc.Dev.DeviceParams != nil {
            value.Dev.DeviceParams = sc.Dev.DeviceParams
        }
        if sc.Dev.GpsParams != nil {
            value.Dev.GpsParams = sc.Dev.GpsParams
        }
        if sc.Dev.ServiceParams != nil {
            value.Dev.ServiceParams = sc.Dev.ServiceParams
        }
        if sc.Dev.TtnParams != nil {
            value.Dev.TtnParams = sc.Dev.TtnParams
        }
        if sc.Dev.SensorParams != nil {
            value.Dev.SensorParams = sc.Dev.SensorParams
        }
        if sc.Dev.TransmittedBytes != nil {
            value.Dev.TransmittedBytes = sc.Dev.TransmittedBytes
        }
        if sc.Dev.ReceivedBytes != nil {
            value.Dev.ReceivedBytes = sc.Dev.ReceivedBytes
        }
        if sc.Dev.CommsResets != nil {
            value.Dev.CommsResets = sc.Dev.CommsResets
        }
        if sc.Dev.CommsFails != nil {
            value.Dev.CommsFails = sc.Dev.CommsFails
        }
        if sc.Dev.CommsPowerFails != nil {
            value.Dev.CommsPowerFails = sc.Dev.CommsPowerFails
        }
        if sc.Dev.DeviceRestarts != nil {
            value.Dev.DeviceRestarts = sc.Dev.DeviceRestarts
        }
        if sc.Dev.Motiondrops != nil {
            value.Dev.Motiondrops = sc.Dev.Motiondrops
        }
        if sc.Dev.Oneshots != nil {
            value.Dev.Oneshots = sc.Dev.Oneshots
        }
        if sc.Dev.OneshotSeconds != nil {
            value.Dev.OneshotSeconds = sc.Dev.OneshotSeconds
        }
        if sc.Dev.Iccid != nil {
            value.Dev.Iccid = sc.Dev.Iccid
        }
        if sc.Dev.Cpsi != nil {
            value.Dev.Cpsi = sc.Dev.Cpsi
        }
        if sc.Dev.Dfu != nil {
            value.Dev.Dfu = sc.Dev.Dfu
        }
        if sc.Dev.DeviceLabel != nil {
            value.Dev.DeviceLabel = sc.Dev.DeviceLabel
        }
        if sc.Dev.FreeMem != nil {
            value.Dev.FreeMem = sc.Dev.FreeMem
        }
        if sc.Dev.NTPCount != nil {
            value.Dev.NTPCount = sc.Dev.NTPCount
        }
        if sc.Dev.LastFailure != nil {
            value.Dev.LastFailure = sc.Dev.LastFailure
        }
        if sc.Dev.Status != nil {
            value.Dev.Status = sc.Dev.Status
        }
    }

    // Calculate a time of the shuffle, allowing for the fact that our preferred time
    // CapturedAt may not be available.
    ShuffledAt := value.UploadedAt
    if value.CapturedAt != nil {
        ShuffledAt = value.CapturedAt
    }

    // Shuffle
    if ChangedLocation {
        for i:=len(value.LocationHistory)-1; i>0; i-- {
            value.LocationHistory[i] = value.LocationHistory[i-1]
        }
        new := SafecastData{}
        new.DeviceID = value.DeviceID
        new.CapturedAt = ShuffledAt
        new.Loc = value.Loc
        value.LocationHistory[0] = new
    }

    // Shuffle
    if ChangedPms {
        for i:=len(value.PmsHistory)-1; i>0; i-- {
            value.PmsHistory[i] = value.PmsHistory[i-1]
        }
        new := SafecastData{}
        new.DeviceID = value.DeviceID
        new.CapturedAt = ShuffledAt
        new.Pms = value.Pms
        value.PmsHistory[0] = new
    }

    // Shuffle
    if ChangedOpc {
        for i:=len(value.OpcHistory)-1; i>0; i-- {
            value.OpcHistory[i] = value.OpcHistory[i-1]
        }
        new := SafecastData{}
        new.DeviceID = value.DeviceID
        new.CapturedAt = ShuffledAt
        new.Opc = value.Opc
        value.OpcHistory[0] = new
    }

    // Shuffle
    if ChangedGeiger {
        for i:=len(value.GeigerHistory)-1; i>0; i-- {
            value.GeigerHistory[i] = value.GeigerHistory[i-1]
        }
        new := SafecastData{}
        new.DeviceID = value.DeviceID
        new.CapturedAt = ShuffledAt
        new.Lnd = value.Lnd
        value.GeigerHistory[0] = new
    }

    // If the current transport has an IP address, try to
    // get the IP info

    if value.Net != nil && value.Net.Transport != nil {
        ipInfo := IPInfoData{}
        Str1 := strings.Split(*value.Net.Transport, ":")
        IP := Str1[len(Str1)-1]
        Str2 := strings.Split(IP, ".")
        isValidIP := len(Str1) > 1 && len(Str2) == 4
        if (isValidIP) {
            response, err := http.Get("http://ip-api.com/json/" + IP)
            if err == nil {
                defer response.Body.Close()
                contents, err := ioutil.ReadAll(response.Body)
                if err == nil {
                    var info IPInfoData
                    err = json.Unmarshal(contents, &info)
                    if err == nil {
                        ipInfo = info
                    }
                }
            }
        }
        value.IPInfo = ipInfo
    }

    // Write it to the file
    filename := SafecastDirectory() + TTServerValuePath + "/" + fmt.Sprintf("%d", sc.DeviceID) + ".json"
    valueJSON, _ := json.MarshalIndent(value, "", "    ")
    fd, err := os.OpenFile(filename, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0666)
    if err == nil {
        fd.WriteString(string(valueJSON));
        fd.Close();
    }

}

// Get summary of a device
func SafecastGetSummary(DeviceID uint32) (Label string, Gps string, Summary string) {

    // Generate the filename, which we'll use twice
    filename := SafecastDirectory() + TTServerValuePath + "/" + fmt.Sprintf("%d", DeviceID) + ".json"

    // Read the file if it exists, else blank out value
    value := SafecastValue{}
    file, err := ioutil.ReadFile(filename)
    if err != nil {
        return "", "", ""
    }

    // Read it as JSON
    err = json.Unmarshal(file, &value)
    if err != nil {
        return "", "", ""
    }

    // Get the label
    label := ""
    if value.Dev != nil && value.Dev.DeviceLabel != nil {
        label = *value.Dev.DeviceLabel
    }

    gps := ""
    if value.Loc != nil {
        gps = fmt.Sprintf("<http://maps.google.com/maps?z=12&t=m&q=loc:%f+%f|gps>", value.Loc.Lat, value.Loc.Lon)
    }

    // Build the summary
    s := ""

    if value.Bat != nil && value.Bat.Voltage != nil {
        s += fmt.Sprintf("%.1fv ", *value.Bat.Voltage)
    }

    if value.Lnd != nil {
        didlnd := false
        if value.Lnd.U7318 != nil {
            s += fmt.Sprintf("%.0f", *value.Lnd.U7318)
            didlnd = true;
        }
        if value.Lnd.C7318 != nil {
            if (didlnd) {
                s += "|"
            }
            s += fmt.Sprintf("%.0f", *value.Lnd.C7318)
            didlnd = true;
        }
        if value.Lnd.EC7128 != nil {
            if (didlnd) {
                s += "|"
            }
            s += fmt.Sprintf("%.0f", *value.Lnd.EC7128)
            didlnd = true;
        }
        if (didlnd) {
            s += "cpm "
        }
    }
    if value.Opc != nil {
        if value.Opc.Pm01_0 != nil && value.Opc.Pm02_5 != nil && value.Opc.Pm10_0 != nil {
            s += fmt.Sprintf("%.1f|%.1f|%.1fug/m3 ", *value.Opc.Pm01_0, *value.Opc.Pm02_5, *value.Opc.Pm10_0)
        }
    } else if value.Pms != nil {
        if value.Pms.Pm01_0 != nil && value.Pms.Pm02_5 != nil && value.Pms.Pm10_0 != nil {
            s += fmt.Sprintf("%.1f|%.1f|%.1fug/m3 ", *value.Pms.Pm01_0, *value.Pms.Pm02_5, *value.Pms.Pm10_0)
        }
    }

    // Done
    return label, gps, s

}
