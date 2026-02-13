package api

// handlers_tasks_v2.go implements the Phase 5 structured task endpoints:
//   POST /v1/tasks
//   GET  /v1/tasks
//   GET  /v1/tasks/{taskID}
//   POST /v1/tasks/{taskID}/accept

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/sha3"

	"github.com/AgentMesh-Net/indexer-go/internal/ethutil"
	"github.com/AgentMesh-Net/indexer-go/internal/store"
	"github.com/AgentMesh-Net/indexer-go/internal/util"
)

var reHexAddr = regexp.MustCompile(`(?i)^0x[0-9a-fA-F]{40}$`)
var reHexHash = regexp.MustCompile(`(?i)^0x[0-9a-fA-F]{64}$`)
var reHexSig  = regexp.MustCompile(`(?i)^0x[0-9a-fA-F]{130}$`) // 65 bytes = 130 hex chars

// ── Request types ──────────────────────────────────────────────────────────────

type createTaskReq struct {
	TaskID          string         `json:"task_id"`
	Title           string         `json:"title"`
	ChainID         int            `json:"chain_id"`
	AmountWei       string         `json:"amount_wei"`
	DeadlineUnix    int64          `json:"deadline_unix"`
	EmployerAddress string         `json:"employer_address"`
	TaskHash        string         `json:"task_hash"`
	EscrowAddress   string         `json:"escrow_address"`
	Signature       string         `json:"signature"`   // required: EIP-191 personal_sign over keccak256(task_id)
	Payload         map[string]any `json:"payload"`     // optional extra metadata
}

type acceptTaskReq struct {
	AcceptID      string `json:"accept_id"`
	WorkerAddress string `json:"worker_address"`
	Signature     string `json:"signature"` // required: EIP-191 personal_sign over keccak256(task_id + accept_id)
}

// ── keccak256 helper ───────────────────────────────────────────────────────────

func keccak256Hex(data []byte) string {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return "0x" + hex.EncodeToString(h.Sum(nil))
}

// ── POST /v1/tasks ─────────────────────────────────────────────────────────────

