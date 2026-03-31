# Distributed Real-Time Drawing Board

A collaborative whiteboard backed by a Mini-RAFT consensus cluster.

## Architecture

```text
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
| replica1  | 9001 | RAFT node instance (shared image)  |
| replica2  | 9002 | RAFT node instance (shared image)  |
| replica3  | 9003 | RAFT node instance (shared image)  |

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
docker compose up --build
```

Open [http://localhost:3000](http://localhost:3000) in multiple tabs to test.

## RAFT Election Testing

### 1) Start cluster

```bash
docker compose up --build
```

In a second terminal, watch replica logs:

```bash
docker compose logs -f replica1 replica2 replica3
```

Expected within ~500-800ms:

- one node logs `starting election term=<n>`
- majority vote logs appear
- one node logs `became leader term=<n>`

### 2) Verify leader failover

Stop the current leader container (example shown for replica1):

```bash
docker compose stop replica1
```

Expected within ~1s:

- one remaining follower starts a new election
- one remaining node becomes leader with incremented term

### 3) Bring node back

```bash
docker compose start replica1
```

It should rejoin as follower once it receives heartbeat.

### 4) Optional API sanity checks

From host, you can probe each replica:

```bash
curl -X POST http://localhost:9001/heartbeat -H "Content-Type: application/json" -d '{"term":1,"leaderId":"replica1"}'
curl -X POST http://localhost:9002/request-vote -H "Content-Type: application/json" -d '{"term":2,"candidateId":"replica1"}'
```

### 5) Cleanup

```bash
docker compose down
```

## RAFT Log Replication Testing

### 1) Leader write + majority commit

Send a stroke to the current leader (or draw in the frontend):

```bash
curl -X POST http://localhost:9001/append-entry -H "Content-Type: application/json" -d '{"type":"stroke","points":[{"x":10,"y":10},{"x":20,"y":20}],"color":"#ff0000","width":3}'
```

Expected in leader logs:

- `appended entry index=<n>`
- `replicated entry to <replica>`
- `committed index=<n>`

### 2) Quorum failure behavior

Stop two replicas so only one remains alive:

```bash
docker compose stop replica2 replica3
```

Now writes should fail with:

- HTTP `503`
- body: `{"error":"replication_failed"}`

Bring one replica back to restore quorum:

```bash
docker compose start replica2
```

Writes should succeed again.

## Team Responsibilities

| Person | Area                  |
|--------|-----------------------|
| 1      | /frontend             |
| 2      | /gateway              |
| 3 & 4  | /replica (RAFT)       |
