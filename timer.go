// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"time"
)

// General periodic housekeeping
func timer1m() {
	for {

		// Restart this instance if instructed to do so
		ControlFileCheck()

		// Write out current status to the file system
		WriteServerStatus()

		// Stir the random pot
		for i := 0; i < Random(1, 10); i++ {
			Random(0, 12345)
		}

		// Sleep
		time.Sleep(1 * time.Minute)

	}
}

// General periodic housekeeping
func timer5m() {
	for {

		// On the monitor role, track expired devices.
		// We do this before the first sleep so we have a list of device ASAP
		if ThisServerIsMonitor {
			sendExpiredSafecastDevicesToSlack()
			sendExpiredSafecastGatewaysToSlack()
			sendExpiredSafecastServersToSlack()
		}

		// Sleep
		time.Sleep(5 * time.Minute)

	}
}

// General periodic housekeeping
func timer15m() {
	for {

		// Sleep
		time.Sleep(15 * time.Minute)

		// Post long TTN outages
		if ThisServerServesMQTT {
			MQTTSubscriptionNotifier()
		}

	}

}

// General periodic housekeeping
func timer1h() {
	for {

		// Sleep
		time.Sleep(60 * time.Minute)

		// Post Safecast errors, but only on the monitor process.  We only do this to prevent
		// noise, and under the assumption that if it happens to one instance it is happening to
		// all of them.
		if ThisServerIsMonitor {
			sendSafecastCommsErrorsToSlack(60)
		}

	}

}

// General periodic housekeeping
func timer12h() {
	for {

		// Update/output stats, returning "" on first iteration and when nothing has changed)
		summary := SummarizeStatsDelta()
		if summary != "" {
			ServerLog(fmt.Sprintf("%s\n", summary))
		}

		// Snooze
		time.Sleep(12 * time.Hour)

	}
}

// RandomRestart does a restart of this instance after a random delay
func RandomRestart() {

	// Stagger the instances so that we don't have a complete outage
	minutes := time.Duration(Random(3, 15))

	// To ensure a best-efforts sequencing in log, impose a delay in proportion to sequencing
	sendToSafecastOps(fmt.Sprintf("** %s will restart in %d minutes **", TTServeInstanceID, minutes), SlackMsgUnsolicitedOps)
	time.Sleep(minutes * time.Minute)

	// Log
	ServerLog("*** RESTARTING because of Slack 'restart' command\n")

	// Exit
	os.Exit(0)

}

// ControlFileCheck checks to see if we should restart
func ControlFileCheck() {

	// Exit if we're the monitor process
	if ThisServerIsMonitor {
		return
	}

	// Slack restart
	if ControlFileTime(TTServerRestartAllControlFile, "") != AllServersSlackRestartRequestTime {
		AllServersSlackRestartRequestTime = ControlFileTime(TTServerRestartAllControlFile, "")
		go RandomRestart()
	}

}

// ControlFileTime gets the modified time of a special file
func ControlFileTime(controlfilename string, message string) (restartTime time.Time) {

	filename := SafecastDirectory() + TTServerControlPath + "/" + controlfilename

	// Overwrite the file if requested to do so
	if message != "" {
		fd, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
		if err == nil {
			fd.WriteString(message)
			fd.Close()
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
