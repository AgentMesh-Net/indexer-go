// Package chain provides onchain event watching for settlement contracts.
package chain

import (
	"context"
	"encoding/hex"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/AgentMesh-Net/indexer-go/internal/config"
	"github.com/AgentMesh-Net/indexer-go/internal/store"
)

// settlementABI is the minimal ABI fragment for the four events we watch.
// We declare them inline to avoid depending on an external ABI file.
const settlementABIJSON = `[
  {
    "anonymous": false,
    "inputs": [
      {"indexed": true,  "name": "taskHash",  "type": "bytes32"},
      {"indexed": true,  "name": "employer",  "type": "address"},
      {"indexed": false, "name": "amount",    "type": "uint256"},
      {"indexed": false, "name": "deadline",  "type": "uint64"}
    ],
    "name": "Created",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {"indexed": true, "name": "taskHash", "type": "bytes32"},
      {"indexed": true, "name": "worker",   "type": "address"}
    ],
    "name": "WorkerSet",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {"indexed": true, "name": "taskHash", "type": "bytes32"}
    ],
    "name": "Released",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {"indexed": true, "name": "taskHash", "type": "bytes32"}
    ],
    "name": "Refunded",
    "type": "event"
  }
]`

// Watcher monitors a single chain for settlement contract events and
// syncs task state in the database.
type Watcher struct {
	rpcURL           string
	contractAddr     common.Address
	minConfirmations int
	chainID          int
	taskRepo         store.TaskRepo
	parsedABI        abi.ABI
}

// NewWatcher creates a Watcher for the given chain config.
// rpcURL is the WebSocket or HTTP RPC endpoint for the chain.
func NewWatcher(rpcURL string, chainCfg config.ChainConfig, taskRepo store.TaskRepo) (*Watcher, error) {
	parsedABI, err := abi.JSON(strings.NewReader(settlementABIJSON))
	if err != nil {
		return nil, err
	}
	return &Watcher{
		rpcURL:           rpcURL,
		contractAddr:     common.HexToAddress(chainCfg.SettlementContract),
		minConfirmations: chainCfg.MinConfirmations,
		chainID:          chainCfg.ChainID,
		taskRepo:         taskRepo,
		parsedABI:        parsedABI,
	}, nil
}

// Run starts the watcher loop. It reconnects automatically on error and
// exits when ctx is cancelled. Errors are logged but never panic.
//
// Intended to be called as: go watcher.Run(ctx)
func (w *Watcher) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("[watcher chain=%d] context cancelled, stopping", w.chainID)
			return
		default:
		}

		if err := w.runOnce(ctx); err != nil {
			log.Printf("[watcher chain=%d] error: %v — reconnecting in 10s", w.chainID, err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}
}

// runOnce connects and subscribes; returns on error or context cancel.
func (w *Watcher) runOnce(ctx context.Context) error {
	client, err := ethclient.DialContext(ctx, w.rpcURL)
	if err != nil {
		return err
	}
	defer client.Close()

	query := ethereum.FilterQuery{
		Addresses: []common.Address{w.contractAddr},
	}

	logs := make(chan types.Log, 64)
	sub, err := client.SubscribeFilterLogs(ctx, query, logs)
	if err != nil {
		// Fallback: use polling via FilterLogs for HTTP endpoints
		return w.pollLogs(ctx, client)
	}
	defer sub.Unsubscribe()

	log.Printf("[watcher chain=%d] subscribed to %s", w.chainID, w.contractAddr.Hex())

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-sub.Err():
			return err
		case vLog := <-logs:
			w.handleLog(ctx, client, vLog)
		}
	}
}

// pollLogs is a fallback for HTTP RPC endpoints that don't support subscriptions.
// It polls every 12 seconds starting from the latest block.
func (w *Watcher) pollLogs(ctx context.Context, client *ethclient.Client) error {
	log.Printf("[watcher chain=%d] subscription not available, falling back to poll mode", w.chainID)

	latestBlock, err := client.BlockNumber(ctx)
	if err != nil {
		return err
	}
	fromBlock := new(big.Int).SetUint64(latestBlock)

	ticker := time.NewTicker(12 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		currentBlock, err := client.BlockNumber(ctx)
		if err != nil {
			return err
		}
		if currentBlock <= fromBlock.Uint64() {
			continue
		}

		toBlock := new(big.Int).SetUint64(currentBlock)
		query := ethereum.FilterQuery{
			FromBlock: fromBlock,
			ToBlock:   toBlock,
			Addresses: []common.Address{w.contractAddr},
		}

		fetched, err := client.FilterLogs(ctx, query)
		if err != nil {
			log.Printf("[watcher chain=%d] filter logs error: %v", w.chainID, err)
			continue
		}

		for _, vLog := range fetched {
			w.handleLog(ctx, client, vLog)
		}

		fromBlock = new(big.Int).SetUint64(currentBlock + 1)
	}
}

