// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Formats and uploads messages for Safecast
package main

import (
    "io"
    "io/ioutil"
    "net/http"
    "fmt"
    "bytes"
    "time"
    "strings"
    "strconv"
    "encoding/json"
    "crypto/md5"
    "github.com/safecast/ttproto/golang"
    "github.com/google/open-location-code/go"
)

// Debugging
const v1UploadDebug bool = false
const verboseTransactions bool = false

// Synchronous vs asynchronous V1 API requests
const v1UploadAsyncFakeResults bool = false
const v1UploadFakeResult string ="{\"id\":00000001}"

// For dealing with transaction timeouts
var httpTransactionsInProgress int = 0
var httpTransactions = 0
const httpTransactionsRecorded = 500
var httpTransactionDurations[httpTransactionsRecorded] int
var httpTransactionTimes[httpTransactionsRecorded] time.Time
var httpTransactionErrorTime string
var httpTransactionErrorUrl string
var httpTransactionErrorString string
var httpTransactionErrors = 0
var httpTransactionErrorFirst bool = true

// Checksums of recently-processed messages
type receivedMessage struct {
    checksum            uint32
    seen                time.Time
}
var recentlyReceived [25]receivedMessage

// Process an inbound Safecast message, as an asynchronous goroutine
func SendSafecastMessage(req IncomingAppReq, msg ttproto.Telecast, checksum uint32) {

    // Discard it if it's a duplicate
    if isDuplicate(checksum) {
        fmt.Printf("%s DISCARDING duplicate message\n", time.Now().Format(logDateFormat));
        return
    }

    // Process stamps by adding or removing fields from the message
    if (!stampSetOrApply(&msg)) {
        fmt.Printf("%s DISCARDING un-stampable message\n", time.Now().Format(logDateFormat));
        return
    }

    // This is the ONLY required field
    if msg.DeviceId == nil {
        fmt.Printf("%s DISCARDING message with no DeviceId\n", time.Now().Format(logDateFormat));
        return
    }

    // Generate the fields common to all uploads to safecast
    sd := SafecastData{}

    sd.DeviceId = uint64(msg.GetDeviceId())

    // CapturedAt
    if msg.CapturedAt != nil {
        sd.CapturedAt = msg.CapturedAt
    } else if msg.CapturedAtDate != nil && msg.CapturedAtTime != nil {
        var i64 uint64
        var offset uint32 = 0
        if (msg.CapturedAtOffset != nil) {
            offset = msg.GetCapturedAtOffset()
        }
        s := fmt.Sprintf("%06d%06d", msg.GetCapturedAtDate(), msg.GetCapturedAtTime())
        i64, _ = strconv.ParseUint(fmt.Sprintf("%c%c", s[0], s[1]), 10, 32)
        day := uint32(i64)
        i64, _ = strconv.ParseUint(fmt.Sprintf("%c%c", s[2], s[3]), 10, 32)
        month := uint32(i64)
        i64, _ = strconv.ParseUint(fmt.Sprintf("%c%c", s[4], s[5]), 10, 32)
        year := uint32(i64) + 2000
        i64, _ = strconv.ParseUint(fmt.Sprintf("%c%c", s[6], s[7]), 10, 32)
        hour := uint32(i64)
        i64, _ = strconv.ParseUint(fmt.Sprintf("%c%c", s[8], s[9]), 10, 32)
        minute := uint32(i64)
        i64, _ = strconv.ParseUint(fmt.Sprintf("%c%c", s[10], s[11]), 10, 32)
        second := uint32(i64)
        tbefore := time.Date(int(year), time.Month(month), int(day), int(hour), int(minute), int(second), 0, time.UTC)
        tafter := tbefore.Add(time.Duration(offset) * time.Second)
        tstr := tafter.UTC().Format("2006-01-02T15:04:05Z")
        sd.CapturedAt = &tstr
    }

    // Loc
    if msg.Latitude != nil || msg.Longitude != nil || msg.Motion != nil {
        var loc Loc
        if msg.Latitude != nil && msg.Longitude != nil {
            Olc := olc.Encode(float64(msg.GetLatitude()), float64(msg.GetLongitude()), 0)
            loc.Olc = &Olc;
        }
        if msg.Latitude != nil {
            loc.Lat = msg.GetLatitude()
        }
        if msg.Longitude != nil {
            loc.Lon = msg.GetLongitude()
        }
        if msg.Altitude != nil {
            var alt float32
            alt = float32(msg.GetAltitude())
            loc.Alt = &alt
        }
        if msg.Motion != nil {
            mode := msg.GetMotion()
            loc.Motion = &mode
        }
        sd.Loc = &loc
    }

    // Dev
    var dev Dev
    var dodev = false

    if msg.Test != nil {
        mode := msg.GetTest()
        dev.Test = &mode
        dodev = true
    }
    if msg.EncTemp != nil {
        dev.Temp = msg.EncTemp
        dodev = true
    }
    if msg.EncHumid != nil {
        dev.Humid = msg.EncHumid
        dodev = true
    }
    if msg.EncPressure != nil {
        dev.Press = msg.EncPressure
        dodev = true
    }
    if msg.StatsUptimeMinutes != nil {
        mins := msg.GetStatsUptimeMinutes()
        if msg.StatsUptimeDays != nil {
            mins += msg.GetStatsUptimeDays() * 24 * 60
        }
        dev.UptimeMinutes = &mins
        dodev = true
    }
    if msg.StatsAppVersion != nil {
        dev.AppVersion = msg.StatsAppVersion
        dodev = true
    }
    if msg.StatsDeviceParams != nil {
        dev.DeviceParams = msg.StatsDeviceParams
        dodev = true
    }
    if msg.StatsTransmittedBytes != nil {
        dev.TransmittedBytes = msg.StatsTransmittedBytes
        dodev = true
    }
    if msg.StatsReceivedBytes != nil {
        dev.ReceivedBytes = msg.StatsReceivedBytes
        dodev = true
    }
    if msg.StatsCommsResets != nil {
        dev.CommsResets = msg.StatsCommsResets
        dodev = true
    }
    if msg.StatsCommsPowerFails != nil {
        dev.CommsPowerFails = msg.StatsCommsPowerFails
        dodev = true
    }
    if msg.StatsOneshots != nil {
        dev.Oneshots = msg.StatsOneshots
        dodev = true
    }
    if msg.StatsOneshotSeconds != nil {
        dev.OneshotSeconds = msg.StatsOneshotSeconds
        dodev = true
    }
    if msg.StatsMotiondrops != nil {
        dev.Motiondrops = msg.StatsMotiondrops
        dodev = true
    }
    if msg.StatsIccid != nil {
        dev.Iccid = msg.StatsIccid
        dodev = true
    }
    if msg.StatsCpsi != nil {
        dev.Cpsi = msg.StatsCpsi
        dodev = true
    }
    if msg.StatsDfu != nil {
        dev.Dfu = msg.StatsDfu
        dodev = true
    }
    if msg.StatsModuleLora != nil {
        dev.ModuleLora = msg.StatsModuleLora
        dodev = true
    }
    if msg.StatsModuleFona != nil {
        dev.ModuleFona = msg.StatsModuleFona
        dodev = true
    }
    if msg.StatsDeviceLabel != nil {
        dev.DeviceLabel = msg.StatsDeviceLabel
        dodev = true
    }
    if msg.StatsGpsParams != nil {
        dev.GpsParams = msg.StatsGpsParams
        dodev = true
    }
    if msg.StatsServiceParams != nil {
        dev.ServiceParams = msg.StatsServiceParams
        dodev = true
    }
    if msg.StatsTtnParams != nil {
        dev.TtnParams = msg.StatsTtnParams
        dodev = true
    }
    if msg.StatsSensorParams != nil {
        dev.SensorParams = msg.StatsSensorParams
        dodev = true
    }
    if msg.ErrorsOpc != nil {
        dev.ErrorsOpc = msg.ErrorsOpc
        dodev = true
    }
    if msg.ErrorsPms != nil {
        dev.ErrorsPms = msg.ErrorsPms
        dodev = true
    }
    if msg.ErrorsBme0 != nil {
        dev.ErrorsBme0 = msg.ErrorsBme0
        dodev = true
    }
    if msg.ErrorsBme1 != nil {
        dev.ErrorsBme1 = msg.ErrorsBme1
        dodev = true
    }
    if msg.ErrorsLora != nil {
        dev.ErrorsLora = msg.ErrorsLora
        dodev = true
    }
    if msg.ErrorsFona != nil {
        dev.ErrorsFona = msg.ErrorsFona
        dodev = true
    }
    if msg.ErrorsGeiger != nil {
        dev.ErrorsGeiger = msg.ErrorsGeiger
        dodev = true
    }
    if msg.ErrorsMax01 != nil {
        dev.ErrorsMax01 = msg.ErrorsMax01
        dodev = true
    }
    if msg.ErrorsUgps != nil {
        dev.ErrorsUgps = msg.ErrorsUgps
        dodev = true
    }
    if msg.ErrorsTwi != nil {
        dev.ErrorsTwi = msg.ErrorsTwi
        dodev = true
    }
    if msg.ErrorsTwiInfo != nil {
        dev.ErrorsTwiInfo = msg.ErrorsTwiInfo
        dodev = true
    }
    if msg.ErrorsLis != nil {
        dev.ErrorsLis = msg.ErrorsLis
        dodev = true
    }
    if msg.ErrorsSpi != nil {
        dev.ErrorsSpi = msg.ErrorsSpi
        dodev = true
    }
    if msg.ErrorsConnectLora != nil {
        dev.ErrorsConnectLora = msg.ErrorsConnectLora
        dodev = true
    }
    if msg.ErrorsConnectFona != nil {
        dev.ErrorsConnectFona = msg.ErrorsConnectFona
        dodev = true
    }
    if msg.ErrorsConnectWireless != nil {
        dev.ErrorsConnectWireless = msg.ErrorsConnectWireless
        dodev = true
    }
    if msg.ErrorsConnectData != nil {
        dev.ErrorsConnectData = msg.ErrorsConnectData
        dodev = true
    }
    if msg.ErrorsConnectService != nil {
        dev.ErrorsConnectService = msg.ErrorsConnectService
        dodev = true
    }

    if dodev {
        sd.Dev = &dev
    }

    // Bat
    var bat Bat
    var dobat = false

    if msg.BatVoltage != nil {
        bat.Voltage = msg.BatVoltage
        dobat = true
    }
    if msg.BatSoc != nil {
        bat.Charge = msg.BatSoc
        dobat = true;
    }
    if msg.BatCurrent != nil {
        bat.Current = msg.BatCurrent
        dobat = true;
    }

    if dobat {
        sd.Bat = &bat
    }

    // Env
    var env Env
    var doenv = false

    if msg.EnvTemp != nil {
        env.Temp = msg.EnvTemp
        doenv = true
    }
    if msg.EnvHumid != nil {
        env.Humid = msg.EnvHumid
        doenv = true
    }
    if msg.EnvPressure != nil {
        env.Press = msg.EnvPressure
        doenv = true
    }

    if doenv {
        sd.Env = &env
    }

    // Service
    var svc Service
    var dosvc = false

    if req.SvUploadedAt != "" {
        svc.UploadedAt = &req.SvUploadedAt
        dosvc = true
    }
    if req.SvTransport != "" {
        svc.Transport = &req.SvTransport
        dosvc = true
    }

    if dosvc {
        sd.Service = &svc
    }

    // Gateway
    var gate Gateway
    var dogate = false

    if msg.WirelessSnr != nil {
        gate.SNR = msg.WirelessSnr
        dogate = true
    } else if req.GwSnr != nil {
        gate.SNR = req.GwSnr
        dogate = true
    }
    if req.GwReceivedAt != nil {
        gate.ReceivedAt = req.GwReceivedAt
        dogate = true
    }
    if req.GwLatitude != nil {
        gate.Lat = req.GwLatitude
        dogate = true
    }
    if req.GwLongitude != nil {
        gate.Lon = req.GwLongitude
        dogate = true
    }
    if req.GwAltitude != nil {
        gate.Alt = req.GwAltitude
        dogate = true
    }

    if dogate {
        sd.Gateway = &gate
    }

    // Pms
    var pms Pms
    var dopms = false

    if msg.PmsPm01_0 != nil && msg.PmsPm02_5 != nil && msg.PmsPm10_0 != nil {
        Pm01_0 := float32(msg.GetPmsPm01_0())
        pms.Pm01_0 = &Pm01_0
        Pm02_5 := float32(msg.GetPmsPm02_5())
        pms.Pm02_5 = &Pm02_5
        Pm10_0 := float32(msg.GetPmsPm10_0())
        pms.Pm10_0 = &Pm10_0
        dopms = true
    }

    if dopms {
        if msg.PmsC00_30 != nil {
            pms.Count00_30 = msg.PmsC00_30
        }
        if msg.PmsC00_50 != nil {
            pms.Count00_50 = msg.PmsC00_50
        }
        if msg.PmsC01_00 != nil {
            pms.Count01_00 = msg.PmsC01_00
        }
        if msg.PmsC02_50 != nil {
            pms.Count02_50 = msg.PmsC02_50
        }
        if msg.PmsC05_00 != nil {
            pms.Count05_00 = msg.PmsC05_00
        }
        if msg.PmsC10_00 != nil {
            pms.Count10_00 = msg.PmsC10_00
        }
        if msg.PmsCsecs != nil {
            pms.CountSecs = msg.PmsCsecs
        }
    }

    if dopms {
        sd.Pms = &pms
    }

    // Opc
    var opc Opc
    var doopc = false

    if msg.OpcPm01_0 != nil && msg.OpcPm02_5 != nil && msg.OpcPm10_0 != nil {
        opc.Pm01_0 = msg.OpcPm01_0
        opc.Pm02_5 = msg.OpcPm02_5
        opc.Pm10_0 = msg.OpcPm10_0
        doopc = true
    }

    if doopc {
        if msg.OpcC00_38 != nil {
            opc.Count00_38 = msg.OpcC00_38
        }
        if msg.OpcC00_54 != nil {
            opc.Count00_54 = msg.OpcC00_54
        }
        if msg.OpcC01_00 != nil {
            opc.Count01_00 = msg.OpcC01_00
        }
        if msg.OpcC02_10 != nil {
            opc.Count02_10 = msg.OpcC02_10
        }
        if msg.OpcC05_00 != nil {
            opc.Count05_00 = msg.OpcC05_00
        }
        if msg.OpcC10_00 != nil {
            opc.Count10_00 = msg.OpcC10_00
        }
        if msg.OpcCsecs != nil {
            opc.CountSecs = msg.OpcCsecs
        }
    }

    if doopc {
        sd.Opc = &opc
    }

    // Lnd, assuming a pair of 7318s
    var lnd Lnd
    var dolnd = false

    if msg.Lnd_7318U != nil {
        var cpm float32 = float32(msg.GetLnd_7318U())
        lnd.U7318 = &cpm
        dolnd = true
    }
    if msg.Lnd_7318C != nil {
        var cpm float32 = float32(msg.GetLnd_7318C())
        lnd.C7318 = &cpm
        dolnd = true
    }
    if msg.Lnd_7128Ec != nil {
        var cpm float32 = float32(msg.GetLnd_7128Ec())
        lnd.EC7128 = &cpm
        dolnd = true
    }

    if dolnd {
        sd.Lnd = &lnd
    }

    // Generate the hash of the original device data
    hash := HashSafecastData(sd)
    sd.Service.HashMd5 = &hash

    // Add info about the server instance that actually did the upload
    sd.Service.Handler = &TTServeInstanceID

    // Log as accurately as we can with regard to what came in
    SafecastWriteToLogs(req.SvUploadedAt, sd)

    // Upload
    SafecastUpload(sd)

}

