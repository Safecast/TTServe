// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Main service entry point
func main() {

	// Read our service config file
	ServiceConfig = ServiceReadConfig()

	// Spawn our signal handler
	go signalHandler()

	// Remember boot time
	stats.Started = time.Now()
	stats.Count.Restarts++

	// Get our external IP address
	rsp, err := http.Get("http://checkip.amazonaws.com")
	if err != nil {
		fmt.Printf("Can't get our own IP address: %v\n", err)
		os.Exit(0)
	}
	defer rsp.Body.Close()
	buf, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		fmt.Printf("Error fetching IP addr: %v\n", err)
		os.Exit(0)
	}
	ThisServerAddressIPv4 = string(bytes.TrimSpace(buf))
	stats.AddressIPv4 = ThisServerAddressIPv4

	// Get AWS info about this instance
	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html
	rsp, erraws := http.Get("http://169.254.169.254/latest/dynamic/instance-identity/document")
	if erraws != nil {
		fmt.Printf("Can't get our own instance info: %v\n", erraws)
		os.Exit(0)
	}
	defer rsp.Body.Close()
	buf, errread := ioutil.ReadAll(rsp.Body)
	if errread != nil {
		fmt.Printf("Error fetching instance info: %v\n", errread)
		os.Exit(0)
	}

	err = json.Unmarshal(buf, &stats.AWSInstance)
	if err != nil {
		fmt.Printf("*** Badly formatted AWS Info ***\n")
		os.Exit(0)
	}

	TTServeInstanceID = stats.AWSInstance.InstanceID
	ServerLog(fmt.Sprintf("*** STARTUP\n"))
	fmt.Printf("\n%s *** AWS %s %s\n", LogTime(), stats.AWSInstance.Region, stats.AWSInstance.InstanceID)

	// Init our utility packages, but only after we've got our server instance ID
	UtilInit()

	// Look up the two IP addresses that we KNOW have only a single A record,
	// and determine if WE are the server for those protocols
	for {
		addrs, err := net.LookupHost(TTServerUDPAddress)
		if err == nil {
			if len(addrs) >= 1 {
				TTServerUDPAddressIPv4 = addrs[0]
				break
			}
			err = fmt.Errorf("insufficient addr records for UDP")
		}
		fmt.Printf("Can't resolve %s: %v\n", TTServerUDPAddress, err)
		time.Sleep(3 * time.Second)
	}
	ThisServerServesUDP = TTServerUDPAddressIPv4 == ThisServerAddressIPv4

	// We have one server instance that is configured to field inbound requests
	// from web hooks configured on external websites.
	ThisServerIsMonitor = ThisServerServesUDP
	if ThisServerIsMonitor {
		fmt.Printf("THIS SERVER IS THE MONITOR INSTANCE\n")
	}

	// We all support TCP because it's load-balanced.
	ThisServerServesTCP := true

	// We all support HTTP because it's load-balanced.
	ThisServerServesHTTP := true

	// If and only if we're using MQTT (rather than TTN HTTP), do it on the UDP server
	if TTNMQTTMode {
		ThisServerServesMQTT = ThisServerServesUDP
	}

	// Get the date/time of the special files that we monitor
	AllServersSlackRestartRequestTime = ControlFileTime(TTServerRestartAllControlFile, "")

	// Init our web request inbound server
	if ThisServerServesHTTP {
		go HTTPInboundHandler()
		stats.Services = "HTTP"
	}

	// Init our UDP single-sample upload request inbound server
	if ThisServerServesUDP {
		go UDPInboundHandler()
		stats.Services += ", UDP"
	}

	// Init our TCP server
	if ThisServerServesTCP {
		go TCPInboundHandler()
		stats.Services += ", TCP"
	}

	// Spawn the TTNhandlers
	if ThisServerServesMQTT {
		go MQTTInboundHandler()
		stats.Services += ", MQTT"
	}

	// Spawn the broker publisher
	// DISABLED 2020-08 by Ray because CloudMQTT got rid of their free plan
	// and it doesn't appear that anyone was using this feature of ttserve.
	if false {
		go brokerOutboundPublisher()
	}

	// If this server is the monitor, indicate our other services
	if ThisServerIsMonitor {
		stats.Services += ", SLACK"
		stats.Services += ", GITHUB"
		stats.Services += ", WATCHDOG"
	}

	// One time at startup, refresh our knowledge of devices for the benefit of Slack UI
	if ThisServerIsMonitor {
		refreshDeviceSummaryLabels()
	}

	// Spawn the input handler
	go inputHandler()

	// Spawn timer tasks, assuming the role of one of them
	go timer12h()
	go timer1m()
	go timer15m()
	go timer5m()
	timer1m()

}

// Our app's signal handler
func signalHandler() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGTERM)
	signal.Notify(ch, syscall.SIGINT)
	signal.Notify(ch, syscall.SIGSEGV)
	for {
		switch <-ch {
		case syscall.SIGINT:
			fmt.Printf("*** Exiting %s because of SIGNAL \n", LogTime())
			os.Exit(0)
		case syscall.SIGTERM:
			break
		}
	}
}
