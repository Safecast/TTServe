// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Common support for all HTTP topic handlers
package main

import (
	"strings"
)

// Filter abusive ports
func isAbusiveIP(ipaddr string) bool {

	// Block all from the tencent cloud, which is constantly hammering us
	if strings.HasPrefix(ipaddr, "118.24.") || strings.HasPrefix(ipaddr, "118.25.") {
		return true
	}

	// Not known to be abusive
	return false

}
