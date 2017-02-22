// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"os"
	"fmt"
	"time"
)

// General periodic housekeeping
func timer1m() {
    for {
        time.Sleep(1 * 60 * time.Second)

        // Restart this instance if instructed to do so
        ControlFileCheck()

    }
}

// General periodic housekeeping
func timer15m() {
    for {

        // On the monitor role, track expired devices.
        // We do this before the first sleep so we have a list of device ASAP
        if ThisServerIsMonitor {
            sendExpiredSafecastDevicesToSlack()
        }

        // Sleep
        time.Sleep(15 * 60 * time.Second)

        // Post Safecast errors
        sendSafecastCommsErrorsToSlack(15)

        // Post long TTN outages
        if ThisServerServesMQQT {
            MqqtSubscriptionNotifier()
        }

        // Post stats
        ILog(fmt.Sprintf("Stats: UDP:%d HTTPDevice:%d HTTPGateway:%d HTTPRelay:%d HTTPRedirect:%d TTN:%d\n\n",
            CountUDP, CountHTTPDevice, CountHTTPGateway, CountHTTPRelay, CountHTTPRedirect, CountTTN))

    }

}

// General periodic housekeeping
func timer12h() {
    for {

        // Send a hello message to devices that have never reported stats
        if ThisServerIsMonitor {
            sendHelloToNewDevices()
        }

        // Snooze
        time.Sleep(12 * 60 * 60 * time.Second)

    }
}

// Server health check
func ServerHealthCheck() string {
    log := fmt.Sprintf("<http://%s%s%d|log>", TTServerHTTPAddress, TTServerTopicInstance, InstanceLogFilename(".log"), TTServeInstanceID)
    s := ""
    var minutesAgo uint32 = uint32(int64(time.Now().Sub(ThisServerBootTime) / time.Minute))
    var hoursAgo uint32 = minutesAgo / 60
    var daysAgo uint32 = hoursAgo / 24
    minutesAgo -= hoursAgo * 60
    hoursAgo -= daysAgo * 24
    if daysAgo != 0 {
        s = fmt.Sprintf("%s %s last restarted %dd %dh %dm ago", TTServeInstanceID, log, daysAgo, hoursAgo, minutesAgo)
    } else if hoursAgo != 0 {
        s = fmt.Sprintf("%s %s last restarted %dh %dm ago", TTServeInstanceID, hoursAgo, minutesAgo)
    } else {
        s = fmt.Sprintf("%s %s last restarted %dm ago", TTServeInstanceID, minutesAgo)
    }
    return s
}

// Check to see if we should restart
func ControlFileCheck() {

    // Slack restart
    if (ControlFileTime(TTServerRestartAllControlFile, "") != AllServersSlackRestartRequestTime) {
        sendToSafecastOps(fmt.Sprintf("** %s restarting **", ThisServerAddressIPv4), SLACK_MSG_UNSOLICITED)
        ILog(fmt.Sprintf("\n***\n***\n*** RESTARTING because of Slack 'restart' command\n***\n***\n\n"))
        os.Exit(0)
    }

    // Github restart
    if (ControlFileTime(TTServerRestartGithubControlFile, "") != AllServersGithubRestartRequestTime) {
        ILog(fmt.Sprintf("\n***\n***\n*** RESTARTING because of Github push command\n***\n***\n\n"))
        os.Exit(0)
    }

    // Heath
    if (ControlFileTime(TTServerHealthControlFile, "") != AllServersSlackHealthRequestTime) {
        sendToSafecastOps(ServerHealthCheck(), SLACK_MSG_UNSOLICITED)
        AllServersSlackHealthRequestTime = ControlFileTime(TTServerHealthControlFile, "")
    }

}

// Get the modified time of a special file
func ControlFileTime(controlfilename string, message string) (restartTime time.Time) {

    filename := SafecastDirectory() + TTServerControlPath + "/" + controlfilename

    // Overwrite the file if requested to do so
    if (message != "") {
        fd, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
        if (err == nil) {
            fd.WriteString(message);
            fd.Close();
        }
    }

    // Get the file date/time, returning a stable time if we fail
    file, err := os.Stat(filename)
    if err != nil {
        fmt.Printf("*** Error fetching file time for %s: %v\n", filename, err)
        return ThisServerBootTime
    }

    return file.ModTime()
}
