// Handle inbound and outbound Telecast messages
package main

import (
    "os"
    "fmt"
    "time"
    "strings"
    "strconv"
    "io/ioutil"
    "encoding/json"
    "github.com/golang/protobuf/proto"
    "github.com/rayozzie/teletype-proto/golang"
)

// Safecast Command as saved in text file
type safecastCommand struct {
    Command               string `json:"command,omitempty"`
    IssuedAt              string `json:"issued_at,omitempty"`
    IssuedBy              string `json:"issued_by,omitempty"`
}

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

// Get a summary of devices that are older than this many minutes ago
func sendTelecastOutboundSummaryToSlack() {

    first := true
    s := "Nothing pending for transmission."

    // Open the directory
    files, err := ioutil.ReadDir(SafecastDirectory() + TTServerCommandPath)
    if err == nil {

        // Iterate over each of the pending commands
        for _, file := range files {

            // Extract device ID from filename
            deviceStr := strings.Split(file.Name(), ".")[0]
            i64, _ := strconv.ParseUint(deviceStr, 10, 32)
            deviceID := uint32(i64)

            // Get the command info
            isValid, cmd := getCommand(deviceID)
            if (isValid) {

                if (first) {
                    first = false
                    s = ""
                } else {
                    s = s + "\n"
                }

                // Extract the time
                IssuedAt, _ := time.Parse("2006-01-02T15:04:05Z", cmd.IssuedAt)
                IssuedAtStr := IssuedAt.Format("2006-01-02 15:04 UTC")

                s = s + fmt.Sprintf("%d: %s (%s %s)", deviceID, cmd.Command, cmd.IssuedBy, IssuedAtStr)

            }

        }

    }

    // Send it to Slack
    sendToSafecastOps(s)

}

// Process inbound telecast message
func ProcessTelecastMessage(msg *teletype.Telecast, devEui string) {

    // Keep track of devices from whom we've received message
    deviceID := TelecastDeviceID(msg)

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
            sendCommand("", deviceID, "@server: Hello.")
        } else {
            sendCommand("", deviceID, "@server: "+argRest)
        }

        // Handle an inbound upstream-only ping (blank message) by just ignoring it
    case "":
        fmt.Printf("%s Telecast \"Ping\" message\n", time.Now().Format(logDateFormat))


    }

}

// Construct the path of a command file
func SafecastCommandFilename(DeviceID uint32) string {
    directory := SafecastDirectory()
    file := directory + TTServerCommandPath + "/" + fmt.Sprintf("%d", DeviceID) + ".json"
    return file
}

// Send a message to a specific device
func sendCommand(sender string, deviceID uint32, message string) {

    // Generate a command
    cmd := &safecastCommand{}
    cmd.IssuedBy = sender
    cmd.IssuedAt = time.Now().UTC().Format("2006-01-02T15:04:05Z")
    cmd.Command = message
    cmdJSON, _ := json.Marshal(cmd)

    // Write it to a file, overwriting if it already exists
    file := SafecastCommandFilename(deviceID)
    fd, err := os.OpenFile(file, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0666)
    if (err != nil) {
        fmt.Printf("SendCommand: error creating file %s: %s\n", file, err);
        return;
    }
    fd.WriteString(string(cmdJSON));

    // Done
    fd.Close();

}

// Cancel a command destined for a specific device
func cancelCommand(deviceID uint32) (isCancelled bool) {

    file := SafecastCommandFilename(deviceID)
    err := os.Remove(file)
    return err == nil

}

// Get command text
func getCommand(deviceID uint32) (isValid bool, command safecastCommand) {
    cmd := safecastCommand{}

    // Read the file and delete it
    file, err := ioutil.ReadFile(SafecastCommandFilename(deviceID))
    if err != nil {
        return false, cmd
    }

    // Read it as JSON
    err = json.Unmarshal(file, &cmd)
    if err != nil {
        fmt.Printf("getCommand unmarshaling error: ", err)
        return false, cmd
    }

    // Got it
    return true, cmd

}

// See if there is an outbound payload waiting for this device.
// If so, fetch it, clear it out, and return it.
func TelecastOutboundPayload(deviceID uint32) (isAvailable bool, payload []byte) {

    // Read the file and delete it
    isValid, cmd := getCommand(deviceID)
    if !isValid {
        return false, nil
    }

    // Marshal the command into a telecast message
    deviceType := teletype.Telecast_TTSERVE
    tmsg := &teletype.Telecast{}
    tmsg.DeviceIDNumber = &deviceID;
    tmsg.DeviceType = &deviceType
    tmsg.Message = proto.String(cmd.Command)
    tdata, terr := proto.Marshal(tmsg)
    if terr != nil {
        fmt.Printf("send msg marshaling error: ", terr)
        return false, nil
    }

    // Delete the file
    cancelCommand(deviceID);

    // Done
    return true, tdata

}