func (h *handlers) PostTask(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, h.maxBody+1))
	if err != nil || int64(len(body)) > h.maxBody {
		util.WriteError(w, http.StatusBadRequest, "invalid_request", "body read error or too large")
		return
	}

	var req createTaskReq
	if err := json.Unmarshal(body, &req); err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid_request", "invalid JSON: "+err.Error())
		return
	}

	// Validate required fields
	if req.TaskID == "" {
		util.WriteError(w, http.StatusBadRequest, "invalid_request", "task_id is required")
		return
	}
	if req.ChainID == 0 {
		util.WriteError(w, http.StatusBadRequest, "invalid_request", "chain_id is required")
		return
	}
	if !reHexAddr.MatchString(req.EmployerAddress) {
		util.WriteError(w, http.StatusBadRequest, "invalid_request", "employer_address must be 0x + 40 hex chars")
		return
	}
	if !reHexHash.MatchString(req.TaskHash) {
		util.WriteError(w, http.StatusBadRequest, "invalid_request", "task_hash must be 0x + 64 hex chars")
		return
	}

	// Validate amount_wei > 0
	amtStr := strings.TrimSpace(req.AmountWei)
	amt, ok := new(big.Int).SetString(amtStr, 10)
	if !ok || amt.Sign() <= 0 {
		util.WriteError(w, http.StatusBadRequest, "invalid_request", "amount_wei must be a positive integer string")
		return
	}

	// Validate deadline
	if req.DeadlineUnix <= 0 || req.DeadlineUnix > (1<<62) {
		util.WriteError(w, http.StatusBadRequest, "invalid_request", "deadline_unix out of valid range")
		return
	}

	// Verify task_hash == keccak256(utf8(task_id))
	expected := keccak256Hex([]byte(req.TaskID))
	if !strings.EqualFold(req.TaskHash, expected) {
		util.WriteError(w, http.StatusBadRequest, "invalid_request",
			fmt.Sprintf("task_hash mismatch: expected %s, got %s", expected, req.TaskHash))
		return
	}

	// A1: Employer signature verification (EIP-191 personal_sign over keccak256(task_id))
	if req.Signature == "" {
		util.WriteError(w, http.StatusUnauthorized, "unauthorized", "signature is required")
		return
	}
	if !reHexSig.MatchString(req.Signature) {
		util.WriteError(w, http.StatusBadRequest, "invalid_request", "signature must be 0x + 130 hex chars")
		return
	}
	if err := ethutil.VerifyPersonalSign([]byte(req.TaskID), req.Signature, req.EmployerAddress); err != nil {
		if errors.Is(err, ethutil.ErrSignerMismatch) || errors.Is(err, ethutil.ErrInvalidSignature) {
			util.WriteError(w, http.StatusUnauthorized, "unauthorized",
				"signature verification failed: signer does not match employer_address")
			return
		}
		util.WriteError(w, http.StatusBadRequest, "invalid_request", "signature error: "+err.Error())
		return
	}

	// Validate chain_id is supported
	escrow := req.EscrowAddress
	chainOK := false
	for _, c := range h.cfg.SupportedChains {
		if c.ChainID == req.ChainID {
			chainOK = true
			if escrow == "" {
				escrow = c.SettlementContract
			}
			break
		}
	}
	if !chainOK {
		supported := make([]string, len(h.cfg.SupportedChains))
		for i, c := range h.cfg.SupportedChains {
			supported[i] = strconv.Itoa(c.ChainID)
		}
		util.WriteError(w, http.StatusBadRequest, "invalid_request",
			fmt.Sprintf("chain_id %d not supported (supported: %s)", req.ChainID, strings.Join(supported, ",")))
		return
	}

	task := &store.Task{
		TaskID:            req.TaskID,
		TaskHash:          strings.ToLower(req.TaskHash),
		ChainID:           req.ChainID,
		EscrowAddress:     escrow,
		EmployerAddress:   strings.ToLower(req.EmployerAddress),
		EmployerSignature: strings.ToLower(req.Signature),
		AmountWei:         amtStr,
		DeadlineUnix:      req.DeadlineUnix,
		Title:             req.Title,
		Status:            store.TaskStatusCreated,
		IndexerFeeBPS:     h.cfg.FeeBPS,
	}

	if err := h.taskRepo.InsertTask(r.Context(), task); err != nil {
		if errors.Is(err, store.ErrConflict) {
			util.WriteError(w, http.StatusConflict, "conflict", "task_id already exists")
			return
		}
		util.WriteError(w, http.StatusInternalServerError, "internal", "failed to store task")
		return
	}

	util.WriteJSON(w, http.StatusCreated, map[string]any{
		"task_id":          task.TaskID,
		"task_hash":        task.TaskHash,
		"status":           task.Status,
		"chain_id":         task.ChainID,
		"escrow_address":   task.EscrowAddress,
		"employer_address": task.EmployerAddress,
		"amount_wei":       task.AmountWei,
		"deadline_unix":    task.DeadlineUnix,
		"indexer_fee_bps":  task.IndexerFeeBPS,
	})
}

// ── GET /v1/tasks ──────────────────────────────────────────────────────────────

func (h *handlers) ListTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	chainID := 0
	if s := q.Get("chain_id"); s != "" {
		chainID, _ = strconv.Atoi(s)
	}
	status := q.Get("status")
	limit := 50
	offset := 0
	if s := q.Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if s := q.Get("offset"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			offset = n
		}
	}

	tasks, err := h.taskRepo.ListTasks(r.Context(), chainID, status, limit, offset)
	if err != nil {
		util.WriteError(w, http.StatusInternalServerError, "internal", "failed to list tasks")
		return
	}

	items := make([]map[string]any, 0, len(tasks))
	for _, t := range tasks {
		items = append(items, taskToMap(t))
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ── GET /v1/tasks/{taskID} ─────────────────────────────────────────────────────

func (h *handlers) GetTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	task, err := h.taskRepo.GetTask(r.Context(), taskID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			util.WriteError(w, http.StatusNotFound, "not_found", "task not found")
			return
		}
		util.WriteError(w, http.StatusInternalServerError, "internal", "failed to get task")
		return
	}
	util.WriteJSON(w, http.StatusOK, taskToMap(task))
}

// ── POST /v1/tasks/{taskID}/accept ────────────────────────────────────────────

