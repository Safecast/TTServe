// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Slack channel handling, both inbound and outbound
package main

import (
    "os"
    "fmt"
    "html"
    "time"
    "bytes"
    "strings"
    "net/url"
    "net/http"
    "io/ioutil"
    "encoding/json"
)

// Slack webhook
func inboundWebSlackHandler(rw http.ResponseWriter, req *http.Request) {
    stats.Count.HTTP++
    stats.Count.HTTPSlack++

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
        fmt.Printf("Slack token not present\n")
        return
    }
    token := t[0]

    u, present := urlParams["user_name"]
    if !present {
        fmt.Printf("Slack user_name not present\n")
        return
    }
    user := u[0]

    m, present := urlParams["text"]
    if !present {
        fmt.Printf("Slack message not present\n")
        return
    }
    message := m[0]

    // If the message is surrounded in backticks to eliminate formatting, remove them
    message = strings.Replace(message, "`", "", -1)

    // Process the command arguments
    args := strings.Split(message, " ")
    argsLC := strings.Split(strings.ToLower(message), " ")

    firstArgLC := ""
    if len(args) > 1 {
        firstArgLC = argsLC[1]
    }

    secondArgLC := ""
    if len(args) > 2 {
        secondArgLC = argsLC[2]
    }

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

    // Remember when this command was received, and make sure
    // that all other instances also know that precise time
    SlackCommandTime = time.Now()
    ControlFileTime(TTServerSlackCommandControlFile, user)

    // Figure out who is sending this to us
    SlackCommandSource = SLACK_OPS_NONE
    str := strings.Split(ServiceConfig.SlackInboundTokens, ",")
    for source, tok := range str {
        if tok == token {
            SlackCommandSource = source
        }
    }
    if SlackCommandSource == SLACK_OPS_NONE {
        fmt.Printf("*** Slack command from uknown token: %s\n", token)
        return
    }

    // Process common argument
    fMobile := firstArgLC == "mobile" || secondArgLC == "mobile"
    fDetails := fMobile || firstArgLC == "detail" || firstArgLC == "details" || secondArgLC == "detail" || secondArgLC == "-d" || secondArgLC == "details"

    // Process queries
    switch argsLC[0] {

    case "online":
        go sendSafecastDeviceSummaryToSlack("", false, fMobile, fDetails)

    case "offline":
        go sendSafecastDeviceSummaryToSlack("", true, fMobile, fDetails)

    case "did":
        fallthrough
    case "deviceid":
        if len(args) != 2 {
            sendToSafecastOps("Command format: deviceid three-simple-words", SLACK_MSG_REPLY)
        } else {
            found, did := WordsToNumber(args[1])
            if !found {
                sendToSafecastOps("Device ID not found.", SLACK_MSG_REPLY)
            } else {
                sendToSafecastOps(fmt.Sprintf("%s is %d", args[1], did), SLACK_MSG_REPLY)
            }
        }

    case "device":
        fallthrough
    case "devices":
        fallthrough
    case "ttnode":
        go sendSafecastDeviceSummaryToSlack("== Offline ==", true, fMobile, fDetails)
        time.Sleep(2 * time.Second)
        go sendSafecastDeviceSummaryToSlack("== Online ==", false, fMobile, fDetails)

    case "gateway":
        fallthrough
    case "gateways":
        fallthrough
    case "ttgate":
        sendSafecastGatewaySummaryToSlack("", fMobile, fDetails)

    case "server":
        fallthrough
    case "servers":
        fallthrough
    case "ttserve":
        sendSafecastServerSummaryToSlack("", fMobile, fDetails)

    case "summary":
        fallthrough
    case "status":
        go sendSafecastServerSummaryToSlack("== Servers ==", fMobile, fDetails)
        time.Sleep(1 * time.Second)
        go sendSafecastGatewaySummaryToSlack("== Gateways ==", fMobile, fDetails)
        time.Sleep(1 * time.Second)
        go sendSafecastDeviceSummaryToSlack("== Devices Offline ==", true, fMobile, fDetails)
        time.Sleep(1 * time.Second)
        go sendSafecastDeviceSummaryToSlack("== Devices Online ==", false, fMobile, fDetails)

    case "select":
        if len(args) < 2 {
            sendToSafecastOps("Command format: SELECT <query>", SLACK_MSG_REPLY)
        } else {
            // Unescape the string, which substitutes &gt for >
            rawQuery := html.UnescapeString(messageAfterFirstWord)
            fmt.Printf("\n%s *** Influx query: \"%s\"\n", logTime(), rawQuery)
            // Perform the query
            success, result, numrows := InfluxQuery(user, rawQuery)
            if !success {
                sendToSafecastOps(fmt.Sprintf("Query error: %s: %s", result, "SELECT " + rawQuery), SLACK_MSG_REPLY)
            } else {
                sendToSafecastOps(fmt.Sprintf("%d rows of data are <%s|here>, @%s.", numrows, result, user), SLACK_MSG_REPLY)
            }
        }

    case "sn":
        if len(args) != 2 {
            sendToSafecastOps("Command format: sn <deviceID>", SLACK_MSG_REPLY)
        } else {
            found, deviceID := WordsToNumber(args[1])
            if !found {
                sendToSafecastOps(fmt.Sprintf("Invalid device ID."), SLACK_MSG_REPLY)
            } else {
                sn, reason := SafecastDeviceIDToSN(deviceID)
                if sn == 0 {
                    sendToSafecastOps(fmt.Sprintf("Unable to find S/N: %s", reason), SLACK_MSG_REPLY)
                } else {
                    sendToSafecastOps(fmt.Sprintf("S/N: %d", sn), SLACK_MSG_REPLY)
                }
            }
        }

    case "deveui":
        generateTTNCTLDeviceRegistrationScript()

    case "pending":
        fallthrough
    case "outbound":
        sendTelecastOutboundSummaryToSlack()

    case "cancel":
        if len(args) != 2 {
            sendToSafecastOps("Command format: cancel <deviceID>", SLACK_MSG_REPLY)
        } else {
            found, deviceID := WordsToNumber(args[1])
            if !found {
                sendToSafecastOps("Invalid Device ID.", SLACK_MSG_REPLY)
            } else {
                if cancelCommand(deviceID) {
                    sendToSafecastOps("Cancelled.", SLACK_MSG_REPLY)
                } else {
                    sendToSafecastOps("Not found.", SLACK_MSG_REPLY)
                }
            }
        }

    case "reboot-all":
    case "restart-all":
        sendToSafecastOps(fmt.Sprintf("Restarting all service instances."), SLACK_MSG_REPLY)
        time.Sleep(2 * time.Second)
        ServerLog(fmt.Sprintf("*** RESTARTING because of Slack 'restart-all' command\n"))
        ControlFileTime(TTServerRestartAllControlFile, user)
        sendToSafecastOps(fmt.Sprintf("** %s restarting **", TTServeInstanceID), SLACK_MSG_UNSOLICITED_OPS)
        time.Sleep(3 * time.Second)
        os.Exit(0)

    case "reboot":
    case "restart":
        sendToSafecastOps(fmt.Sprintf("Restarting non-monitor service instances."), SLACK_MSG_REPLY)
        ControlFileTime(TTServerRestartAllControlFile, user)

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
            found, deviceID := WordsToNumber(args[1])
            if !found {
                sendToSafecastOps("Invalid Device ID.", SLACK_MSG_REPLY)
            } else {
                sendToSafecastOps(fmt.Sprintf("Sending to %d: %s", deviceID, messageAfterSecondWord), SLACK_MSG_REPLY)
                sendCommand(user, deviceID, messageAfterSecondWord)
            }
        }

    case "clear-logs":
        if len(args) == 2 {
            found, deviceID := WordsToNumber(args[1])
            if !found {
                sendToSafecastOps("Invalid Device ID.", SLACK_MSG_REPLY)
            } else {
                sendToSafecastOps(SafecastDeleteLogs(deviceID), SLACK_MSG_REPLY)
            }
        } else {
            sendToSafecastOps("Command format: clear <deviceID>", SLACK_MSG_REPLY)
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

// Send a text string to the Safecast #ops channel.  Note that this MUST BE FAST
// because there are assumptions that this will return quickly because it's called
// within an HTTP request handler that must return so as to flush the response buffer
// back to the callers.
func sendToSafecastOps(msg string, destination int) {
    if destination == SLACK_MSG_UNSOLICITED_ALL {
        str := strings.Split(ServiceConfig.SlackOutboundUrls, ",")
        for _, url := range str {
            go sendToOpsViaSlack(msg, url)
        }
    } else if destination == SLACK_MSG_UNSOLICITED_OPS {
        str := strings.Split(ServiceConfig.SlackOutboundUrls, ",")
        go sendToOpsViaSlack(msg, str[SLACK_OPS_SAFECAST])
    } else {
        if SlackCommandSource != SLACK_OPS_NONE {
            str := strings.Split(ServiceConfig.SlackOutboundUrls, ",")
            go sendToOpsViaSlack(msg, str[SlackCommandSource])
        }
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

    // Wait for it to complete, because we seem to lose it on os.Exit()
    time.Sleep(5 * time.Second)

}
