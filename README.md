# Safecast TTServe

Safecast TTServe is a data ingestion and processing server for Safecast environmental sensor networks. It handles inbound data from a wide variety of devices (including Pointcast, Safecast Air, GeigieCast, nGeigie, Solarcast, and Blues Notecard-based devices), processes and augments the data with metadata, and routes it to multiple databases and endpoints.

## Features
- Accepts data from HTTP, UDP, MQTT, and Blues/Notehub sources
- Adds essential metadata (device info, location, sensor status, etc.)
- Supports multiple device types and ID schemes
- Stores data in three main databases:
  - Flat file database (device logs, status as JSON)
  - API database (radiation measurements for Safecast)
  - Ingest database (all incoming data)
- Prevents circular data flows (e.g., Notehub data is not sent back to Notehub)
- Tracks device status and history

## Device ID Ranges
- **Pointcast:** 1000–1999, 100000–299999
- **Safecast Air:** 50000–59999
- **GeigieCast:** 60000–64999
- **GeigieCast Zen:** 65000–69999
- **nGeigie:** 1–999
- **Solarcast and Blues Notecard:** 0–1048575 (special CRC32-based scheme)

## Data Flow
1. **Inbound Data:** Received via HTTP, UDP, MQTT, or Notehub.
2. **Metadata Augmentation:** Adds device, location, and sensor metadata.
3. **Routing:**
   - Flat file: Device logs/status as JSON
   - API: Radiation data (SafecastDataV1)
   - Ingest: All data (ttdata.SafecastData)
4. **Device Tracking:** Maintains device status and history.

## Running TTServe

### Prerequisites
- Go 1.16+
- (Optional) MQTT broker for real-time data
- (Optional) Notehub credentials for Blues device integration

### Build and Run
```sh
go build
./TTServe [data-directory]
```

### Configuration
TTServe reads its configuration from a config file or environment variables. See `config.go` for details.

## Key Files
- `safecast.go`: Main data processing and upload logic
- `http-send.go`, `http-ttn.go`, `http-note.go`: HTTP handlers for various sources
- `dlog.go`, `dstatus.go`: Device logging and status tracking
- `reformat.go`, `sc-v1-defs.go`: Data format conversion and definitions

## Security
- Prevents circular uploads (e.g., Notehub data not sent back to Notehub)
- Tracks device activity and status for monitoring

## Contributing
Pull requests and issues are welcome! Please follow standard Go formatting and include tests where possible.

## License
Copyright (c) Safecast. Licensed under the terms found in the LICENSE file.
