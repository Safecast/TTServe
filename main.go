// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
    "os"
    "net"
    "fmt"
    "bytes"
    "os/signal"
    "syscall"
    "io/ioutil"
    "net/http"
    "time"
    "encoding/json"
)

// Main service entry point
func main() {

	// Init our utility package
	UtilInit()
	
    // Spawn our signal handler
    go signalHandler()

    // Remember boot time
	stats.Started = time.Now()
	stats.Count.Restarts++
	
    // Get our external IP address
    rsp, err := http.Get("http://checkip.amazonaws.com")
    if err != nil {
        fmt.Printf("Can't get our own IP address: %v\n", err);
        os.Exit(0)
    }
    defer rsp.Body.Close()
    buf, err := ioutil.ReadAll(rsp.Body)
    if err != nil {
        fmt.Printf("Error fetching IP addr: %v\n", err);
        os.Exit(0)
    }
    ThisServerAddressIPv4 = string(bytes.TrimSpace(buf))
	stats.AddressIPv4 = ThisServerAddressIPv4
	
	// Get AWS info about this instance
	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html
    rsp, erraws := http.Get("http://169.254.169.254/latest/dynamic/instance-identity/document")
    if erraws != nil {
        fmt.Printf("Can't get our own instance info: %v\n", erraws);
        os.Exit(0)
    }
    defer rsp.Body.Close()
    buf, errread := ioutil.ReadAll(rsp.Body)
    if errread != nil {
        fmt.Printf("Error fetching instance info: %v\n", errread);
        os.Exit(0)
    }
	
    err = json.Unmarshal(buf, &stats.AWSInstance)
    if err != nil {
        fmt.Printf("*** Badly formatted AWS Info ***\n");
		os.Exit(0)
    }

	TTServeInstanceID = stats.AWSInstance.InstanceId
	ServerLog(fmt.Sprintf("*** STARTUP\n"))
	fmt.Printf("%s *** AWS %s %s\n", time.Now().Format(logDateFormat), stats.AWSInstance.Region, stats.AWSInstance.InstanceId)

    // Look up the two IP addresses that we KNOW have only a single A record,
    // and determine if WE are the server for those protocols
    addrs, err := net.LookupHost(TTServerUDPAddress)
    if err != nil {
        fmt.Printf("Can't resolve %s: %v\n", TTServerUDPAddress, err);
        os.Exit(0)
    }
    if len(addrs) < 1 {
        fmt.Printf("Can't resolve %s: %v\n", TTServerUDPAddress, err);
        os.Exit(0)
    }
    TTServerUDPAddressIPv4 = addrs[0]
    ThisServerServesUDP = TTServerUDPAddressIPv4 == ThisServerAddressIPv4

	// We all support TCP because it's load-balanced.
    ThisServerServesTCP := true

	// We all support HTTP because it's load-balanced.
	ThisServerServesHTTP := true
	
	// Configure FTP, which only runs on the primary server because it's not load-balanced.
    addrs, err = net.LookupHost(TTServerFTPAddress)
    if err != nil {
        fmt.Printf("Can't resolve %s: %v\n", TTServerFTPAddress, err);
        os.Exit(0)
    }
    if len(addrs) < 1 {
        fmt.Printf("Can't resolve %s: %v\n", TTServerFTPAddress, err);
        os.Exit(0)
    }
    TTServerFTPAddressIPv4 = addrs[0]
    ThisServerServesFTP = TTServerFTPAddressIPv4 == ThisServerAddressIPv4

	// If and only if we're using MQQT (rather than TTN HTTP), do it on the UDP server
	if TTNMQQTMode {
	    ThisServerServesMQQT = ThisServerServesUDP
	}

	// We have one server instance that is configured to field inbound requests
	// from web hooks configured on external websites.
    ThisServerIsMonitor = ThisServerServesFTP

    // Get the date/time of the special files that we monitor
    AllServersSlackRestartRequestTime = ControlFileTime(TTServerRestartAllControlFile, "")
    AllServersGithubRestartRequestTime = ControlFileTime(TTServerRestartGithubControlFile, "")

    // Init our web request inbound server
	if ThisServerServesHTTP {
	    go HttpInboundHandler()
		stats.Services = "HTTP"
	}

    // Init our UDP single-sample upload request inbound server
    if ThisServerServesUDP {
        go UdpInboundHandler()
		stats.Services += ", UDP"
    }

    // Init our TCP server
    if ThisServerServesTCP {
        go TcpInboundHandler()
		stats.Services += ", TCP"
    }

    // Init our FTP server
    if ThisServerServesFTP {
        go FtpInboundHandler()
		stats.Services += ", FTP"
    }

    // Spawn the TTNhandlers
    if ThisServerServesMQQT {
        go MqqtInboundHandler()
		stats.Services += ", MQQT"
    }

	// If this server is the monitor, indicate our other services
	if ThisServerIsMonitor {
		stats.Services += ", SLACK"
		stats.Services += ", GITHUB"
		stats.Services += ", WATCHDOG"
	}

    // Spawn timer tasks, assuming the role of one of them
    go timer12h()
    go timer15m()
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
            fmt.Printf("*** Exiting %s because of SIGNAL \n", time.Now().Format(logDateFormat))
            os.Exit(0)
        case syscall.SIGTERM:
			FtpStop()
            break
        }
    }
}
