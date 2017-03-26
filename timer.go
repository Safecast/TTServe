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

		// Stir the random pot
		for i:=0; i<random(1,10); i++ {
			random(0, 12345)
		}
		
        // Sleep
        time.Sleep(1 * time.Minute)

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
        time.Sleep(15 * time.Minute)

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

        // Update/output stats, returning "" on first iteration and when nothing has changed)
        summary := SafecastSummarizeStatsDelta()
        if summary != "" {
            ServerLog(fmt.Sprintf("%s\n", summary))
        }

        // Send a hello message to devices that have never reported stats
        if ThisServerIsMonitor {
            sendHelloToNewDevices()
        }

        // Snooze
        time.Sleep(12 * time.Hour)

    }
}

// Do a restart after a random delay
func RandomRestart() {

	// Stagger the instances so that we don't have a complete outage
	minutes := time.Duration(random(3, 15))

    // To ensure a best-efforts sequencing in log, impose a delay in proportion to sequencing
		sendToSafecastOps(fmt.Sprintf("** %s will restart in %d minutes **", TTServeInstanceID, minutes), SLACK_MSG_UNSOLICITED)
    time.Sleep(minutes * time.Minute)

    // Log
    ServerLog(fmt.Sprintf("*** RESTARTING because of Slack 'restart' command\n"))

    // Exit
    os.Exit(0)

}

// Check to see if we should restart
func ControlFileCheck() {

    // Exit if we're the monitor process
    if ThisServerIsMonitor {
        return
    }

    // Slack restart
    if (ControlFileTime(TTServerRestartAllControlFile, "") != AllServersSlackRestartRequestTime) {
        AllServersSlackRestartRequestTime = ControlFileTime(TTServerRestartAllControlFile, "")
        go RandomRestart()
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
