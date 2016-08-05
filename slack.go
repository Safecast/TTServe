// Slack #ops channel handling, both inbound & outbound
package main

import (
    "fmt"
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
    user := urlParams["user_name"][0]
    message := urlParams["text"][0]
    args := strings.Split(message, " ")
    argsLC := strings.Split(strings.ToLower(message), " ")
    messageAfterFirstWord := strings.Join(args[1:], " ")
    messageAfterSecondWord := strings.Join(args[2:], " ")

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

	case "send":
		if len(args) < 3 {
				sendToSafecastOps("Command format: send <deviceID> <message>")
			} else {
				i64, err := strconv.ParseUint(args[1], 10, 32)
				deviceID := int32(i64)
				if err != nil {
					sendToSafecastOps("Command format: send <deviceID> <message>")
				} else {
					sendToSafecastOps(fmt.Sprintf("Sending to %d: %s", deviceID, messageAfterSecondWord))
					}
		}

	case "broadcast":
			if len(args) < 2 {
				sendToSafecastOps("Command format: broadcast <message>")
			} else {
				sendToSafecastOps(fmt.Sprintf("Broadcasting: %s", messageAfterFirstWord))
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

}
