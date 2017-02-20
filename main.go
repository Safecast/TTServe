// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
    "bytes"
    "os"
    "os/signal"
    "syscall"
    "io/ioutil"
    "net/http"
    "fmt"
    "net"
    "time"
)

// Main entry point for app
func main() {

    // Spawn our signal handler
    go signalHandler()

    // Remember boot time
    ThisServerBootTime = time.Now()

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

	// Configure FTP
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
    AllServersSlackHealthRequestTime = ControlFileTime(TTServerHealthControlFile, "")

	// Synchronously init the app request queue before anyone tries to service it or push to it
    AppReqInit()

    // Spawn the app request handler shared by both TTN and direct inbound server
    go AppReqHandler()

    // Init our web request inbound server
    go HttpInboundHandler()

    // Init our UDP single-sample upload request inbound server
    if ThisServerServesUDP {
        go UdpInboundHandler()
    }

    // Init our FTP server
    if ThisServerServesFTP {
        go FtpInboundHandler()
    }

    // Spawn the TTNhandlers
    if ThisServerServesMQQT {
        go MqqtInboundHandler()
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
            fmt.Printf("\n***\n***\n*** Exiting at user's request \n***\n***\n\n")
            os.Exit(0)
        case syscall.SIGTERM:
			FtpStop()
            break
        }
    }
}
