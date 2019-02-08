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
    "strconv"
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
    SlackCommandSource = SlackOpsNone
    str := strings.Split(ServiceConfig.SlackInboundTokens, ",")
    for source, tok := range str {
        if tok == token {
            SlackCommandSource = source
        }
    }
    if SlackCommandSource == SlackOpsNone {
        fmt.Printf("*** Slack command from uknown token: %s\n", token)
        return
    }

    // Process common arguments
    devicelist := ""
    fDetails := false
    if firstArgLC == "detail" || firstArgLC == "details" {
        fDetails = true
        if secondArgLC != "" {
            devicelist = args[2]
        }
    } else if secondArgLC == "detail" || secondArgLC == "details" {
        fDetails = true
        devicelist = args[1]
    } else {
        if firstArgLC != "" {
            devicelist = args[1]
        }
    }


    // Process queries
    switch argsLC[0] {

    case "help":
        help := ""
        help += "Named Device Lists:\n"
        help += "     device <subcommand> <devlistname> <args>\n"
        help += "Get bulk or select device status:\n"
        help += "     status [devlistname] [details]\n"
        help += "Show bulk device status:\n"
        help += "     online [details]\n"
        help += "     offline [details]\n"
        help += "Show gateway and server status:\n"
        help += "     gateway [details]\n"
        help += "     server [details]\n"
        help += "Send/cancel/display device-targeted messages:\n"
        help += "     send <device> <message>\n"
        help += "     cancel <device>\n"
        help += "     pending\n"
        help += "Reset device logs used for 'chk':\n"
        help += "     clear-logs-and-make-ray-sad <device>\n"
        help += "Named report and time marker management\n"
        help += "     mark <subcommand> <markername> <args>\n"
        help += "     report <args> (type 'report' for more info)\n"
        help += "     check (same as 'report', but performs a value checkup)\n"
        go sendToSafecastOps(help, SlackMsgReply)

    case "check":
        fallthrough
    case "checkall":
        fallthrough
    case "device":
        fallthrough
    case "devices":
        fallthrough
    case "mark":
        fallthrough
    case "marks":
        fallthrough
    case "report":
        fallthrough
    case "reportall":
        go sendCommandToSlack(user, message)

    case "online":
        go sendSafecastDeviceSummaryToSlack(user, "", devicelist, false, fDetails)

    case "offline":
        go sendSafecastDeviceSummaryToSlack(user, "", devicelist, true, fDetails)

    case "did":
        fallthrough
    case "deviceid":
        if len(args) != 2 {
            sendToSafecastOps("Command format: deviceid <number> or <three-simple-words>", SlackMsgReply)
        } else {
            if !strings.Contains(args[1], "-") {
                i64, err := strconv.ParseUint(args[1], 10, 32)
                if err != nil {
                    sendToSafecastOps(fmt.Sprintf("%s", err), SlackMsgReply)
                } else {
                    sendToSafecastOps(fmt.Sprintf("%d is %s", i64, WordsFromNumber(uint32(i64))), SlackMsgReply)
                }
            } else {
                found, did := WordsToNumber(args[1])
                if !found {
                    sendToSafecastOps("Device ID not found.", SlackMsgReply)
                } else {
                    sendToSafecastOps(fmt.Sprintf("%s is %d", args[1], did), SlackMsgReply)
                }
            }
        }

    case "gateway":
        fallthrough
    case "gateways":
        sendSafecastGatewaySummaryToSlack("", fDetails)

    case "server":
        fallthrough
    case "servers":
        fallthrough
    case "ttserve":
        sendSafecastServerSummaryToSlack("", fDetails)

    case "status":
        if devicelist != "" {
            go sendSafecastDeviceSummaryToSlack(user, "", devicelist, false, fDetails)
        } else {
            if (false) {
                go sendSafecastDeviceSummaryToSlack(user, "== Offline ==", devicelist, true, fDetails)
                time.Sleep(2 * time.Second)
                go sendSafecastDeviceSummaryToSlack(user, "== Online ==", devicelist, false, fDetails)
            } else {
                sendToSafecastOps("Please use 'status <named-device-list>', using the 'device' command to manage lists. You may also use 'online' or 'offline' to see full status, but the output is *very* large.", SlackMsgReply)
            }
        }

    case "q":
        if len(args) < 2 {
            sendToSafecastOps("Command format: Q <query>", SlackMsgReply)
        } else {
            // Unescape the string, which substitutes &gt for >
            rawQuery := html.UnescapeString(messageAfterFirstWord)
            fmt.Printf("\n%s *** Database query: \"%s\"\n", LogTime(), rawQuery)
			// Perform the query
            numRows, url, _, qerr := logQuery(rawQuery, true, user)
            if qerr != nil {
                fmt.Printf("QUERY ERROR: %s\n", qerr)
                sendToSafecastOps(fmt.Sprintf("Query error: %s", qerr), SlackMsgReply)
            } else {
                sendToSafecastOps(fmt.Sprintf("%d rows of data are <%s|here>, %s.", numRows, url, user), SlackMsgReply)
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
            sendToSafecastOps("Command format: cancel <deviceID>", SlackMsgReply)
        } else {
            go sendSafecastDeviceCommand(user, args[1], "")
        }

    case "reboot-all":
    case "restart-all":
        sendToSafecastOps(fmt.Sprintf("Restarting all service instances."), SlackMsgReply)
        time.Sleep(2 * time.Second)
        ServerLog(fmt.Sprintf("*** RESTARTING because of Slack 'restart-all' command\n"))
        ControlFileTime(TTServerRestartAllControlFile, user)
        sendToSafecastOps(fmt.Sprintf("** %s restarting **", TTServeInstanceID), SlackMsgUnsolicitedOps)
        time.Sleep(3 * time.Second)
        os.Exit(0)

    case "reboot":
    case "restart":
        sendToSafecastOps(fmt.Sprintf("Restarting non-monitor service instances."), SlackMsgReply)
        ControlFileTime(TTServerRestartAllControlFile, user)

    case "send":
        if len(args) == 1 {
            sendToSafecastOps("Command format: send <deviceID> <message>", SlackMsgReply)
        } else if len(args) == 2 {
            switch argsLC[1] {
            case "hello":
                sendHelloToNewDevices()
            default:
                sendToSafecastOps("Unrecognized subcommand of 'send'", SlackMsgReply)
            }
        } else {
            go sendSafecastDeviceCommand(user, args[1], messageAfterSecondWord)
        }

    case "clear-logs-and-make-ray-sad":
        if len(args) == 2 {
            found, deviceID := WordsToNumber(args[1])
            if !found {
                sendToSafecastOps("Invalid Device ID.", SlackMsgReply)
            } else {
                sendToSafecastOps(DeleteLogs(deviceID), SlackMsgReply)
            }
        } else {
            sendToSafecastOps("Command format: clear <deviceID>", SlackMsgReply)
        }

    case "hello":
        if len(args) == 1 {
            sendToSafecastOps(fmt.Sprintf("Hello there. Nice day, isn't it, %s?", user), SlackMsgReply)
        } else {
            sendToSafecastOps(fmt.Sprintf("Back at you: %s", messageAfterFirstWord), SlackMsgReply)
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
    if destination == SlackMsgUnsolicitedAll {
        str := strings.Split(ServiceConfig.SlackOutboundUrls, ",")
        for _, url := range str {
            go sendToOpsViaSlack(msg, url)
        }
    } else if destination == SlackMsgUnsolicitedOps {
        str := strings.Split(ServiceConfig.SlackOutboundUrls, ",")
        go sendToOpsViaSlack(msg, str[SlackOpsSafecast])
    } else {
        if SlackCommandSource != SlackOpsNone {
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
