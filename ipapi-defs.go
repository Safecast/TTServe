// IP-API JSON format, derived from:
// http://ip-api.com/docs/api:json
package main

import (
    "net"
)

type IPInfoData struct {
    AS           string  `json:"as"`
    City         string  `json:"city"`
    Country      string  `json:"country"`
    CountryCode  string  `json:"countryCode"`
    ISP          string  `json:"isp"`
    Latitude     float32 `json:"lat"`
    Longitude    float32 `json:"lon"`
    Organization string  `json:"org"`
    IP           net.IP  `json:"query"`
    Region       string  `json:"region"`
    RegionName   string  `json:"regionName"`
    Timezone     string  `json:"timezone"`
    Zip          string  `json:"zip"`
}
