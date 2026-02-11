# indexer-go

Reference Indexer implementation for [AgentMesh-Net](https://github.com/AgentMesh-Net/spec) (Go + Postgres).

Stores and serves signed protocol objects (Task, Bid, Accept, Artifact) with ed25519 signature verification per the AgentMesh-Net spec `v0.1`.

## Quickstart

```bash
docker compose up --build
```

The indexer will be available at `http://localhost:8080`.

### Submit a task

```bash
curl -s -X POST http://localhost:8080/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "object_type": "task",
    "object_version": "0.1",
    "object_id": "01J0000000000000000000TEST",
    "created_at": "2025-01-01T00:00:00Z",
    "payload": {"description": "a test", "title": "test task"},
    "signature": "5vNLiFEPahJCdqvg8w7cRZhdMmEBh4OHfF00LV0xGCmU7x5Y4E8YklW+SjYXeCVRC0SxcegUllxfL6GLQA57Bg==",
    "signer": {"algo": "ed25519", "pubkey": "5pCB+DwMAPVHm8aabzPlBWx3kBVX94EOijtjcU4/Gzc="}
  }' | jq .
```

### List tasks

```bash
curl -s http://localhost:8080/v1/tasks | jq .
```

### Submit a bid

```bash
curl -s -X POST http://localhost:8080/v1/bids \
  -H "Content-Type: application/json" \
  -d '{
    "object_type": "bid",
    "object_version": "0.1",
    "object_id": "01J00000000000000000000BID",
    "created_at": "2025-01-01T00:00:30Z",
    "payload": {"price": "100", "task_id": "01J0000000000000000000TEST"},
    "signature": "xQQlVa8Vww22hkMfzOQ+4puR4T/FJGclVfmLJe4xoomyB4Q3oRyzAuuJB7nvYl75C47gBW4vH2QnDMIrGv0LDQ==",
    "signer": {"algo": "ed25519", "pubkey": "5pCB+DwMAPVHm8aabzPlBWx3kBVX94EOijtjcU4/Gzc="}
  }' | jq .
```

### Submit an accept

```bash
curl -s -X POST http://localhost:8080/v1/accepts \
  -H "Content-Type: application/json" \
  -d '{
    "object_type": "accept",
    "object_version": "0.1",
    "object_id": "01J0000000000000000000ACPT",
    "created_at": "2025-01-01T00:01:00Z",
    "payload": {"task_id": "01J0000000000000000000TEST"},
    "signature": "NquujNYmexNWvu8m0X0UN5PngabR3ZMQ1PeVe0wIPa+ePFsAsQoRyYWfJ7dolKvnmBiV0d5EN6aYPOCEeSHNDA==",
    "signer": {"algo": "ed25519", "pubkey": "5pCB+DwMAPVHm8aabzPlBWx3kBVX94EOijtjcU4/Gzc="}
  }' | jq .
```

### Indexer info

```bash
curl -s http://localhost:8080/v1/indexer/info | jq .
```

### Pagination

```bash
curl -s "http://localhost:8080/v1/tasks?limit=10" | jq .
# Use next_cursor from response for next page:
curl -s "http://localhost:8080/v1/tasks?limit=10&cursor=<next_cursor>" | jq .
```

## v0.1 Limitations

- No task execution or sandboxing
- No escrow, payment, or settlement
- No artifact correctness evaluation
- No private key management

The indexer verifies signatures and enforces accept validation rules only.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `AMN_DB_DSN` / `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/indexer?sslmode=disable` | Postgres connection string |
| `AMN_HTTP_ADDR` | `:8080` | HTTP listen address |
| `AMN_MAX_BODY_BYTES` | `2097152` (2MB) | Max request body size |

## Development

```bash
# Run Postgres locally
docker compose up postgres -d

# Run the indexer
export AMN_DB_DSN="postgres://postgres:postgres@localhost:5432/indexer?sslmode=disable"
go run ./cmd/indexer

# Run tests
go test ./...
```

## Spec

See [AgentMesh-Net/spec tag spec-v0.1.0](https://github.com/AgentMesh-Net/spec/tree/spec-v0.1.0).
