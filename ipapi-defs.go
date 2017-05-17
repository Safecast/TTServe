// IP-API JSON format, derived from:
// http://ip-api.com/docs/api:json

package main

import (
    "net"
)

// IPInfoData is the data structure returned by IP-API
type IPInfoData struct {
    IP           net.IP  `json:"query,omitempty"`
	Message		 string  `json:"message,omitempty"`
	Status		 string  `json:"status,omitempty"`
    AS           string  `json:"as,omitempty"`
    City         string  `json:"city,omitempty"`
    Country      string  `json:"country,omitempty"`
    CountryCode  string  `json:"countryCode,omitempty"`
    ISP          string  `json:"isp,omitempty"`
    Latitude     float32 `json:"lat,omitempty"`
    Longitude    float32 `json:"lon,omitempty"`
    Organization string  `json:"org,omitempty"`
    Region       string  `json:"region,omitempty"`
    RegionName   string  `json:"regionName,omitempty"`
    Timezone     string  `json:"timezone,omitempty"`
    Zip          string  `json:"zip,omitempty"`
}
