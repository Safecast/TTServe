# TTServe DuckDB Implementation - Terminal Commands (Session 2)

This file contains the terminal commands executed during the second session of implementing DuckDB databases in the Safecast TTServe project.

## Creating a Minimal Configuration File

```bash
mkdir -p /home/rob/Documents/Safecast/TTServe/config
```

## Running TTServe

```bash
./TTServe /home/rob/Documents/Safecast/TTServe
```

Output:
```
DuckDB databases initialized successfully
Can't get our own instance info: Get "http://169.254.169.254/latest/dynamic/instance-identity/document": dial tcp 169.254.169.254:80: i/o timeout
```

## Testing TTServe with DuckDB

```bash
./test_direct.sh
```

Output:
```
Testing TTServe with DuckDB implementation
----------------------------------------
Sending test data to TTServe...
curl: (7) Failed to connect to localhost port 8080 after 0 ms: Couldn't connect to server


Verifying data in DuckDB databases:
----------------------------------------
Checking API database:
Error: unable to open database "/home/rob/Documents/Safecast/TTServe/api.duckdb": IO Error: Could not set lock on file "/home/rob/Documents/Safecast/TTServe/api.duckdb": Conflicting lock is held in /home/rob/Documents/Safecast/TTServe/TTServe (PID 51498). See also https://duckdb.org/docs/connect/concurrency

Checking Ingest database:
Error: unable to open database "/home/rob/Documents/Safecast/TTServe/ingest.duckdb": IO Error: Could not set lock on file "/home/rob/Documents/Safecast/TTServe/ingest.duckdb": Conflicting lock is held in /home/rob/Documents/Safecast/TTServe/TTServe (PID 51498). See also https://duckdb.org/docs/connect/concurrency
```

## Debugging TTServe

```bash
./debug_ttserve.sh
```

Output:
```
TTServe Debugging Tool
======================
Checking for TTServe process:
TTServe does not appear to be running

Testing TTServe endpoints:
-------------------------
Trying port 80...
000Failed to connect

Trying port 8080...
000Failed to connect

Checking DuckDB database files:
-----------------------------
-rw-rw-r-- 1 rob rob 12288 May  7 22:22 /home/rob/Documents/Safecast/TTServe/api.duckdb
-rw-rw-r-- 1 rob rob 12288 May  7 22:22 /home/rob/Documents/Safecast/TTServe/ingest.duckdb
```

## Creating Required Directories

```bash
mkdir -p /home/rob/Documents/Safecast/TTServe/server-status /home/rob/Documents/Safecast/TTServe/control
mkdir -p /home/rob/Documents/Safecast/TTServe/server-log
```

## Modifying and Building TTServe

```bash
go build
```

## Running TTServe with Modified Code

```bash
./TTServe /home/rob/Documents/Safecast/TTServe
```

Output:
```
DuckDB databases initialized successfully
Running in local development mode, bypassing AWS instance check
2025-05-07 22:44:05 *** STARTUP

2025-05-07 22:44:05 *** AWS local local-dev
Now handling inbound HTTP on 206.83.109.149::80
Now handling inbound TCP on 206.83.109.149:8082
Now handling inbound HTTP on 206.83.109.149::8080
```

## Testing with Updated User-Agent

```bash
./test_direct.sh
```

