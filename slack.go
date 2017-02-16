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
    t, present := urlParams["token"]
    if !present {
        fmt.Printf("Slack token not present\n");
        return
    }
    token := t[0]

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

	// Figure out who is sending this to us
	SlackCommandSource = SLACK_OPS_NONE
    for source, tok := range SlackInboundTokens {
		if tok == token {
			SlackCommandSource = source
		}
	}
	if SlackCommandSource == SLACK_OPS_NONE {
		fmt.Printf("*** Slack command from uknown token: %s\n", token)
		return
	}
	
    // Process queries
    switch argsLC[0] {

    case "status":
        if messageAfterFirstWord == "" {
            sendSafecastDeviceSummaryToSlack(false, false)
        }

    case "details":
        if messageAfterFirstWord == "" {
            sendSafecastDeviceSummaryToSlack(false, true)
        }

    case "mobile":
        if messageAfterFirstWord == "" {
            sendSafecastDeviceSummaryToSlack(true, true)
        }

    case "pending":
        fallthrough
    case "outbound":
        sendTelecastOutboundSummaryToSlack()

    case "cancel":
        if len(args) != 2 {
            sendToSafecastOps("Command format: cancel <deviceID>", SLACK_MSG_REPLY)
        } else {
            i64, _ := strconv.ParseUint(args[1], 10, 32)
            deviceID := uint32(i64)
            if (cancelCommand(deviceID)) {
                sendToSafecastOps("Cancelled.", SLACK_MSG_REPLY)
            } else {
                sendToSafecastOps("Not found.", SLACK_MSG_REPLY)
            }
        }

    case "restart":
		ControlFileTime(TTServerRestartAllControlFile, user)
        sendToSafecastOps(fmt.Sprintf("** %s restarting **", ThisServerAddressIPv4), SLACK_MSG_REPLY)
        fmt.Printf("\n***\n***\n*** RESTARTING because of Slack 'restart' command\n***\n***\n\n")
        os.Exit(0)

    case "health":
        sendToSafecastOps(fmt.Sprintf("Checking health of all instances..."), SLACK_MSG_REPLY)
		ControlFileTime(TTServerHealthControlFile, user)

    case "send":
        if len(args) == 1 {
            sendToSafecastOps("Command format: send <deviceID> <message>", SLACK_MSG_REPLY)
		} else if len(args) == 2 {
			switch argsLC[1] {
			case "hello":
	            sendHelloToNewDevices()
			default:
	            sendToSafecastOps("Unrecognized subcommand of 'send'", SLACK_MSG_REPLY)
			}
        } else {
            i64, err := strconv.ParseUint(args[1], 10, 32)
            deviceID := uint32(i64)
            if err != nil {
                sendToSafecastOps("Command format: send <deviceID> <message>", SLACK_MSG_REPLY)
            } else {
                sendToSafecastOps(fmt.Sprintf("Sending to %d: %s", deviceID, messageAfterSecondWord), SLACK_MSG_REPLY)
                sendCommand(user, deviceID, messageAfterSecondWord)
            }
        }

    case "hello":
        if len(args) == 1 {
            sendToSafecastOps(fmt.Sprintf("Hello back, %s.", user), SLACK_MSG_REPLY)
        } else {
            sendToSafecastOps(fmt.Sprintf("Back at you: %s", messageAfterFirstWord), SLACK_MSG_REPLY)
        }

    default:
        // Default is to do nothing
    }

}

// Send a text string to the Safecast #ops channel
func sendToSafecastOps(msg string, isUnsolicited bool) {
	if (isUnsolicited) {
	    for _, url := range SlackOutboundURLs {
		    sendToOpsViaSlack(msg, url)
		}
	} else {
		sendToOpsViaSlack(msg, SlackOutboundURLs[SlackCommandSource])
	}
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
