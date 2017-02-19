// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"
	"time"
	"strings"
)

// Get the current time in UTC as a string
func nowInUTC() string {
    return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

// Extract just the IPV4 address, eliminating the port
func ipv4(Str1 string) string {
    Str2 := strings.Split(Str1, ":")
    if len(Str2) > 0 {
        return Str2[0]
    }
    return Str1
}

// Function to clean up an error string to eliminate the filename
func errorString(err error) string {
    errString := fmt.Sprintf("%s", err)
    s0 := strings.Split(errString, ":")
	s1 := s0[len(s0)-1]
	s2 := strings.TrimSpace(s1)
    return s2
}
