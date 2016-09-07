// Handle inbound and outbound Telecast messages
package main

import (
    "fmt"
	"time"
    "strings"
	"strconv"
    "github.com/golang/protobuf/proto"
    "github.com/rayozzie/teletype-proto/golang"
)

// Describes every device to which we've sent a message
type knownDevice struct {
    devEui string
	deviceID uint32
	messageToDevice []byte
}

// Statics
var knownDevices []knownDevice

// Get a Telecast Device ID number for this message
func TelecastDeviceID (msg *teletype.Telecast) uint32 {
    if msg.DeviceIDNumber != nil {
        return msg.GetDeviceIDNumber()
    } else if msg.DeviceIDString != nil {
        i64, err := strconv.ParseInt(msg.GetDeviceIDString(), 10, 64)
        if err == nil {
            return uint32(i64)
        }
    }
	return 0
}	

// Process inbound telecast message
func ProcessTelecastMessage(msg *teletype.Telecast, devEui string) {

	// Keep track of devices from whom we've received message
	deviceID := TelecastDeviceID(msg)
    addKnownDevice(devEui, deviceID)

    // Unpack the message arguments
    message := msg.GetMessage()
    args := strings.Split(message, " ")
    arg0 := args[0]
    arg0LC := strings.ToLower(args[0])
    argRest := strings.Trim(strings.TrimPrefix(message, arg0), " ")

    // Handle hard-wired commands
    switch arg0LC {

        // Hard-wired commands
    case "/echo":
        fallthrough
    case "/hello":
        fallthrough
    case "/hi":
        fmt.Printf("%s Telecast \"Hello\" message\n", time.Now().Format(logDateFormat))
        if argRest == "" {
            sendMessage(deviceID, "@ttserve: Hello.")
        } else {
            sendMessage(deviceID, "@ttserve: "+argRest)
        }

        // Handle an inbound upstream-only ping (blank message) by just ignoring it
    case "":
		fmt.Printf("%s Telecast \"Ping\" message\n", time.Now().Format(logDateFormat))
		
        // Anything else is broadcast to all OTHER known devices
    default:
        fmt.Printf("\n%s \"Broadcast\" message: '%s'\n\n", time.Now().Format(logDateFormat), message)
        broadcastMessage(message, deviceID)
    }

}

// Send a message to a specific device
func sendMessage(deviceID uint32, message string) {

	// Marshal the text string into a telecast message
    deviceType := teletype.Telecast_TTSERVE
    tmsg := &teletype.Telecast{}
	tmsg.DeviceIDNumber = &deviceID;
    tmsg.DeviceType = &deviceType
    tmsg.Message = proto.String(message)
    tdata, terr := proto.Marshal(tmsg)
    if terr != nil {
        fmt.Printf("t marshaling error: ", terr)
    }

	// Ask TTN to publish it, else leave it waiting
	// as an outbound message for the next time
	// the node polls.
    for i := 0; i < len(knownDevices); i++ {
		if (knownDevices[i].deviceID == deviceID) {
			if (knownDevices[i].devEui != "") {
				ttnOutboundPublish(knownDevices[i].devEui, tdata)
			} else {
				knownDevices[i].messageToDevice = tdata
				fmt.Printf("Enqueued %d-byte message for device %d\n", len(tdata), deviceID)
			}
		break;
		}
	}

}

// See if there is an outbound payload waiting for this device.
// If so, fetch it, clear it out, and return it.
func TelecastOutboundPayload(msg *teletype.Telecast) (isAvailable bool, payload []byte) {

	deviceID := TelecastDeviceID(msg);
    for i := 0; i < len(knownDevices); i++ {
		if (knownDevices[i].deviceID == deviceID) {
			if (knownDevices[i].messageToDevice != nil) {
				messageToDevice := knownDevices[i].messageToDevice
				knownDevices[i].messageToDevice = nil
				fmt.Printf("Dequeued %d-byte message for device %d\n", len(messageToDevice), deviceID)
				return true, messageToDevice
			}
		break;
		}
	}

	return false, nil

}	

// Keep track of known devices
func addKnownDevice(devEui string, deviceID uint32) {
    var e knownDevice
    for _, e = range knownDevices {
        if deviceID == e.deviceID {
            return
        }
    }
    e.devEui = devEui
	e.deviceID = deviceID;
	e.messageToDevice = nil
    knownDevices = append(knownDevices, e)
}

// Broadcast a message to all known devices except the one specified by 'skip'
func broadcastMessage(message string, skipDeviceID uint32) {
    if skipDeviceID == 0 {
        fmt.Printf("Broadcast '%s'\n", message)
    } else {
        fmt.Printf("Skipping %d, broadcast '%s'\n", skipDeviceID, message)
    }
    for _, e := range knownDevices {
		if (skipDeviceID == 0) {
			sendMessage(e.deviceID, message)
		} else {
			if e.deviceID != skipDeviceID {
	            sendMessage(e.deviceID, message)
			}
        }
    }
}