// Begin transaction and return the transaction ID
func beginTransaction(version string,  message1 string, message2 string) int {
    httpTransactionsInProgress += 1
    httpTransactions += 1
    transaction := httpTransactions % httpTransactionsRecorded
    httpTransactionTimes[transaction] = time.Now()
    if verboseTransactions {
        fmt.Printf("%s >>> %s [%d] %s %s\n", time.Now().Format(logDateFormat), version, transaction, message1, message2)
    }
    return transaction
}

// End transaction and issue warnings
func endTransaction(transaction int, url string, errstr string) {
    httpTransactionsInProgress -= 1
    duration := int(time.Now().Sub(httpTransactionTimes[transaction]) / time.Second)
    httpTransactionDurations[transaction] = duration

    if errstr != "" {
        httpTransactionErrors = httpTransactionErrors + 1
        if (httpTransactionErrorFirst) {
            httpTransactionErrorTime = time.Now().Format(logDateFormat)
            httpTransactionErrorUrl = url
            httpTransactionErrorString = errstr
            httpTransactionErrorFirst = false
        }
        if verboseTransactions {
            fmt.Printf("%s <<<    [%d] *** ERROR\n", time.Now().Format(logDateFormat), transaction)
        }
        ServerLog(fmt.Sprintf("After %d seconds, error uploading to %s %s\n", duration, url, errstr))
    } else {
        if verboseTransactions {
            if (duration < 5) {
                fmt.Printf("%s <<<    [%d]\n", time.Now().Format(logDateFormat), transaction);
            } else {
                fmt.Printf("%s <<<    [%d] completed after %d seconds\n", time.Now().Format(logDateFormat), transaction, duration);
            }
        }
    }

    theMin := 99999
    theMax := 0
    theTotal := 0
    theCount := 0
    for theCount < httpTransactions && theCount < httpTransactionsRecorded {
        theTotal += httpTransactionDurations[theCount]
        if httpTransactionDurations[theCount] < theMin {
            theMin = httpTransactionDurations[theCount]
        }
        if httpTransactionDurations[theCount] > theMax {
            theMax = httpTransactionDurations[theCount]
        }
        theCount += 1
    }
    theMean := theTotal / theCount

    // Output to console every time we are in a "slow mode"
    if (theMin > 5) {
        fmt.Printf("%s Safecast Upload Statistics\n", time.Now().Format(logDateFormat))
        fmt.Printf("%s *** %d total uploads since restart\n", time.Now().Format(logDateFormat), httpTransactions)
        if (httpTransactionsInProgress > 0) {
            fmt.Printf("%s *** %d uploads still in progress\n", time.Now().Format(logDateFormat), httpTransactionsInProgress)
        }
        fmt.Printf("%s *** Last %d: min=%ds, max=%ds, avg=%ds\n", time.Now().Format(logDateFormat), theCount, theMin, theMax, theMean)

    }

    // If there's a problem, output to Slack once every 25 transactions
    if (theMin > 5 && transaction == 0) {
        // If all of them have the same timeout value, the server must be down.
        s := ""
        if (theMin == theMax && theMin == theMean) {
            s = fmt.Sprintf("HTTP Upload: all of the most recent %d uploads failed. Please check the service.", theCount)
        } else {
            s = fmt.Sprintf("HTTP Upload: of the previous %d uploads, min=%ds, max=%ds, avg=%ds", theCount, theMin, theMax, theMean)
        }
        sendToSafecastOps(s, SLACK_MSG_UNSOLICITED);
    }

}

