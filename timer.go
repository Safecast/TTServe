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
		
        // Restart this instance if instructed to do so
        ControlFileCheck()

		// Write out current status to the file system
		SafecastWriteServerStatus()

		// Sleep
        time.Sleep(1 * 60 * time.Second)
		
    }
}

// General periodic housekeeping
func timer15m() {
    for {

        // On the monitor role, track expired devices.
        // We do this before the first sleep so we have a list of device ASAP
        if ThisServerIsMonitor {
            sendExpiredSafecastDevicesToSlack()
            sendExpiredSafecastGatewaysToSlack()
            sendExpiredSafecastServersToSlack()
        }

        // Sleep
        time.Sleep(15 * 60 * time.Second)

		// Update and output the stats
		summary := SafecastSummarizeStatsDelta()
		if summary != "" {
			ServerLog(fmt.Sprintf("%s\n", summary))
		}

        // Post Safecast errors
        sendSafecastCommsErrorsToSlack(15)

        // Post long TTN outages
        if ThisServerServesMQQT {
            MqqtSubscriptionNotifier()
        }

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

// Check to see if we should restart
func ControlFileCheck() {

    // Slack restart
    if (ControlFileTime(TTServerRestartAllControlFile, "") != AllServersSlackRestartRequestTime) {
        sendToSafecastOps(fmt.Sprintf("** %s restarting **", TTServeInstanceID), SLACK_MSG_UNSOLICITED)
        ServerLog(fmt.Sprintf("*** RESTARTING because of Slack 'restart' command\n"))
        os.Exit(0)
    }

    // Github restart
    if (ControlFileTime(TTServerRestartGithubControlFile, "") != AllServersGithubRestartRequestTime) {
        ServerLog(fmt.Sprintf("*** RESTARTING because of Github push command\n"))
        os.Exit(0)
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
        return stats.Started
    }

    return file.ModTime()
}
