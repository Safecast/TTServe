// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound TCP support
package main

import (
    "net"
    "fmt"
    "encoding/json"
)

// Kick off TCP single-upload request server
func TcpInboundHandler() {

    fmt.Printf("Now handling inbound TCP on %s\n", TTServerTCPPort)

    ServerAddr, err := net.ResolveTCPAddr("tcp", TTServerTCPPort)
    if err != nil {
        fmt.Printf("Error resolving TCP port: \n%v\n", err)
        return
    }

    ServerConn, err := net.ListenTCP("tcp", ServerAddr)
    if err != nil {
        fmt.Printf("Error listening on TCP port: \n%v\n", err)
        return
    }
    defer ServerConn.Close()

    for {
        buf := make([]byte, 4096)

		conn, err := ServerConn.AcceptTCP()
		if err != nil {
			fmt.Printf("Error accepting TCP session: \n%v\n", err)
			continue
		}

		n, err := conn.Read(buf)
		if err != nil {
			fmt.Printf("TCP read error: \n%v\n", err)
			conn.Close()
			continue
		}

		remoteaddr := ipv4(conn.RemoteAddr().String())

		conn.Close()

        ttg := &TTGateReq{}
        ttg.Payload = buf[0:n]
        ttg.Transport = "device-tcp:" + remoteaddr
        data, err := json.Marshal(ttg)
        if err == nil {
            go UploadToWebLoadBalancer(data, n, ttg.Transport)
            stats.Count.TCP++;
        }


    }

}
