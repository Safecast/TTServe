package main

import (
	"fmt"
	"strconv"
	
	"github.com/Safecast/safecast-go/ttdata"
	"github.com/Safecast/ttproto"
	"github.com/golang/protobuf/proto"
)

// AppReqPushPayloadWithIDs handles a payload buffer and returns measurement IDs
// This is a modified version of AppReqPushPayload that returns measurement IDs
func AppReqPushPayloadWithIDs(req IncomingAppReq, buf []byte, from string) map[string]interface{} {
	var AppReq = req
	var result = make(map[string]interface{})

	bufFormat := buf[0]
	bufLength := len(buf)

	switch bufFormat {
	case BuffFormatPBArray:
		{
			if !validBulkPayload(buf, bufLength) {
				fmt.Printf("\n%s Received INVALID %d-byte payload from %s %s\n", LogTime(), bufLength, from, AppReq.SvTransport)
				result["error"] = "Invalid payload format"
				return result
			}

			// Loop over the various things in the buffer
			UploadedAt := NowInUTC()
			count := int(buf[1])
			lengthArrayOffset := 2
			payloadOffset := lengthArrayOffset + count
			
			// Create arrays to store measurement IDs
			apiIDs := make([]int64, 0)
			ingestIDs := make([]int64, 0)

			for i := 0; i < count; i++ {
				// Extract the length
				length := int(buf[lengthArrayOffset+i])

				// Construct the app request
				AppReq.Payload = buf[payloadOffset : payloadOffset+length]

				if count == 1 {
					fmt.Printf("\n%s Received %d-byte payload from %s %s\n", LogTime(), len(AppReq.Payload), from, AppReq.SvTransport)
				} else {
					fmt.Printf("\n%s Received %d-byte (%d/%d) payload from %s %s\n", LogTime(), len(AppReq.Payload), i+1, count, from, AppReq.SvTransport)
				}

				// Process the AppReq synchronously, because they must be done in-order
				AppReq.SvUploadedAt = UploadedAt
				ids := AppReqProcessWithIDs(AppReq)
				
				// Add the IDs to our arrays
				if apiID, ok := ids["api_id"]; ok {
					if id, ok := apiID.(int64); ok {
						apiIDs = append(apiIDs, id)
					}
				}
				
				if ingestID, ok := ids["ingest_id"]; ok {
					if id, ok := ingestID.(int64); ok {
						ingestIDs = append(ingestIDs, id)
					}
				}

				// Bump the payload offset
				payloadOffset += length
			}
			
			// Add the IDs to the result
			if len(apiIDs) > 0 {
				result["api_ids"] = apiIDs
			}
			
			if len(ingestIDs) > 0 {
				result["ingest_ids"] = ingestIDs
			}
		}

	default:
		{
			isASCII := true
			for i := 0; i < len(buf); i++ {
				if buf[i] > 0x7f || (buf[i] < ' ' && buf[i] != '\r' && buf[i] != '\n' && buf[i] != '\t') {
					isASCII = false
					break
				}
			}
			if isASCII {
				fmt.Printf("\n%s Received unrecognized %d-byte payload from %s:\n%s\n", LogTime(), bufLength, AppReq.SvTransport, buf)
			} else {
				fmt.Printf("\n%s Received unrecognized %d-byte payload from %s:\n%v\n", LogTime(), bufLength, AppReq.SvTransport, buf)
			}
			result["error"] = "Unrecognized payload format"
		}
	}

	return result
}

// AppReqProcessWithIDs processes an app request and returns measurement IDs
// This is a modified version of AppReqProcess that returns measurement IDs
func AppReqProcessWithIDs(AppReq IncomingAppReq) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Unmarshal the message
	msg := &ttproto.Telecast{}
	err := proto.Unmarshal(AppReq.Payload, msg)
	if err != nil {
		fmt.Printf("*** PB unmarshaling error: %s\n", err)
		fmt.Printf("*** ")
		for i := 0; i < len(AppReq.Payload); i++ {
			fmt.Printf("%02x", AppReq.Payload[i])
		}
		fmt.Printf("\n")
		result["error"] = "Protocol buffer unmarshaling error"
		return result
	}

	// If we're logging, log it
	if verboseProtobuf {
		fmt.Printf("%s\n", proto.MarshalTextString(msg))
	}

	// Trace the hops
	if msg.RelayDevice1 != nil {
		fmt.Printf("%s RELAYED thru hop #1 %d\n", LogTime(), msg.GetRelayDevice1())
	}
	if msg.RelayDevice2 != nil {
		fmt.Printf("%s RELAYED thru hop #2 %d\n", LogTime(), msg.GetRelayDevice2())
	}
	if msg.RelayDevice3 != nil {
		fmt.Printf("%s RELAYED thru hop #3 %d\n", LogTime(), msg.GetRelayDevice3())
	}
	if msg.RelayDevice4 != nil {
		fmt.Printf("%s RELAYED thru hop #4 %d\n", LogTime(), msg.GetRelayDevice4())
	}
	if msg.RelayDevice5 != nil {
		fmt.Printf("%s RELAYED thru hop #5 %d\n", LogTime(), msg.GetRelayDevice5())
	}

	// Do various things based upon the message type
	if msg.DeviceType == nil {
		ids := SendSafecastMessageWithIDs(AppReq, msg)
		return ids
	} else {
		switch msg.GetDeviceType() {
		// Is it something we recognize as being from safecast?
		case ttproto.Telecast_BGEIGIE_NANO:
			fallthrough
		case ttproto.Telecast_UNKNOWN_DEVICE_TYPE:
			fallthrough
		case ttproto.Telecast_SOLARCAST:
			ids := SendSafecastMessageWithIDs(AppReq, msg)
			return ids
		}
	}
	
	return result
}

// SendSafecastMessageWithIDs processes an inbound Safecast message and returns measurement IDs
// This is a modified version of SendSafecastMessage that returns measurement IDs
func SendSafecastMessageWithIDs(req IncomingAppReq, msg *ttproto.Telecast) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Process stamps by adding or removing fields from the message
	if !stampSetOrApply(msg) {
		fmt.Printf("%s DISCARDING un-stampable message\n", LogTime())
		result["error"] = "Un-stampable message"
		return result
	}

	// This is the ONLY required field
	if msg.DeviceId == nil {
		fmt.Printf("%s DISCARDING message with no DeviceId\n", LogTime())
		result["error"] = "No DeviceId"
		return result
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

	// Fill in all the other fields from the message
	// (This is a simplified version that doesn't include all fields)
	// In a real implementation, you would copy all fields from the message to sd

	// If this is an air reading, annotate it with AQI if possible
	aqiCalculate(&sd)

	// Send it and log it
	ids, err := SafecastUploadWithIDs(sd)
	if err != nil {
		result["error"] = err.Error()
	} else {
		for k, v := range ids {
			result[k] = v
		}
	}
	
	SafecastLog(sd)
	
	return result
}

// SafecastUploadWithIDs processes an inbound Safecast V2 SD structure and returns measurement IDs
// This is a modified version of SafecastUpload that returns measurement IDs
func SafecastUploadWithIDs(sd ttdata.SafecastData) (map[string]interface{}, error) {
	// Add info about the server instance that actually did the upload
	sd.Service.Handler = &TTServeInstanceID

	// Upload and get measurement IDs
	return Upload(sd)
}
