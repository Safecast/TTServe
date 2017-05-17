// IP-API JSON format, derived from:
// http://ip-api.com/docs/api:json

package main

import (
    "net"
)

// AWSInstanceIdentity is a structure returned by a query to AWS
type AWSInstanceIdentity struct {
	DevPayProductCodes		string  `json:"devpayProductCodes,omitempty"`
    PrivateIP				net.IP  `json:"privateIp,omitempty"`
	Region					string  `json:"region,omitempty"`
	KernelID				string  `json:"kernelId,omitempty"`
	RamdiskID				string  `json:"ramdiskId,omitempty"`
	AvailabilityZone		string  `json:"availabilityZone,omitempty"`
	AccountID				string  `json:"accountId,omitempty"`
	Version					string  `json:"version,omitempty"`
	InstanceID				string  `json:"instanceId,omitempty"`
	BillingProducts			string  `json:"billingProducts,omitempty"`
	Architecture			string  `json:"architecture,omitempty"`
	ImageID					string  `json:"imageId,omitempty"`
	PendingTime				string  `json:"pendingTime,omitempty"`
	InstanceType			string  `json:"instanceType,omitempty"`
}