// handleLog dispatches a log to the appropriate event handler after
// confirming it has enough confirmations.
func (w *Watcher) handleLog(ctx context.Context, client *ethclient.Client, vLog types.Log) {
	// Skip removed (reorg) logs
	if vLog.Removed {
		log.Printf("[watcher chain=%d] skipping removed log tx=%s", w.chainID, vLog.TxHash.Hex())
		return
	}

	// Check confirmations
	if w.minConfirmations > 0 {
		currentBlock, err := client.BlockNumber(ctx)
		if err != nil {
			log.Printf("[watcher chain=%d] cannot get block number: %v", w.chainID, err)
			return
		}
		if currentBlock < vLog.BlockNumber+uint64(w.minConfirmations) {
			log.Printf("[watcher chain=%d] log block=%d current=%d minConf=%d — waiting",
				w.chainID, vLog.BlockNumber, currentBlock, w.minConfirmations)
			return
		}
	}

	if len(vLog.Topics) == 0 {
		return
	}

	eventID := vLog.Topics[0]

	switch eventID {
	case w.parsedABI.Events["Created"].ID:
		w.onCreated(ctx, vLog)
	case w.parsedABI.Events["WorkerSet"].ID:
		w.onWorkerSet(ctx, vLog)
	case w.parsedABI.Events["Released"].ID:
		w.onReleased(ctx, vLog)
	case w.parsedABI.Events["Refunded"].ID:
		w.onRefunded(ctx, vLog)
	default:
		// Unknown event — ignore
	}
}

// ── Event handlers ─────────────────────────────────────────────────────────────

// taskHashFromTopic decodes a bytes32 topic as a 0x-prefixed hex string.
func taskHashFromTopic(topic common.Hash) string {
	return "0x" + hex.EncodeToString(topic.Bytes())
}

func (w *Watcher) onCreated(ctx context.Context, vLog types.Log) {
	if len(vLog.Topics) < 2 {
		return
	}
	taskHash := taskHashFromTopic(vLog.Topics[1])
	txHash := vLog.TxHash.Hex()
	blockTime := time.Now() // approximate; use block timestamp in production if needed

	task, err := w.taskRepo.GetTaskByHash(ctx, taskHash)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[watcher chain=%d] Created event for unknown taskHash=%s tx=%s — audit: unexpected_onchain_create",
				w.chainID, taskHash, txHash)
		} else {
			log.Printf("[watcher chain=%d] GetTaskByHash error: %v", w.chainID, err)
		}
		return
	}

	if err := w.taskRepo.UpdateOnchainCreated(ctx, task.TaskID, txHash, blockTime); err != nil {
		log.Printf("[watcher chain=%d] UpdateOnchainCreated error: %v", w.chainID, err)
		return
	}
	log.Printf("[watcher chain=%d] Created: taskID=%s taskHash=%s tx=%s", w.chainID, task.TaskID, taskHash, txHash)
}

func (w *Watcher) onWorkerSet(ctx context.Context, vLog types.Log) {
	if len(vLog.Topics) < 3 {
		return
	}
	taskHash := taskHashFromTopic(vLog.Topics[1])
	workerAddr := common.BytesToAddress(vLog.Topics[2].Bytes()).Hex()
	txHash := vLog.TxHash.Hex()

	if err := w.taskRepo.UpdateOnchainWorkerSet(ctx, taskHash, strings.ToLower(workerAddr), txHash); err != nil {
		log.Printf("[watcher chain=%d] UpdateOnchainWorkerSet error: %v", w.chainID, err)
		return
	}
	log.Printf("[watcher chain=%d] WorkerSet: taskHash=%s worker=%s tx=%s", w.chainID, taskHash, workerAddr, txHash)
}

func (w *Watcher) onReleased(ctx context.Context, vLog types.Log) {
	if len(vLog.Topics) < 2 {
		return
	}
	taskHash := taskHashFromTopic(vLog.Topics[1])
	txHash := vLog.TxHash.Hex()
	at := time.Now()

	if err := w.taskRepo.UpdateOnchainReleased(ctx, taskHash, txHash, at); err != nil {
		log.Printf("[watcher chain=%d] UpdateOnchainReleased error: %v", w.chainID, err)
		return
	}
	log.Printf("[watcher chain=%d] Released: taskHash=%s tx=%s", w.chainID, taskHash, txHash)
}

func (w *Watcher) onRefunded(ctx context.Context, vLog types.Log) {
	if len(vLog.Topics) < 2 {
		return
	}
	taskHash := taskHashFromTopic(vLog.Topics[1])
	txHash := vLog.TxHash.Hex()
	at := time.Now()

	if err := w.taskRepo.UpdateOnchainRefunded(ctx, taskHash, txHash, at); err != nil {
		log.Printf("[watcher chain=%d] UpdateOnchainRefunded error: %v", w.chainID, err)
		return
	}
	log.Printf("[watcher chain=%d] Refunded: taskHash=%s tx=%s", w.chainID, taskHash, txHash)
}
