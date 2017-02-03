// Slack #ops channel handling, both inbound and outbound
package main

import (
    "os"
    "fmt"
    "time"
    "bytes"
    "strings"
    "strconv"
    "net/url"
    "net/http"
    "io/ioutil"
    "encoding/json"
)

// Slack webhook
func inboundWebSlackHandler(rw http.ResponseWriter, req *http.Request) {

    // Unpack the request
    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        fmt.Printf("Slack webhook: error reading body:", err)
        return
    }
    urlParams, err := url.ParseQuery(string(body))
    if err != nil {
        fmt.Printf("Slack webhook: error parsing body:", err)
        return
    }

    // Extract useful information
    u, present := urlParams["user_name"]
    if !present {
        fmt.Printf("Slack user_name not present\n");
        return
    }
    user := u[0]

    m, present := urlParams["text"]
    if !present {
        fmt.Printf("Slack message not present\n");
        return
    }
    message := m[0]

    args := strings.Split(message, " ")
    argsLC := strings.Split(strings.ToLower(message), " ")

    messageAfterFirstWord := ""
    if len(args) > 1 {
        messageAfterFirstWord = strings.Join(args[1:], " ")
    }
    messageAfterSecondWord := ""
    if len(args) > 2 {
        messageAfterSecondWord = strings.Join(args[2:], " ")
    }

    // If this is a recursive echoing of our own post, bail.
    if user == "slackbot" {
        return
    }


    // Process queries
    switch argsLC[0] {

    case "status":
        if messageAfterFirstWord == "" {
            sendSafecastDeviceSummaryToSlack()
        }

    case "pending":
        fallthrough
    case "outbound":
        sendTelecastOutboundSummaryToSlack()

    case "cancel":
        if len(args) != 2 {
            sendToSafecastOps("Command format: cancel <deviceID>")
        } else {
            i64, _ := strconv.ParseUint(args[1], 10, 32)
            deviceID := uint32(i64)
            if (cancelCommand(deviceID)) {
                sendToSafecastOps("Cancelled.")
            } else {
                sendToSafecastOps("Not found.")
            }
        }

    case "restart":
        sendToSafecastOps(fmt.Sprintf("** %s restarting **", TTServerIP))
        fmt.Printf("\n***\n***\n*** RESTARTING because of Slack 'restart' command\n***\n***\n\n")
        os.Exit(0)

    case "restart-all":
        sendToSafecastOps(fmt.Sprintf("Restarting all instances..."))
		ControlFileTime(TTServerRestartAllControlFile, user)

    case "health":
        sendToSafecastOps(fmt.Sprintf("Checking health of all instances..."))
		ControlFileTime(TTServerHealthControlFile, user)

    case "send":
        if len(args) < 3 {
            sendToSafecastOps("Command format: send <deviceID> <message>")
        } else {
            i64, err := strconv.ParseUint(args[1], 10, 32)
            deviceID := uint32(i64)
            if err != nil {
                sendToSafecastOps("Command format: send <deviceID> <message>")
            } else {
                sendToSafecastOps(fmt.Sprintf("Sending to %d: %s", deviceID, messageAfterSecondWord))
                sendCommand(user, deviceID, messageAfterSecondWord)
            }
        }

    case "hello":
        if len(args) == 1 {
            sendToSafecastOps(fmt.Sprintf("Hello back, %s.", user))
        } else {
            sendToSafecastOps(fmt.Sprintf("Back at you: %s", messageAfterFirstWord))
        }

    default:
        // Default is to do nothing
    }

}

// Send a text string to the Safecast #ops channel
func sendToSafecastOps(msg string) {
    sendToOpsViaSlack(msg, "https://hooks.slack.com/services/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")
}

// Send a text string to the Safecast #api channel
func sendToSafecastApi(msg string) {
    sendToOpsViaSlack(msg, "https://hooks.slack.com/services/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")
}

// Send a text string to the TTN  #ops channel
func sendToTTNOps(msg string) {
    // Do nothing for now
}

// Send a string as a slack post to the specified channel
func sendToOpsViaSlack(msg string, SlackOpsPostURL string) {

    type SlackData struct {
        Message string `json:"text"`
    }

    m := SlackData{}
    m.Message = msg

    mJSON, _ := json.Marshal(m)
    req, err := http.NewRequest("POST", SlackOpsPostURL, bytes.NewBuffer(mJSON))
    req.Header.Set("User-Agent", "TTSERVE")
    req.Header.Set("Content-Type", "application/json")

    httpclient := &http.Client{}
    resp, err := httpclient.Do(req)
    if err != nil {
        fmt.Printf("*** Error uploading %s to Slack  %s\n\n", msg, err)
    } else {
        resp.Body.Close()
    }

    // Wait for it to complete, because we seem to lose it on os.Exit();
    time.Sleep(5 * time.Second)

}
