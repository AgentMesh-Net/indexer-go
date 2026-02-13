# Changelog

All notable changes to `indexer-go` are documented here.

## [v0.3.0] — 2025-xx-xx

### Added — Phase 6A: Protocol Security

- **Employer signature verification** (`POST /v1/tasks`):
  - Request now requires `"signature"` field (EIP-191 `personal_sign`)
  - Message: `keccak256(task_id)`
  - `secp256k1` recovery via `go-ethereum/crypto`
  - Recovered address must match `employer_address`; else `401 Unauthorized`
  - `employer_signature` stored in `tasks` table

- **Worker signature verification** (`POST /v1/tasks/{id}/accept`):
  - Request now requires `"signature"` field (EIP-191 `personal_sign`)
  - Message: `keccak256(task_id + accept_id)`
  - Recovered address must match `worker_address`; else `401 Unauthorized`
  - `worker_signature` stored in `accepts` table

- **Anti-replay protection** (`UNIQUE(task_id, worker_address)` index on `accepts`):
  - Same worker cannot accept the same task twice
  - Enforced at DB level (`migrations/003_onchain_sync.sql`)

- **`internal/ethutil` package**:
  - `VerifyPersonalSign(message, sig, address)` — full EIP-191 verification
  - `RecoverPersonalSign(msgHash, sig)` — recovers signer from prefixed hash
  - `Keccak256(data)` / `Keccak256Hex(data)` — legacy keccak256 helpers

### Added — Phase 6B: Onchain Event Watching

- **`internal/chain.Watcher`**:
  - Connects to configurable RPC endpoints (`INDEXER_RPC_URLS` env var, JSON map)
  - One goroutine per `SupportedChains` entry
  - Supports WebSocket subscription (primary) with HTTP polling fallback
  - Auto-reconnects on error (10s backoff)
  - Respects `min_confirmations` before processing events
  - Skips reorged (removed) logs
  - Errors logged; never panic; never block HTTP service

- **Event handlers**:
  - `Created(taskHash, employer, amount, deadline)` → `UpdateOnchainCreated`
  - `WorkerSet(taskHash, worker)` → `status=accepted_onchain`
  - `Released(taskHash)` → `status=released`, `released_at`
  - `Refunded(taskHash)` → `status=refunded`, `refunded_at`
  - Unknown `taskHash` → audit log `unexpected_onchain_create`

### Added — Schema

- `migrations/003_onchain_sync.sql`:
  - `tasks.employer_signature TEXT`
  - `tasks.onchain_created_at TIMESTAMPTZ`
  - `tasks.released_at TIMESTAMPTZ`
  - `tasks.refunded_at TIMESTAMPTZ`
  - `tasks.onchain_tx_hash TEXT`
  - `accepts.worker_signature TEXT`
  - `UNIQUE INDEX accepts(task_id, worker_address)`
  - Extended `status` CHECK: adds `accepted_onchain`

### Added — Config

- `INDEXER_RPC_URLS` — JSON map of `chain_id → rpc_url`
  - Example: `{"11155111":"wss://sepolia.infura.io/ws/v3/YOUR_KEY"}`

### Changed

- `tasks.status` CHECK constraint extended with `'accepted_onchain'`
- `TaskRepo` interface: added `GetTaskByHash`, `UpdateOnchainCreated`,
  `UpdateOnchainWorkerSet`, `UpdateOnchainReleased`, `UpdateOnchainRefunded`

---

## [v0.2.0] — Phase 5

- `GET /v1/health`, `GET /v1/meta` (Ed25519 signed canonical JSON)
- `POST /v1/tasks`, `GET /v1/tasks`, `GET /v1/tasks/{id}`, `POST /v1/tasks/{id}/accept`
- `migrations/002_tasks.sql`
- keccak256 `task_hash` validation

## [v0.1.0] — Initial

- Legacy object store: bids, accepts, artifacts
- `GET /v1/indexer/info`
- PostgreSQL via pgx/v5, Chi router
