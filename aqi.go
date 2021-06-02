// Copyright 2021 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"math"

	ttdata "github.com/Safecast/ttdefs"
)

// Calculate AQI
func aqiCalculate(sd *ttdata.SafecastData) {
	var aqi uint32
	var pm float64
	var aqiNotes, aqiLevel string

	// Perform calculations based on sensor type
	if sd.Opc != nil && sd.Opc.Pm02_5 != nil {
		pm, aqiNotes = adjustForHumidity(sd, *sd.Opc.Pm02_5, ttdata.AqiCFEN481)
		aqi, aqiLevel = pmToAqi(pm)
		sd.Opc.AqiLevel = &aqiLevel
		sd.Opc.AqiNotes = &aqiNotes
		sd.Opc.Aqi = &aqi
	}
	if sd.Pms2 != nil && sd.Pms2.Pm02_5 != nil {
		if sd.Pms2.Pm02_5cf1 != nil {
			pm, aqiNotes = adjustForHumidity(sd, *sd.Pms2.Pm02_5cf1, ttdata.AqiCF1)
			aqi, aqiLevel = pmToAqi(pm)
		} else {
			pm, aqiNotes = adjustForHumidity(sd, *sd.Pms2.Pm02_5, ttdata.AqiCFATM)
			aqi, aqiLevel = pmToAqi(pm)
		}
		sd.Pms2.AqiNotes = &aqiNotes
		sd.Pms2.AqiLevel = &aqiLevel
		sd.Pms2.Aqi = &aqi
	}
	if sd.Pms != nil && sd.Pms.Pm02_5 != nil {
		if sd.Pms.Pm02_5cf1 != nil {
			pm, aqiNotes = adjustForHumidity(sd, *sd.Pms.Pm02_5cf1, ttdata.AqiCF1)
			aqi, aqiLevel = pmToAqi(pm)
		} else {
			pm, aqiNotes = adjustForHumidity(sd, *sd.Pms.Pm02_5, ttdata.AqiCFATM)
			aqi, aqiLevel = pmToAqi(pm)
		}
		sd.Pms.AqiNotes = &aqiNotes
		sd.Pms.AqiLevel = &aqiLevel
		sd.Pms.Aqi = &aqi
	}

	// Done
	return

}

// Adjust humidity according to US EPA PM2.5 adjustment factor
// https://amt.copernicus.org/preprints/amt-2020-413/amt-2020-413.pdf
// https://cfpub.epa.gov/si/si_public_file_download.cfm?p_download_id=540979&Lab=CEMM
func adjustForHumidity(sd *ttdata.SafecastData, pmIn float64, notesIn string) (pmOut float64, notesOut string) {
	if sd.Env != nil && sd.Env.Humid != nil {
		pmOut = (0.524 * pmIn) - (0.0852 * *sd.Env.Humid) + 5.72
		notesOut += "," + ttdata.AqiUSEPAHumidity
	} else {
		pmOut = pmIn
		notesOut = notesIn
	}
	return
}

// Calculate AQI for PM2.5
// https://forum.airnowtech.org/t/the-aqi-equation/169
func pmToAqi(concIn float64) (aqi uint32, aqiLevel string) {
	var concLo, concHi, aqiLo, aqiHi float64

	// For all AQI calculations, the calculated average concentrations are truncated to 0.1 μg/m3 for PM2.5.
	// This truncated concentration is then used as the input (ConcIn) in the AQI equation. The resulting AQI
	// is rounded to the nearest whole number.
	itemp := uint32(concIn * 10)
	concIn = float64(itemp) / 10

	// Calculate level
	for {
		concLo = 0.0
		concHi = 12.0
		if concIn >= concLo && concIn <= concHi {
			aqiLo = 0
			aqiHi = 50
			aqiLevel = ttdata.AqiLevelGood
			break
		}
		concLo = concHi
		concHi = 35.4
		if concIn >= concLo && concIn <= concHi {
			aqiLo = 51
			aqiHi = 100
			aqiLevel = ttdata.AqiLevelModerate
			break
		}
		concLo = concHi
		concHi = 55.4
		if concIn >= concLo && concIn <= concHi {
			aqiLo = 101
			aqiHi = 150
			aqiLevel = ttdata.AqiLevelUnhealthyIfSensitive
			break
		}
		concLo = concHi
		concHi = 150.4
		if concIn >= concLo && concIn <= concHi {
			aqiLo = 151
			aqiHi = 200
			aqiLevel = ttdata.AqiLevelUnhealthy
			break
		}
		concLo = concHi
		concHi = 250.4
		if concIn >= concLo && concIn <= concHi {
			aqiLo = 201
			aqiHi = 300
			aqiLevel = ttdata.AqiLevelVeryUnhealthy
			break
		}
		concLo = concHi
		concHi = 500.4
		if concIn >= concLo && concIn <= concHi {
			aqiLo = 301
			aqiHi = 500
			aqiLevel = ttdata.AqiLevelHazardous
			break
		}
		// Level is higher than the top of the table.  Luckily,
		// the AQI stops at the same level as the concentration,
		// so we can return a number that looks meaningful.
		aqiLevel = ttdata.AqiLevelVeryHazardous
		aqi = uint32(math.Round(concIn))
		return
	}

	// Compute the AQI according to the equation
	aqi = uint32(math.Round((((aqiHi - aqiLo) / (concHi - concLo)) * (concIn - concLo)) + aqiLo))

	// Done
	return

}
