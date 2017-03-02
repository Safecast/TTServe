// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Common support for all HTTP topic handlers
package main

import (
	"fmt"
	"net"
    "net/http"
	"strings"
	"bytes"
)

// Extract just the IPV4 address, eliminating the port
func ipv4(Str1 string) string {
    Str2 := strings.Split(Str1, ":")
    if len(Str2) > 0 {
        return Str2[0]
    }
    return Str1
}

// Utility to extract the true IP address of a request forwarded by intermediate
// nodes such as the AWS Route 53 load balancer.  This is a vast improvement
// over just calling ipv4(req.RemoteAddr), which returns the internal LB address.
// Thanks to https://husobee.github.io/golang/ip-address/2015/12/17/remote-ip-go.html
func getRequestorIPv4(r *http.Request) (IPstr string, isReal bool) {
	for _, h := range []string{"X-Forwarded-For", "X-Real-Ip"} {
		addresses := strings.Split(r.Header.Get(h), ",")
		// march from right to left until we get a public address
		// that will be the address right before our proxy.
		for i := len(addresses) -1 ; i >= 0; i-- {
			ip := strings.TrimSpace(addresses[i])
			// header can contain spaces too, strip those out.
			realIP := net.ParseIP(ip)
//ozzie
			fmt.Printf("ip=%v realIP=%v\n", ip, realIP)
			if !realIP.IsGlobalUnicast() || isPrivateSubnet(realIP) {
				// bad address, go to next
				continue
			}
			return ip, true
		}
	}
	return ipv4(r.RemoteAddr), !isPrivateSubnet(net.ParseIP(ipv4(r.RemoteAddr)))
}

// Private IP ranges
// See https://en.wikipedia.org/wiki/Private_network
type ipRange struct {
	start net.IP
	end net.IP
}

var privateRanges = []ipRange{
	ipRange{
		start: net.ParseIP("10.0.0.0"),
		end:   net.ParseIP("10.255.255.255"),
	},
	ipRange{
		start: net.ParseIP("100.64.0.0"),
		end:   net.ParseIP("100.127.255.255"),
	},
	ipRange{
		start: net.ParseIP("172.16.0.0"),
		end:   net.ParseIP("172.31.255.255"),
	},
	ipRange{
		start: net.ParseIP("192.0.0.0"),
		end:   net.ParseIP("192.0.0.255"),
	},
	ipRange{
		start: net.ParseIP("192.168.0.0"),
		end:   net.ParseIP("192.168.255.255"),
	},
	ipRange{
		start: net.ParseIP("198.18.0.0"),
		end:   net.ParseIP("198.19.255.255"),
	},
}

// isPrivateSubnet - check to see if this ip is in a private subnet
func isPrivateSubnet(ipAddress net.IP) bool {
	// my use case is only concerned with ipv4 atm
	if ipCheck := ipAddress.To4(); ipCheck != nil {
		// iterate over all our ranges
		for _, r := range privateRanges {
			// check if this ip is in a private range
			if inRange(r, ipAddress){
				return true
			}
		}
	}
	return false
}

// Check to see if a given ip address is within a range given
func inRange(r ipRange, ipAddress net.IP) bool {
	// strcmp type byte comparison
	if bytes.Compare(ipAddress, r.start) >= 0 && bytes.Compare(ipAddress, r.end) < 0 {
		return true
	}
	return false
}