// Check to see if this is a duplicate of a message we've recently seen
func isDuplicate(checksum uint32) bool {

    // Sweep through all recent messages, looking for a duplicate in the past minute
    for i := 0; i < len(recentlyReceived); i++ {
        if recentlyReceived[i].checksum == checksum {
            if (int64(time.Now().Sub(recentlyReceived[i].seen) / time.Second) < 60) {
                return true
            }
        }
    }

    // Shift them all down
    for i := len(recentlyReceived)-1; i > 0; i-- {
        recentlyReceived[i] = recentlyReceived[i-1]
    }

    // Insert this new one
    recentlyReceived[0].checksum = checksum;
    recentlyReceived[0].seen = time.Now().UTC()
    return false

}

// Update message ages and notify
func sendSafecastCommsErrorsToSlack(PeriodMinutes uint32) {
    if (httpTransactionErrors != 0) {
        if (httpTransactionErrors == 1) {
            sendToSafecastOps(fmt.Sprintf("** Warning **  At %s UTC, one error uploading to %s:%s",
                httpTransactionErrorTime, httpTransactionErrorUrl, httpTransactionErrorString), SLACK_MSG_UNSOLICITED_OPS);
        } else {
            sendToSafecastOps(fmt.Sprintf("** Warning **  At %s UTC, %d errors uploading in %d minutes to %s:%s",
                httpTransactionErrorTime, httpTransactionErrors, PeriodMinutes, httpTransactionErrorUrl, httpTransactionErrorString), SLACK_MSG_UNSOLICITED_OPS);
        }
        httpTransactionErrors = 0
        httpTransactionErrorFirst = true;
    }
}

