# Distributed Real-Time Drawing Board

A collaborative whiteboard backed by a Mini-RAFT consensus cluster.

## Architecture

```
Browser(s)
   │ WebSocket
   ▼
 Gateway  (:8080)
   │ HTTP
   ▼
Leader Replica ──AppendEntries──▶ Replica 2
(replica1/2/3)                  ▶ Replica 3
```

## Services

| Service   | Port | Description                        |
|-----------|------|------------------------------------|
| frontend  | 3000 | Static canvas UI                   |
| gateway   | 8080 | WebSocket server + leader routing  |
| replica1  | 9001 | RAFT node                          |
| replica2  | 9002 | RAFT node                          |
| replica3  | 9003 | RAFT node                          |

## Stroke JSON Schema

All strokes use this format :

```json
{
  "type": "stroke",
  "points": [{"x": 100, "y": 200}, {"x": 105, "y": 210}],
  "color": "#ff0000",
  "width": 4
}
```

For clear events:
```json
{ "type": "clear" }
```

## Running

```bash
docker-compose up --build
```

Open http://localhost:3000 in multiple tabs to test.

## Team Responsibilities

| Person | Area                  |
|--------|-----------------------|
| 1      | /frontend             |
| 2      | /gateway              |
| 3 & 4  | /replica1/2/3 (RAFT)  |
