// Handle inbound and outbound Telecast messages
package main

import (
    "fmt"
	"time"
    "strings"
    "github.com/golang/protobuf/proto"
    "github.com/rayozzie/teletype-proto/golang"
)

// Describes every device to which we've sent a message
type sentDevice struct {
    devEui string
}

// Statics
var sentDevices []sentDevice

// Process inbound telecast message
func ProcessTelecastMessage(msg *teletype.Telecast, devEui string) {

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
        fmt.Printf("/hello from %s\n", devEui)
        if argRest == "" {
            sendMessage(devEui, "@ttserve: Hello.")
        } else {
            sendMessage(devEui, "@ttserve: "+argRest)
        }

        // Handle an inbound upstream-only ping (blank message) by just ignoring it
    case "":
		if (devEui == "") {
	        fmt.Printf("%s Ping from device\n", time.Now().Format(logDateFormat))
		} else {
	        fmt.Printf("%s Ping from %s\n", time.Now().Format(logDateFormat), devEui)
		}
		
        // Anything else is broadcast to all OTHER known devices
    default:
        fmt.Printf("\n%s Broadcast from %s: 'message'\n\n", time.Now().Format(logDateFormat), devEui, message)
        broadcastMessage(message, devEui)
    }

}

// Send a message to a specific device
func sendMessage(devEui string, message string) {

	// Keep track of devices to whom we've sent messages
    addKnownDevice(devEui)

	// Marshal the text string into a telecast message
    deviceType := teletype.Telecast_TTSERVE
    tmsg := &teletype.Telecast{}
    tmsg.DeviceType = &deviceType
    tmsg.Message = proto.String(message)
    tdata, terr := proto.Marshal(tmsg)
    if terr != nil {
        fmt.Printf("t marshaling error: ", terr)
    }

	// Ask TTN to publish it, noting that there
	// is currently no way to broadcast to non-TTN
	// nodes - such as via TTGATE.
    ttnOutboundPublish(devEui, tdata)

}

// Keep track of known devices
func addKnownDevice(devEui string) {
    var e sentDevice
    for _, e = range sentDevices {
        if devEui == e.devEui {
            return
        }
    }
    e.devEui = devEui
    sentDevices = append(sentDevices, e)
}

// Broadcast a message to all known devices except the one specified by 'skip'
func broadcastMessage(message string, skipDevEui string) {
    if skipDevEui == "" {
        fmt.Printf("Broadcast '%s'\n", message)
    } else {
        fmt.Printf("Skipping %s, broadcast '%s'\n", skipDevEui, message)
    }
    for _, e := range sentDevices {
        if e.devEui != skipDevEui {
            sendMessage(e.devEui, message)
        }
    }
}
