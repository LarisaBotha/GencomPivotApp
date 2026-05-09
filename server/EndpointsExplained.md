# API Documentation

The server base url: https://pivot-api-of3d.onrender.com

## Ping

**Access URL:** https://pivot-api-of3d.onrender.com/api/ping

**Purpose / Use:** Check if the server is alive and responding.

**Request Type / Method:** GET

**Arguments / Body:** None

**Response:** Returns the current server time as text.

**Example:** 

Request:
```
GET /api/ping HTTP/1.1
Host: pivot-api-of3d.onrender.com
```

Response:
```
HTTP/1.1 200 OK
Content-Type: text/plain; charset=utf-8
Content-Length: 32
Connection: close

PONG - 2026-03-30T15:00:00Z
```

## Sync Pivot

**Access URL:** https://pivot-api-of3d.onrender.com/api/sync_pivot

**Purpose / Use:** Updates the database with the pivot's current status, so that the client interfaces can fetch the latest data.

**Request Type / Method:** POST

**Arguments / Body:**

- imei (String) - Pivot identifier (Required) 
- position_deg (Float) - Current pivot angle (Optional)
- speed_pct (Float) - Current speed percentage (Optional)
- direction (String) - 'forward' OR 'reverse' (Optional)
- status (String) - 'running' OR 'stopped' OR 'error' OR 'offline' (Optional)
- battery_pct (Float) - Battery level (Optional)
- wet (Bool) (Optional)
- pressure (Float) - Current water pressure (Optional)

**Response:** A JSON array of pending commands. Only the most recent version of each command type is returned to avoid redundancy. These commands will repeat in every sync until the pivot confirms receipt via /api/ack_commands.

**Example:** 

Request: 
```
POST /api/sync_pivot HTTP/1.1
Host: pivot-api-of3d.onrender.com
Content-Type: application/json

{
  "imei": "864201040000001",
  "position_deg": 180.5,
  "speed_pct": 50.65,
  "status": "Running",
  "wet": true
}
```

Response: 
```
HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 124
Connection: close

[
  {
    "id": 101,
    "command": "Update",
    "payload": "[{\"start\":0,\"end\":180,\"value\":10.0,\"unit\":\"mm\"},{\"start\":180,\"end\":0,\"value\":5.0,\"unit\":\"mm\"}]"
  },
  {
    "id": 102,
    "command": "Set_Control",
    "payload": "{\"direction\":\"reverse\",\"wet\":true}"
  },
  {
    "id": 103,
    "command": "Stop",
    "payload": null
  }
]
```

# Get Online Users

**Access URL:** https://pivot-api-of3d.onrender.com/api/get_subscriber_count

**Purpose / Use:** Returns the number of active SSE (Server-Sent Events) connections for a specific pivot. This is useful for monitoring how many clients are currently watching a pivot's status in real-time.

**Request Type / Method:** GET

**Arguments / Body:**

- imei (String) - Pivot identifier (Required) 

**Response:** A JSON object containing the current count of active subscribers.

**Example:**

Request:
```
GET /api/subscriber_count?imei=864201040000001 HTTP/1.1
Host: pivot-api-of3d.onrender.com
```

Response:
```
HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 62
Connection: close

{
  "count": 3
}
```

# Acknowledge / Confirm Commands

**Access URL:** https://pivot-api-of3d.onrender.com/api/ack_commands

**Purpose / Use:** Marks specific commands as acknowledged by the pivot device. This prevents the same commands from being sent repeatedly in api/sync_pivot responses.

**Request Type / Method:** POST

**Arguments / Body:**

- ids (Array of Integers) - A list of unique command IDs to acknowledge (Required)

**Response:** Returns a 200 OK status code upon successful update.

**Example:**

Request:
```
POST /api/ack_commands HTTP/1.1
Host: pivot-api-of3d.onrender.com
Content-Type: application/json

{
  "ids": [1, 2, 5]
}
```

Response:
```
HTTP/1.1 200 OK
Content-Length: 0
Connection: close
```