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
- position_deg (Float) - Current pivot angle
- speed_pct (Float) - Current speed percentage
- direction (String) - 'forward' OR 'reverse'
- status (String) - 'running' OR 'stopped' OR 'error' OR 'offline'
- battery_pct (Float) - Battery level
- wet (Bool)

**Response:** A JSON array of newly acknowledged command objects.

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
Content-Length: 54
Connection: close

[{"id": 102, "command": "Stop", "payload": null}]
```