func (h *handlers) PostTaskAccept(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")

	body, err := io.ReadAll(io.LimitReader(r.Body, h.maxBody+1))
	if err != nil || int64(len(body)) > h.maxBody {
		util.WriteError(w, http.StatusBadRequest, "invalid_request", "body read error or too large")
		return
	}

	var req acceptTaskReq
	if err := json.Unmarshal(body, &req); err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid_request", "invalid JSON: "+err.Error())
		return
	}

	if req.AcceptID == "" {
		util.WriteError(w, http.StatusBadRequest, "invalid_request", "accept_id is required")
		return
	}
	if !reHexAddr.MatchString(req.WorkerAddress) {
		util.WriteError(w, http.StatusBadRequest, "invalid_request", "worker_address must be 0x + 40 hex chars")
		return
	}

	// A2: Worker signature verification (EIP-191 personal_sign over keccak256(task_id + accept_id))
	if req.Signature == "" {
		util.WriteError(w, http.StatusUnauthorized, "unauthorized", "signature is required")
		return
	}
	if !reHexSig.MatchString(req.Signature) {
		util.WriteError(w, http.StatusBadRequest, "invalid_request", "signature must be 0x + 130 hex chars")
		return
	}
	workerSigMsg := []byte(taskID + req.AcceptID)
	if err := ethutil.VerifyPersonalSign(workerSigMsg, req.Signature, req.WorkerAddress); err != nil {
		if errors.Is(err, ethutil.ErrSignerMismatch) || errors.Is(err, ethutil.ErrInvalidSignature) {
			util.WriteError(w, http.StatusUnauthorized, "unauthorized",
				"signature verification failed: signer does not match worker_address")
			return
		}
		util.WriteError(w, http.StatusBadRequest, "invalid_request", "signature error: "+err.Error())
		return
	}

	// Verify task exists and is in created state
	task, err := h.taskRepo.GetTask(r.Context(), taskID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			util.WriteError(w, http.StatusNotFound, "not_found", "task not found")
			return
		}
		util.WriteError(w, http.StatusInternalServerError, "internal", "failed to get task")
		return
	}
	if task.Status != store.TaskStatusCreated {
		util.WriteError(w, http.StatusConflict, "conflict",
			fmt.Sprintf("task is not in 'created' state (current: %s)", task.Status))
		return
	}

	accept := &store.Accept{
		AcceptID:        req.AcceptID,
		TaskID:          taskID,
		WorkerAddress:   strings.ToLower(req.WorkerAddress),
		WorkerSignature: strings.ToLower(req.Signature),
	}
	if err := h.taskRepo.InsertAccept(r.Context(), accept); err != nil {
		if errors.Is(err, store.ErrConflict) {
			util.WriteError(w, http.StatusConflict, "conflict", "accept_id already exists")
			return
		}
		util.WriteError(w, http.StatusInternalServerError, "internal", "failed to store accept")
		return
	}

	if err := h.taskRepo.UpdateTaskWorker(r.Context(), taskID, strings.ToLower(req.WorkerAddress), store.TaskStatusAccepted); err != nil {
		util.WriteError(w, http.StatusInternalServerError, "internal", "failed to update task")
		return
	}

	util.WriteJSON(w, http.StatusCreated, map[string]any{
		"task_id":        taskID,
		"accept_id":      req.AcceptID,
		"status":         "accepted",
		"worker_address": strings.ToLower(req.WorkerAddress),
	})
}

// ── helper ─────────────────────────────────────────────────────────────────────

func taskToMap(t *store.Task) map[string]any {
	m := map[string]any{
		"task_id":          t.TaskID,
		"task_hash":        t.TaskHash,
		"status":           t.Status,
		"chain_id":         t.ChainID,
		"escrow_address":   t.EscrowAddress,
		"employer_address": t.EmployerAddress,
		"worker_address":   t.WorkerAddress,
		"amount_wei":       t.AmountWei,
		"deadline_unix":    t.DeadlineUnix,
		"title":            t.Title,
		"indexer_fee_bps":  t.IndexerFeeBPS,
		"created_at":       t.CreatedAt,
		"updated_at":       t.UpdatedAt,
	}
	if t.OnchainCreatedAt != nil {
		m["onchain_created_at"] = t.OnchainCreatedAt
	}
	if t.ReleasedAt != nil {
		m["released_at"] = t.ReleasedAt
	}
	if t.RefundedAt != nil {
		m["refunded_at"] = t.RefundedAt
	}
	if t.OnchainTxHash != "" {
		m["onchain_tx_hash"] = t.OnchainTxHash
	}
	return m
}
