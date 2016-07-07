// Slack #ops channel handling, both inbound & outbound
package main

import (
    "fmt"
    "bytes"
    "strings"
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
    case "hello":
        if len(args) == 1 {
            sendToSlack(fmt.Sprintf("Hello back, %s.", user))
        } else {
            sendToSlack(fmt.Sprintf("Back at you: %s", messageAfterFirstWord))
        }
    default:
        // Default is to do nothing
    }

}

// Send a string as a slack post
func sendToSlack(msg string) {

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
