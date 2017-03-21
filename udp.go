// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound UDP support
package main

import (
    "bytes"
    "net"
    "net/http"
    "fmt"
	"time"
    "encoding/json"
)

// Kick off UDP single-upload request server
func UdpInboundHandler() {

    fmt.Printf("Now handling inbound UDP on %s\n", TTServerUDPPort)

    ServerAddr, err := net.ResolveUDPAddr("udp", TTServerUDPPort)
    if err != nil {
        fmt.Printf("Error resolving UDP port: \n%v\n", err)
        return
    }

    ServerConn, err := net.ListenUDP("udp", ServerAddr)
    if err != nil {
        fmt.Printf("Error listening on UDP port: \n%v\n", err)
        return
    }
    defer ServerConn.Close()

    for {
        buf := make([]byte, 8192)

        n, addr, err := ServerConn.ReadFromUDP(buf)
        if (err != nil) {
            fmt.Printf("UDP read error: \n%v\n", err)
        } else {

            ttg := &TTGateReq{}
            ttg.Payload = buf[0:n]
            ttg.Transport = "device-udp:" + ipv4(addr.String())
            data, err := json.Marshal(ttg)
            if err == nil {
                go UploadToWebLoadBalancer(data, n, ttg.Transport)
                stats.Count.UDP++;
            }

        }

    }

}

// Upload a Safecast data structure the load balancer for the web service
func UploadToWebLoadBalancer(data []byte, datalen int, transport string) {

    if true {
        fmt.Printf("\n%s Received %d-byte payload from %s, routing to LB\n", time.Now().Format(logDateFormat), datalen, transport)
    }

    url := "http://" + TTServerHTTPAddress + TTServerHTTPPort + TTServerTopicSend

    req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
    req.Header.Set("User-Agent", "TTSERVE")
    req.Header.Set("Content-Type", "text/plain")
    httpclient := &http.Client{
        Timeout: time.Second * 15,
    }
    resp, err := httpclient.Do(req)
    if err != nil {
        fmt.Printf("HTTP POST error: %v\n", err);
    } else {
        resp.Body.Close()
    }

}
