// IP-API JSON format, derived from:
// http://ip-api.com/docs/api:json

package main

import (
    "net"
)

type AWSInstanceIdentity struct {
	DevPayProductCodes		string  `json:"devpayProductCodes,omitempty"`
    PrivateIp				net.IP  `json:"privateIp,omitempty"`
	Region					string  `json:"region,omitempty"`
	KernelId				string  `json:"kernelId,omitempty"`
	RamdiskId				string  `json:"ramdiskId,omitempty"`
	AvailabilityZone		string  `json:"availabilityZone,omitempty"`
	AccountId				string  `json:"accountId,omitempty"`
	Version					string  `json:"version,omitempty"`
	InstanceId				string  `json:"instanceId,omitempty"`
	BillingProducts			string  `json:"billingProducts,omitempty"`
	Architecture			string  `json:"architecture,omitempty"`
	ImageId					string  `json:"imageId,omitempty"`
	PendingTime				string  `json:"pendingTime,omitempty"`
	InstanceType			string  `json:"instanceType,omitempty"`
}
