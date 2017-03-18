// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"os"
	"fmt"
	"math/rand"
	"time"
	"strings"
)

// Initialize package
func UtilInit() {

	// Initialize the random number generator
	rand.Seed(time.Now().Unix())

}

// Get a random number in a range
func random(min, max int) int {
	return rand.Intn(max - min) + min
}

// Get path of the safecast directory
func SafecastDirectory() string {
    directory := os.Args[1]
    if (directory == "") {
        fmt.Printf("TTSERVE: first argument must be folder containing safecast data!\n")
        os.Exit(0)
    }
    return(directory)
}

// Get the current time in UTC as a string
func nowInUTC() string {
    return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

// How long ago, readably, given a count of minutes
func AgoMinutes(minutesAgo uint32) string {
    var hoursAgo uint32 = minutesAgo / 60
    var daysAgo uint32 = hoursAgo / 24
    minutesAgo -= hoursAgo * 60
    hoursAgo -= daysAgo * 24
	s := ""
    if daysAgo == 14 {
        s = fmt.Sprintf("1 week")
    } else if daysAgo > 14 {
        s = fmt.Sprintf("%d weeks", daysAgo/14)
    } else if daysAgo > 2 {
        s = fmt.Sprintf("%d days", daysAgo)
    } else if daysAgo != 0 {
        s = fmt.Sprintf("%dd %dh", daysAgo, hoursAgo)
    } else if hoursAgo != 0 {
        s = fmt.Sprintf("%dh %dm", hoursAgo, minutesAgo)
    } else if minutesAgo < 1 {
        s = fmt.Sprintf("<1m")
    } else if minutesAgo < 100 {
        s = fmt.Sprintf("%02dm", minutesAgo)
    } else {
        s = fmt.Sprintf("%dm", minutesAgo)
    }
	return s
}

// How long ago, readably, given a time
func Ago(when time.Time) string {
	return AgoMinutes(uint32(int64(time.Now().Sub(when) / time.Minute)))
}

// Function to clean up an error string to eliminate the filename
func errorString(err error) string {
    errString := fmt.Sprintf("%s", err)
    s0 := strings.Split(errString, ":")
	s1 := s0[len(s0)-1]
	s2 := strings.TrimSpace(s1)
    return s2
}