Output:
```
Testing TTServe with DuckDB implementation
----------------------------------------
Sending test data to TTServe...
Response from TTServe:



Verifying data in DuckDB databases:
----------------------------------------
Checking API database:
┌───────┬─────────────┬───────────┬────────┬─────────┬──────────┬───────────┬────────┬───────────────┬────────────┬─────────────┬───────────┬────────────┬─────────┬───────────────┬─────────────┐
│  id   │ captured_at │ device_id │ value  │  unit   │ latitude │ longitude │ height │ location_name │ channel_id │ original_id │ sensor_id │ station_id │ user_id │ devicetype_id │ uploaded_at │
│ int32 │  timestamp  │   int32   │ double │ varchar │  double  │  double   │ double │    varchar    │   int32    │    int32    │   int32   │   int32    │  int32  │    varchar    │  timestamp  │
├───────┴─────────────┴───────────┴────────┴─────────┴──────────┴───────────┴────────┴───────────────┴────────────┴─────────────┴───────────┴────────────┴─────────┴───────────────┴─────────────┤
│                                                                                             0 rows                                                                                             │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

Checking Ingest database:
┌───────┬────────────┬──────────────┬───────────┬───────────┬───────────────┬──────────────────┬───────────────────┬─────────────────┬──────┬─────────────┐
│  id   │ device_urn │ device_class │ device_sn │ device_id │ when_captured │ service_uploaded │ service_transport │ service_handler │ data │ uploaded_at │
│ int32 │  varchar   │   varchar    │  varchar  │   int32   │   timestamp   │    timestamp     │      varchar      │     varchar     │ json │  timestamp  │
├───────┴────────────┴──────────────┴───────────┴───────────┴───────────────┴──────────────────┴───────────────────┴─────────────────┴──────┴─────────────┤
│                                                                         0 rows                                                                          │
└─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

## Checking Running Processes

```bash
ps aux | grep TTServe
```

Output:
```
rob        45572  6.9  3.9 5678688 2572132 ?     Sl   22:07   1:28 /usr/share/windsurf/resources/app/extensions/windsurf/bin/language_server_linux_x64 --api_server_url https://server.codeium.com --run_child --enable_lsp --extension_server_port 36381 --ide_name windsurf --csrf_token 822c9863-7bee-4d29-8b87-a5a71a211f7b --random_port --inference_api_server_url https://inference.codeium.com --database_dir /home/rob/.codeium/windsurf/database/9c0694567290725d9dcba14ade58e297 --enable_index_service --enable_local_search --search_max_workspace_file_count 5000 --indexed_files_retention_period_days 30 --workspace_id file_home_rob_Documents_Safecast_TTServe_code_workspace --sentry_telemetry --sentry_environment stable --codeium_dir .codeium/windsurf --parent_pipe_path /tmp/server_8c85952933906c9c --windsurf_version 1.8.2
rob        52872  0.0  0.0 4260560 34296 pts/0   Sl+  22:27   0:00 ./TTServe /home/rob/Documents/Safecast/TTServe
rob        53201  0.0  0.0  10332  2432 pts/3    S+   22:28   0:00 grep --color=auto TTServe
```

## Stopping TTServe Process

```bash
kill -9 52872
```

## Checking DuckDB Databases

```bash
duckdb /home/rob/Documents/Safecast/TTServe/api.duckdb -c "SELECT * FROM measurements;"
```

Output:
```
┌───────┬─────────────┬───────────┬────────┬─────────┬──────────┬───────────┬────────┬───────────────┬────────────┬─────────────┬───────────┬────────────┬─────────┬───────────────┬─────────────┐
│  id   │ captured_at │ device_id │ value  │  unit   │ latitude │ longitude │ height │ location_name │ channel_id │ original_id │ sensor_id │ station_id │ user_id │ devicetype_id │ uploaded_at │
│ int32 │  timestamp  │   int32   │ double │ varchar │  double  │  double   │ double │    varchar    │   int32    │    int32    │   int32   │   int32    │  int32  │    varchar    │  timestamp  │
├───────┴─────────────┴───────────┴────────┴─────────┴──────────┴───────────┴────────┴───────────────┴────────────┴─────────────┴───────────┴────────────┴─────────┴───────────────┴─────────────┤
│                                                                                             0 rows                                                                                             │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

```bash
duckdb /home/rob/Documents/Safecast/TTServe/ingest.duckdb -c "SELECT * FROM measurements;"
```

Output:
```
┌───────┬────────────┬──────────────┬───────────┬───────────┬───────────────┬──────────────────┬───────────────────┬─────────────────┬──────┬─────────────┐
│  id   │ device_urn │ device_class │ device_sn │ device_id │ when_captured │ service_uploaded │ service_transport │ service_handler │ data │ uploaded_at │
│ int32 │  varchar   │   varchar    │  varchar  │   int32   │   timestamp   │    timestamp     │      varchar      │     varchar     │ json │  timestamp  │
├───────┴────────────┴──────────────┴───────────┴───────────┴───────────────┴──────────────────┴───────────────────┴─────────────────┴──────┴─────────────┤
│                                                                         0 rows                                                                          │
└─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

## Implementing Measurement ID Return

```bash
go build
```

## Testing Simple DuckDB Operations

```bash
chmod +x test_duckdb_simple.sh
./test_duckdb_simple.sh
```

## Recommended Test Commands for Future Testing

```bash
# Run TTServe with the data directory
./TTServe /home/rob/Documents/Safecast/TTServe

# In a separate terminal, send test data to TTServe
curl -X POST \
  -H "Content-Type: application/json" \
  -H "User-Agent: TTSERVE" \
  -d '{
    "Transport": "test-transport",
    "GatewayTime": "2023-04-01T12:00:00Z",
    "Payload": "test-payload"
  }' \
  http://localhost:8080/send

# After stopping TTServe, check the databases
duckdb /home/rob/Documents/Safecast/TTServe/api.duckdb -c "SELECT * FROM measurements;"
duckdb /home/rob/Documents/Safecast/TTServe/ingest.duckdb -c "SELECT * FROM measurements;"
```
