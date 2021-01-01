// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Formats and uploads messages for Safecast
package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	olc "github.com/google/open-location-code/go"
	"github.com/safecast/ttdata"
	ttproto "github.com/safecast/ttproto/golang"
)

// Debugging
const v1UploadDebug bool = false
const v1UploadSolarcast bool = true
const v1UploadSolarcastDebug bool = true
const verboseTransactions bool = false

// Synchronous vs asynchronous V1 API requests
const v1UploadAsyncFakeResults bool = false
const v1UploadFakeResult string = "{\"id\":00000001}"

// For dealing with transaction timeouts
var httpTransactionsInProgress int
var httpTransactions int

const httpTransactionsRecorded = 500

var httpTransactionDurations [httpTransactionsRecorded]int
var httpTransactionTimes [httpTransactionsRecorded]time.Time
var httpTransactionErrorTime string
var httpTransactionErrorURL string
var httpTransactionErrorString string
var httpTransactionErrors int
var httpTransactionErrorFirst = true

// SendSafecastMessage processes an inbound Safecast message as an asynchronous goroutine
func SendSafecastMessage(req IncomingAppReq, msg ttproto.Telecast) {

	// Process stamps by adding or removing fields from the message
	if !stampSetOrApply(&msg) {
		fmt.Printf("%s DISCARDING un-stampable message\n", LogTime())
		return
	}

	// This is the ONLY required field
	if msg.DeviceId == nil {
		fmt.Printf("%s DISCARDING message with no DeviceId\n", LogTime())
		return
	}

	// Generate the fields common to all uploads to safecast
	sd := ttdata.SafecastData{}
	did := uint32(msg.GetDeviceId())
	sd.DeviceID = did

	// Add our new device ID field
	devicetype, _ := SafecastDeviceType(did)
	if devicetype == "" {
		devicetype = "safecast"
	}
	sd.DeviceUID = fmt.Sprintf("%s:%d", devicetype, did)
	sd.DeviceClass = devicetype

	// Generate a Serial Number
	sn, _ := sheetDeviceIDToSN(did)
	if sn != "" {
		u64, err2 := strconv.ParseUint(sn, 10, 32)
		if err2 == nil {
			sd.DeviceSN = fmt.Sprintf("#%d", u64)
		}
	}

	// CapturedAt
	if msg.CapturedAt != nil {
		sd.CapturedAt = msg.CapturedAt
	} else if msg.CapturedAtDate != nil && msg.CapturedAtTime != nil && msg.CapturedAtOffset != nil {
		when := GetWhenFromOffset(msg.GetCapturedAtDate(), msg.GetCapturedAtTime(), msg.GetCapturedAtOffset())
		sd.CapturedAt = &when
	}

	// Loc
	if msg.Latitude != nil || msg.Longitude != nil || msg.MotionBeganOffset != nil {
		var loc ttdata.Loc
		if msg.Latitude != nil && msg.Longitude != nil {
			// 11 digits is 3m accuracy
			Olc := olc.Encode(float64(msg.GetLatitude()), float64(msg.GetLongitude()), 11)
			loc.Olc = &Olc
		}
		if msg.Latitude != nil {
			lat := float64(msg.GetLatitude())
			loc.Lat = &lat
		}
		if msg.Longitude != nil {
			lon := float64(msg.GetLongitude())
			loc.Lon = &lon
		}
		if msg.Altitude != nil {
			alt := float64(msg.GetAltitude())
			loc.Alt = &alt
		}
		if msg.MotionBeganOffset != nil && msg.CapturedAtDate != nil && msg.CapturedAtTime != nil {
			when := GetWhenFromOffset(msg.GetCapturedAtDate(), msg.GetCapturedAtTime(), msg.GetMotionBeganOffset())
			loc.MotionBegan = &when
		}
		sd.Loc = &loc
	}

	// Dev
	var dev ttdata.Dev
	var dodev = false

	if msg.Test != nil {
		mode := msg.GetTest()
		dev.Test = &mode
		dodev = true
	}
	if msg.Motion != nil {
		mode := msg.GetMotion()
		dev.Motion = &mode
		dodev = true
	}
	if msg.EncTemp != nil {
		temp := float64(*msg.EncTemp)
		dev.Temp = &temp
		dodev = true
	}
	if msg.EncHumid != nil {
		humid := float64(*msg.EncHumid)
		dev.Humid = &humid
		dodev = true
	}
	if msg.EncPressure != nil {
		press := float64(*msg.EncPressure)
		dev.Press = &press
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
	if msg.StatsOvercurrentEvents != nil {
		dev.OvercurrentEvents = msg.StatsOvercurrentEvents
		dodev = true
	}
	if msg.StatsCommsAntFails != nil {
		dev.CommsAntFails = msg.StatsCommsAntFails
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
	if msg.StatsMotionEvents != nil {
		dev.MotionEvents = msg.StatsMotionEvents
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
	if msg.ErrorsPms2 != nil {
		dev.ErrorsPms2 = msg.ErrorsPms2
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
	if msg.ErrorsMtu != nil {
		dev.ErrorsMtu = msg.ErrorsMtu
		dodev = true
	}
	if msg.StatsSeqno != nil {
		dev.Seqno = msg.StatsSeqno
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
	if msg.ErrorsConnectGateway != nil {
		dev.ErrorsConnectGateway = msg.ErrorsConnectGateway
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
	var bat ttdata.Bat
	var dobat = false

	if msg.BatVoltage != nil {
		voltage := float64(*msg.BatVoltage)
		bat.Voltage = &voltage
		dobat = true
	}
	if msg.BatSoc != nil {
		charge := float64(*msg.BatSoc)
		bat.Charge = &charge
		dobat = true
	}
	if msg.BatCurrent != nil {
		current := float64(*msg.BatCurrent)
		bat.Current = &current
		dobat = true
	}

	if dobat {
		sd.Bat = &bat
	}

	// Env
	var env ttdata.Env
	var doenv = false

	if msg.EnvTemp != nil {
		temp := float64(*msg.EnvTemp)
		env.Temp = &temp
		doenv = true
	}
	if msg.EnvHumid != nil {
		humid := float64(*msg.EnvHumid)
		env.Humid = &humid
		doenv = true
	}
	if msg.EnvPressure != nil {
		press := float64(*msg.EnvPressure)
		env.Press = &press
		doenv = true
	}

	if doenv {
		sd.Env = &env
	}

	// Service
	var svc ttdata.Service
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
	var gate ttdata.Gateway
	var dogate = false

	if msg.WirelessSnr != nil {
		snr := float64(*msg.WirelessSnr)
		gate.SNR = &snr
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
	var pms ttdata.Pms
	var dopms = false

	if msg.PmsPm01_0 != nil {
		Pm01_0 := float64(msg.GetPmsPm01_0())
		pms.Pm01_0 = &Pm01_0
		dopms = true
	}
	if msg.PmsPm02_5 != nil {
		Pm02_5 := float64(msg.GetPmsPm02_5())
		pms.Pm02_5 = &Pm02_5
		dopms = true
	}
	if msg.PmsPm10_0 != nil {
		Pm10_0 := float64(msg.GetPmsPm10_0())
		pms.Pm10_0 = &Pm10_0
		dopms = true
	}

	if msg.PmsStd01_0 != nil {
		f64 := float64(*msg.PmsStd01_0)
		pms.Std01_0 = &f64
		dopms = true
	}
	if msg.PmsStd02_5 != nil {
		f64 := float64(*msg.PmsStd02_5)
		pms.Std02_5 = &f64
		dopms = true
	}
	if msg.PmsStd10_0 != nil {
		f64 := float64(*msg.PmsStd10_0)
		pms.Std10_0 = &f64
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

	// Pms2
	var pms2 ttdata.Pms2
	var dopms2 = false

	if msg.Pms2Pm01_0 != nil {
		Pm01_0 := float64(msg.GetPms2Pm01_0())
		pms2.Pm01_0 = &Pm01_0
		dopms2 = true
	}
	if msg.Pms2Pm02_5 != nil {
		Pm02_5 := float64(msg.GetPms2Pm02_5())
		pms2.Pm02_5 = &Pm02_5
		dopms2 = true
	}
	if msg.Pms2Pm10_0 != nil {
		Pm10_0 := float64(msg.GetPms2Pm10_0())
		pms2.Pm10_0 = &Pm10_0
		dopms2 = true
	}

	if msg.Pms2Std01_0 != nil {
		f64 := float64(*msg.Pms2Std01_0)
		pms2.Std01_0 = &f64
		dopms2 = true
	}
	if msg.Pms2Std02_5 != nil {
		f64 := float64(*msg.Pms2Std02_5)
		pms2.Std02_5 = &f64
		dopms2 = true
	}
	if msg.Pms2Std10_0 != nil {
		f64 := float64(*msg.Pms2Std10_0)
		pms2.Std10_0 = &f64
		dopms2 = true
	}

	if dopms2 {
		if msg.Pms2C00_30 != nil {
			pms2.Count00_30 = msg.Pms2C00_30
		}
		if msg.Pms2C00_50 != nil {
			pms2.Count00_50 = msg.Pms2C00_50
		}
		if msg.Pms2C01_00 != nil {
			pms2.Count01_00 = msg.Pms2C01_00
		}
		if msg.Pms2C02_50 != nil {
			pms2.Count02_50 = msg.Pms2C02_50
		}
		if msg.Pms2C05_00 != nil {
			pms2.Count05_00 = msg.Pms2C05_00
		}
		if msg.Pms2C10_00 != nil {
			pms2.Count10_00 = msg.Pms2C10_00
		}
		if msg.Pms2Csecs != nil {
			pms2.CountSecs = msg.Pms2Csecs
		}
	}

	if dopms2 {
		sd.Pms2 = &pms2
	}

	// Opc
	var opc ttdata.Opc
	var doopc = false

	if msg.OpcPm01_0 != nil {
		f64 := float64(*msg.OpcPm01_0)
		opc.Pm01_0 = &f64
		doopc = true
	}
	if msg.OpcPm02_5 != nil {
		f64 := float64(*msg.OpcPm02_5)
		opc.Pm02_5 = &f64
		doopc = true
	}
	if msg.OpcPm10_0 != nil {
		f64 := float64(*msg.OpcPm10_0)
		opc.Pm10_0 = &f64
		doopc = true
	}

	if msg.OpcStd01_0 != nil {
		f64 := float64(*msg.OpcStd01_0)
		opc.Std01_0 = &f64
		doopc = true
	}
	if msg.OpcStd02_5 != nil {
		f64 := float64(*msg.OpcStd02_5)
		opc.Std02_5 = &f64
		doopc = true
	}
	if msg.OpcStd10_0 != nil {
		f64 := float64(*msg.OpcStd10_0)
		opc.Std10_0 = &f64
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
	var lnd ttdata.Lnd
	var dolnd = false

	if msg.Lnd_7318U != nil {
		var cpm = float64(msg.GetLnd_7318U())
		lnd.U7318 = &cpm
		dolnd = true
	}
	if msg.Lnd_7318C != nil {
		var cpm = float64(msg.GetLnd_7318C())
		lnd.C7318 = &cpm
		dolnd = true
	}
	if msg.Lnd_7128Ec != nil {
		var cpm = float64(msg.GetLnd_7128Ec())
		lnd.EC7128 = &cpm
		dolnd = true
	}
	if msg.Lnd_712U != nil {
		var cpm = float64(msg.GetLnd_712U())
		lnd.U712 = &cpm
		dolnd = true
	}
	if msg.Lnd_78017W != nil {
		var cpm = float64(msg.GetLnd_78017W())
		lnd.W78017 = &cpm
		dolnd = true
	}

	if dolnd {
		sd.Lnd = &lnd
	}

	// Send it and log it
	SafecastUpload(sd)
	SafecastLog(sd)

}

// SafecastUpload processes an inbound Safecast V2 SD structure as an asynchronous goroutine
func SafecastUpload(sd ttdata.SafecastData) {

	// Add info about the server instance that actually did the upload
	sd.Service.Handler = &TTServeInstanceID

	// Upload
	Upload(sd)

}

// SafecastLog logs the event
func SafecastLog(sd ttdata.SafecastData) {

	// Add info about the server instance that actually did the upload
	sd.Service.Handler = &TTServeInstanceID

	// Log as accurately as we can with regard to what came in
	WriteToLogs(sd)

}

// Begin transaction and return the transaction ID
func beginTransaction(version string, message1 string, message2 string) int {
	httpTransactionsInProgress++
	httpTransactions++
	transaction := httpTransactions % httpTransactionsRecorded
	httpTransactionTimes[transaction] = time.Now()
	if verboseTransactions {
		fmt.Printf("%s >>> %s [%d] %s %s\n", LogTime(), version, transaction, message1, message2)
	}
	return transaction
}

// End transaction and issue warnings
func endTransaction(transaction int, url string, errstr string) {
	httpTransactionsInProgress--
	duration := int(time.Now().Sub(httpTransactionTimes[transaction]) / time.Second)
	httpTransactionDurations[transaction] = duration

	if errstr != "" {
		httpTransactionErrors = httpTransactionErrors + 1
		if httpTransactionErrorFirst {
			httpTransactionErrorTime = LogTime()
			httpTransactionErrorURL = url
			httpTransactionErrorString = errstr
			httpTransactionErrorFirst = false
		}
		if verboseTransactions {
			fmt.Printf("%s <<<    [%d] *** ERROR\n", LogTime(), transaction)
		}
		ServerLog(fmt.Sprintf("After %d seconds, error uploading to %s %s\n", duration, url, errstr))
	} else {
		if verboseTransactions {
			if duration < 5 {
				fmt.Printf("%s <<<    [%d]\n", LogTime(), transaction)
			} else {
				fmt.Printf("%s <<<    [%d] completed after %d seconds\n", LogTime(), transaction, duration)
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
		theCount++
	}
	theMean := theTotal / theCount

	// Output to console every time we are in a "slow mode"
	if theMin > 5 {
		fmt.Printf("%s Safecast Upload Statistics\n", LogTime())
		fmt.Printf("%s *** %d total uploads since restart\n", LogTime(), httpTransactions)
		if httpTransactionsInProgress > 0 {
			fmt.Printf("%s *** %d uploads still in progress\n", LogTime(), httpTransactionsInProgress)
		}
		fmt.Printf("%s *** Last %d: min=%ds, max=%ds, avg=%ds\n", LogTime(), theCount, theMin, theMax, theMean)

	}

	// If there's a problem, output to Slack once every 25 transactions
	if theMin > 5 && transaction == 0 {
		// If all of them have the same timeout value, the server must be down.
		s := ""
		if theMin == theMax && theMin == theMean {
			s = fmt.Sprintf("HTTP Upload: all of the most recent %d uploads failed. Please check the service.", theCount)
		} else {
			s = fmt.Sprintf("HTTP Upload: of the previous %d uploads, min=%ds, max=%ds, avg=%ds", theCount, theMin, theMax, theMean)
		}
		sendToSafecastOps(s, SlackMsgUnsolicitedOps)
	}

}

// Update message ages and notify
func sendSafecastCommsErrorsToSlack(PeriodMinutes uint32) {
	if httpTransactionErrors != 0 {
		sendToSafecastOps(fmt.Sprintf("** Warning **  In the %d mins after %s UTC there were errors uploading to %s:%s",
			PeriodMinutes, httpTransactionErrorTime, httpTransactionErrorURL, httpTransactionErrorString), SlackMsgUnsolicitedOps)
		httpTransactionErrors = 0
		httpTransactionErrorFirst = true
	}
}

// SafecastV1Upload uploads a Safecast data structure to the Safecast service
func SafecastV1Upload(body []byte, url string, isDev bool, unit string, value string) (fSuccess bool, result string) {

	if v1UploadAsyncFakeResults {
		go doSafecastV1Upload(body, url, isDev, unit, value)
		return true, v1UploadFakeResult
	}

	return doSafecastV1Upload(body, url, isDev, unit, value)

}

// Perform the body of the upload
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
	requestURI := fmt.Sprintf(SafecastV1UploadPattern, domain, query)
	if v1UploadDebug {
		fmt.Printf("****** '%s'\n%s\n", requestURI, string(body))
	}

	// Perform the transaction
	transaction := beginTransaction(v1str, unit, value)
	req, _ := http.NewRequest("POST", requestURI, bytes.NewBuffer(body))
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
	if err == nil {
		buf, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			if v1UploadDebug {
				fmt.Printf("*** Response:\n%s\n", string(buf))
			}
			// We'd like to return the response
			respstr := string(buf)
			if strings.Contains(respstr, "<head>") {
				fmt.Printf("*** %s response is HTML (%d bytes) rather than JSON ***\n", domain, len(respstr))
				filename := SafecastDirectory() + TTServerLogPath + "/" + domain + ".txt"
				fd, e := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
				if e == nil {
					fd.WriteString(respstr)
					fd.Close()
				}
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
		if errString != "" {
			fmt.Printf("*** Error uploading to %s: %v\n", domain, errString)
		}
		endTransaction(transaction, domain, "")
	}

	return errString == "", response

}

// HashSafecastData returns the MD5 hash of the data structure elements that came from the device
func HashSafecastData(sd ttdata.SafecastData) string {

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

// Do a single solarcast v1 upload
func doSolarcastV1Upload(sdV1Emit *SafecastDataV1ToEmit) {

	sdV1EmitJSON, _ := json.Marshal(sdV1Emit)

	if v1UploadSolarcastDebug {
		fmt.Printf("$$$ Uploading Solarcast to V1 service:\n%s\n", sdV1EmitJSON)
	}

	requestURI := "http://api.safecast.org/measurements.json?api_key=z3sHhgousVDDrCVXhzMT"
	req, _ := http.NewRequest("POST", requestURI, bytes.NewBuffer(sdV1EmitJSON))
	req.Header.Set("User-Agent", "TTSERVE")
	req.Header.Set("Content-Type", "application/json")
	httpclient := &http.Client{
		Timeout: time.Second * 15,
	}
	resp, err := httpclient.Do(req)
	if err == nil {
		buf, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			if v1UploadSolarcastDebug {
				fmt.Printf("$$$ V1 service response:\n%s\n", string(buf))
			}
			resp.Body.Close()
		}
	}

	// Don't overload the server
	time.Sleep(1 * time.Second)

}

// Process solarcast uploads to v1, for "realtime" support
func doSolarcastV1Uploads(sd ttdata.SafecastData) {
	sd1, sd2, sd9, err := SafecastReformatToV1(sd)
	if err != nil {
		return
	}
	if sd1 != nil {
		doSolarcastV1Upload(sd1)
	}
	if sd2 != nil {
		doSolarcastV1Upload(sd2)
	}
	if sd9 != nil {
		doSolarcastV1Upload(sd9)
	}
}

// Upload uploads a Safecast data structure to the Safecast service, either serially or massively in parallel
func Upload(sd ttdata.SafecastData) bool {

	// Upload to all URLs
	for _, url := range SafecastUploadURLs {
		go doUploadToSafecast(sd, url)
	}

	// Upload Safecast data to the v1 production server
	if v1UploadSolarcast {
		go doSolarcastV1Uploads(sd)
	}

	// Upload safecast data to those listening on MQTT
	go brokerPublish(sd)

	return true
}

// Upload a Safecast data structure to the Safecast service
func doUploadToSafecast(sd ttdata.SafecastData, url string) bool {

	var CapturedAt string
	if sd.CapturedAt != nil {
		CapturedAt = *sd.CapturedAt
	}
	transaction := beginTransaction("V2", "captured", CapturedAt)

	// Marshal it to json text
	scJSON, _ := json.Marshal(sd)
	if true {
		fmt.Printf("...ingested as...\n%s\n", scJSON)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(scJSON))
	req.Header.Set("User-Agent", "TTSERVE")
	req.Header.Set("Content-Type", "application/json")
	httpclient := &http.Client{
		Timeout: time.Second * 15,
	}
	resp, err := httpclient.Do(req)

	errString := ""
	if err == nil {
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
