// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound TCP support
package main

import (
    "net"
    "fmt"
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

		if (true) {
			fmt.Printf("******* ACCEPTED TCP CONNECTION *******\n");
		}

        n, err := conn.Read(buf)
        if err != nil {
            fmt.Printf("TCP read error: \n%v\n", err)
            conn.Close()
            continue
        }

        remoteaddr := ipv4(conn.RemoteAddr().String())

        // Initialize a new AppReq
        AppReq := IncomingAppReq{}
        AppReq.SvTransport = "device-tcp:" + remoteaddr

        // Push it
        ReplyToDeviceId := AppReqPushPayload(AppReq, buf[0:n], "device directly")
        stats.Count.TCP++;

        // Is there a device ID to reply to?
        if (ReplyToDeviceId != 0) {

            // See if there's an outbound message waiting for this device.
            isAvailable, payload := TelecastOutboundPayload(ReplyToDeviceId)
            if (isAvailable) {

                // Responses are binary on TCP
                conn.Write(payload)
                sendToSafecastOps(fmt.Sprintf("Device %d picked up its pending command\n", ReplyToDeviceId), SLACK_MSG_UNSOLICITED)
            }

        }

        // Close the connection
        conn.Close()

    }

}