// Upload a Safecast data structure to the Safecast service
func SafecastV1Upload(body []byte, url string, isDev bool, unit string, value string) (fSuccess bool, result string) {

    if v1UploadAsyncFakeResults {
        go doSafecastV1Upload(body, url, isDev, unit, value)
        return true, v1UploadFakeResult
    }

    return doSafecastV1Upload(body, url, isDev, unit, value)

}


func doSafecastV1Upload(body []byte, url string, isDev bool, unit string, value string) (fSuccess bool, result string) {

    // Preset result in case of failure
    response := v1UploadFakeResult

    // Figure out what domain we're posting to
    domain := SafecastV1UploadDomain
    v1str := "V1"
    if isDev {
        domain = SafecastV1UploadDomainDev
        v1str = "D1"
    }

    // Figure out the correct request URI
    str := strings.SplitAfter(url, "?")
    query := str[len(str)-1]
    requestUri := fmt.Sprintf(SafecastV1UploadPattern, domain, query)
    if v1UploadDebug {
        fmt.Printf("****** '%s'\n%s\n", requestUri, string(body))
    }

    // Perform the transaction
    transaction := beginTransaction(v1str, unit, value)
    req, _ := http.NewRequest("POST", requestUri, bytes.NewBuffer(body))
    req.Header.Set("User-Agent", "TTSERVE")
    req.Header.Set("Content-Type", "application/json")
    httpclient := &http.Client{
        Timeout: time.Second * 15,
    }
    if isDev {
        httpclient = &http.Client{
            Timeout: time.Second * 45,
        }
    }
    resp, err := httpclient.Do(req)
    errString := ""
    if (err == nil) {
        buf, err := ioutil.ReadAll(resp.Body)
        if err == nil {
            if v1UploadDebug {
                fmt.Printf("*** Response:\n%s\n", string(buf))
            }
            // We'd like to return the response
            respstr := string(buf)
            if strings.Contains(respstr, "<head>") {
                fmt.Printf("******** Safecast V1 server response is HTML rather than JSON ********\n")
            } else {
                response = respstr
            }
        }
        resp.Body.Close()
    } else {
        // Eliminate the URL from the string because exposing the API key is not secure.
        // Empirically we've seen that the actual error message is after the rightmost colon
        errString = fmt.Sprintf("%s", err)
        s := strings.Split(errString, ":")
        errString = s[len(s)-1]
    }

    // On 2017-02-17 I disabled errors uploading to V1 servers, because it's no longer
    // interesting relative to uploads to the new "Ingest" servers.
    // On 2017-03-13 I re-enabled after "connection refused" errors
    // On 2017-03-25 I re-disabled after the errors were again too noisy
    // On 2017-03-26 I re-enabled it but only for production, not dev
    if !isDev {
        endTransaction(transaction, domain, errString)
    } else {
        if (errString != "") {
            fmt.Printf("*** Error uploading to %s: %v\n", domain, errString)
        }
        endTransaction(transaction, domain, "")
    }

    return errString == "", response

}

// Generate a hasn of the data structure elements that came from the device
func HashSafecastData(sd SafecastData) string {

    // Remove everything that is not generated by the device
    sd.Service = nil
    sd.Gateway = nil

    // Marshall into JSON
    scJSON, _ := json.Marshal(sd)

    // Compute the hash
    h := md5.New()
    io.WriteString(h, string(scJSON))
    hexHash := fmt.Sprintf("%x", h.Sum(nil))

    // Return the CRC
    return hexHash

}

// Upload a Safecast data structure to the Safecast service, either serially or massively in parallel
func SafecastUpload(sd SafecastData) bool {

    // Upload to all URLs
    for _, url := range SafecastUploadURLs {
        go doUploadToSafecast(sd, url)
    }

    return true
}

// Upload a Safecast data structure to the Safecast service
func doUploadToSafecast(sd SafecastData, url string) bool {

    var CapturedAt string = ""
    if sd.CapturedAt != nil {
        CapturedAt = *sd.CapturedAt
    }
    transaction := beginTransaction("V2", "captured", CapturedAt)

    // Marshal it to json text
    scJSON, _ := json.Marshal(sd)
    if (false) {
        fmt.Printf("%s\n", scJSON)
    }

    req, err := http.NewRequest("POST", url, bytes.NewBuffer(scJSON))
    req.Header.Set("User-Agent", "TTSERVE")
    req.Header.Set("Content-Type", "application/json")
    httpclient := &http.Client{
        Timeout: time.Second * 15,
    }
    resp, err := httpclient.Do(req)

    errString := ""
    if (err == nil) {
        resp.Body.Close()
    } else {
        // Eliminate the URL from the string because exposing the API key is not secure.
        // Empirically we've seen that the actual error message is after the rightmost colon
        errString = fmt.Sprintf("%s", err)
        s := strings.Split(errString, ":")
        errString = s[len(s)-1]
    }

    endTransaction(transaction, url, errString)

    return errString == ""
}
