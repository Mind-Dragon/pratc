package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/logger"
	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/jeffersonnunn/pratc/internal/workqueue"
)

// registerQueueRoutes mounts queue API routes on both legacy /queue/* and API /api/queue/* paths.
func registerQueueRoutes(mux *http.ServeMux, queue *workqueue.Queue, defaultRepo string) {
	for _, prefix := range []string{"/queue", "/api/queue"} {
		mux.HandleFunc(prefix+"/claim", func(w http.ResponseWriter, r *http.Request) {
			handleQueueClaim(w, r, queue, defaultRepo)
		})
		mux.HandleFunc(prefix+"/release", func(w http.ResponseWriter, r *http.Request) {
			handleQueueRelease(w, r, queue)
		})
		mux.HandleFunc(prefix+"/heartbeat", func(w http.ResponseWriter, r *http.Request) {
			handleQueueHeartbeat(w, r, queue)
		})
		mux.HandleFunc(prefix+"/status", func(w http.ResponseWriter, r *http.Request) {
			handleQueueStatus(w, r, queue, defaultRepo)
		})
		mux.HandleFunc(prefix+"/proof", func(w http.ResponseWriter, r *http.Request) {
			handleQueueProof(w, r, queue)
		})
	}
}

// claimRequest is the JSON body for POST /queue/claim
type claimRequest struct {
	WorkerID   string `json:"worker_id"`
	TTLSeconds int    `json:"ttl_seconds,omitempty"`  // optional, default 300
	ID         string `json:"id,omitempty"`           // work item ID
	WorkItemID string `json:"work_item_id,omitempty"` // alias for ID
	Repo       string `json:"repo,omitempty"`         // optional, if ID not provided
	Lane       string `json:"lane,omitempty"`         // optional, filter for next claimable
}

// claimResponse is the JSON response for claim
type claimResponse struct {
	Repo        string               `json:"repo"`
	GeneratedAt string               `json:"generatedAt"`
	WorkItem    types.ActionWorkItem `json:"work_item"`
}

// releaseRequest is the JSON body for POST /queue/release
type releaseRequest struct {
	WorkerID   string `json:"worker_id"`
	ID         string `json:"id,omitempty"`
	WorkItemID string `json:"work_item_id,omitempty"`
}

// releaseResponse is the JSON response for release
type releaseResponse struct {
	Repo        string               `json:"repo"`
	GeneratedAt string               `json:"generatedAt"`
	WorkItem    types.ActionWorkItem `json:"work_item"`
}

// heartbeatRequest is the JSON body for POST /queue/heartbeat
type heartbeatRequest struct {
	WorkerID   string `json:"worker_id"`
	TTLSeconds int    `json:"ttl_seconds,omitempty"` // optional, default 300
	ID         string `json:"id,omitempty"`
	WorkItemID string `json:"work_item_id,omitempty"`
}

// heartbeatResponse is the JSON response for heartbeat
type heartbeatResponse struct {
	Repo        string               `json:"repo"`
	GeneratedAt string               `json:"generatedAt"`
	WorkItem    types.ActionWorkItem `json:"work_item"`
}

// statusResponse is the JSON response for GET /queue/status
type statusResponse struct {
	Repo        string                         `json:"repo"`
	GeneratedAt string                         `json:"generatedAt"`
	Counts      map[types.ActionLane]laneCount `json:"counts"`
}

type laneCount struct {
	Claimable int `json:"claimable"`
	Claimed   int `json:"claimed"`
}

// proofAttachRequest is the JSON body for POST /queue/proof
type proofAttachRequest struct {
	WorkItemID  string             `json:"work_item_id"`
	WorkerID    string             `json:"worker_id"`
	ProofBundle types.ProofBundle  `json:"proof_bundle"`
}

// proofAttachResponse is the JSON response for proof attach
type proofAttachResponse struct {
	Repo        string               `json:"repo"`
	GeneratedAt string               `json:"generatedAt"`
	ProofBundle types.ProofBundle    `json:"proof_bundle"`
	WorkItem    types.ActionWorkItem `json:"work_item,omitempty"`
}

// handleQueueClaim handles POST /queue/claim
func handleQueueClaim(w http.ResponseWriter, r *http.Request, queue *workqueue.Queue, defaultRepo string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	log := logger.FromContext(r.Context())

	var req claimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeHTTPError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}
	defer r.Body.Close()

	workerID := strings.TrimSpace(req.WorkerID)
	if workerID == "" {
		writeHTTPError(w, r, http.StatusBadRequest, "worker_id is required")
		return
	}

	ttl := 300 * time.Second
	if req.TTLSeconds > 0 {
		ttl = time.Duration(req.TTLSeconds) * time.Second
	}

	// Determine repo
	repo := strings.TrimSpace(req.Repo)
	if repo == "" {
		repo = defaultRepo
	}
	if repo == "" {
		writeHTTPError(w, r, http.StatusBadRequest, "repo is required")
		return
	}

	var workItem types.ActionWorkItem
	var err error

	// If ID provided, claim that specific item
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = strings.TrimSpace(req.WorkItemID)
	}
	if id != "" {
		workItem, err = queue.Claim(r.Context(), id, workerID, ttl)
		if err != nil {
			log.Error("queue claim failed", "error", err.Error(), "id", id, "worker", workerID)
			// Map error to appropriate HTTP status
			if strings.Contains(err.Error(), "not claimable") {
				writeHTTPError(w, r, http.StatusConflict, "work item not claimable")
			} else if strings.Contains(err.Error(), "not found") {
				writeHTTPError(w, r, http.StatusNotFound, "work item not found")
			} else {
				writeHTTPError(w, r, http.StatusInternalServerError, sanitizedError(err))
			}
			return
		}
	} else {
		// No ID provided, claim next claimable item based on repo/lane filters
		lane := types.ActionLane(strings.TrimSpace(req.Lane))
		items, err := queue.GetClaimable(r.Context(), repo, lane, 1)
		if err != nil {
			log.Error("get claimable failed", "error", err.Error(), "repo", repo, "lane", lane)
			writeHTTPError(w, r, http.StatusInternalServerError, sanitizedError(err))
			return
		}
		if len(items) == 0 {
			writeHTTPError(w, r, http.StatusNotFound, "no claimable work items")
			return
		}
		workItem, err = queue.Claim(r.Context(), items[0].ID, workerID, ttl)
		if err != nil {
			log.Error("queue claim failed", "error", err.Error(), "id", items[0].ID, "worker", workerID)
			if strings.Contains(err.Error(), "not claimable") {
				writeHTTPError(w, r, http.StatusConflict, "work item not claimable")
			} else {
				writeHTTPError(w, r, http.StatusInternalServerError, sanitizedError(err))
			}
			return
		}
	}

	writeHTTPJSON(w, http.StatusOK, claimResponse{
		Repo:        repo,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		WorkItem:    workItem,
	})
}

// handleQueueRelease handles POST /queue/release
func handleQueueRelease(w http.ResponseWriter, r *http.Request, queue *workqueue.Queue) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	log := logger.FromContext(r.Context())

	var req releaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeHTTPError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}
	defer r.Body.Close()

	workerID := strings.TrimSpace(req.WorkerID)
	if workerID == "" {
		writeHTTPError(w, r, http.StatusBadRequest, "worker_id is required")
		return
	}

	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = strings.TrimSpace(req.WorkItemID)
	}
	if id == "" {
		writeHTTPError(w, r, http.StatusBadRequest, "id or work_item_id is required")
		return
	}

	workItem, err := queue.Release(r.Context(), id, workerID)
	if err != nil {
		log.Error("queue release failed", "error", err.Error(), "id", id, "worker", workerID)
		if strings.Contains(err.Error(), "not claimed by") {
			writeHTTPError(w, r, http.StatusForbidden, "work item not claimed by this worker")
		} else if strings.Contains(err.Error(), "not found") {
			writeHTTPError(w, r, http.StatusNotFound, "work item not found")
		} else {
			writeHTTPError(w, r, http.StatusInternalServerError, sanitizedError(err))
		}
		return
	}

	writeHTTPJSON(w, http.StatusOK, releaseResponse{
		Repo:        "", // not known from request
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		WorkItem:    workItem,
	})
}

// handleQueueHeartbeat handles POST /queue/heartbeat
func handleQueueHeartbeat(w http.ResponseWriter, r *http.Request, queue *workqueue.Queue) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	log := logger.FromContext(r.Context())

	var req heartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeHTTPError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}
	defer r.Body.Close()

	workerID := strings.TrimSpace(req.WorkerID)
	if workerID == "" {
		writeHTTPError(w, r, http.StatusBadRequest, "worker_id is required")
		return
	}

	ttl := 300 * time.Second
	if req.TTLSeconds > 0 {
		ttl = time.Duration(req.TTLSeconds) * time.Second
	}

	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = strings.TrimSpace(req.WorkItemID)
	}
	if id == "" {
		writeHTTPError(w, r, http.StatusBadRequest, "id or work_item_id is required")
		return
	}

	workItem, err := queue.Heartbeat(r.Context(), id, workerID, ttl)
	if err != nil {
		log.Error("queue heartbeat failed", "error", err.Error(), "id", id, "worker", workerID)
		if strings.Contains(err.Error(), "not claimed by") {
			writeHTTPError(w, r, http.StatusForbidden, "work item not claimed by this worker")
		} else if strings.Contains(err.Error(), "not found") {
			writeHTTPError(w, r, http.StatusNotFound, "work item not found")
		} else {
			writeHTTPError(w, r, http.StatusInternalServerError, sanitizedError(err))
		}
		return
	}

	writeHTTPJSON(w, http.StatusOK, heartbeatResponse{
		Repo:        "", // not known from request
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		WorkItem:    workItem,
	})
}

// handleQueueStatus handles GET /queue/status
func handleQueueStatus(w http.ResponseWriter, r *http.Request, queue *workqueue.Queue, defaultRepo string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	log := logger.FromContext(r.Context())

	// Determine repo from query param or default
	repo := repoFromQuery(r, defaultRepo)
	if repo == "" {
		writeHTTPError(w, r, http.StatusBadRequest, "repo is required")
		return
	}

	// Optional lane filter (not used for counts, but could be used for filtering)
	laneFilter := strings.TrimSpace(r.URL.Query().Get("lane"))
	var lane types.ActionLane
	if laneFilter != "" {
		lane = types.ActionLane(laneFilter)
	}

	if _, err := queue.ExpireLeases(r.Context()); err != nil {
		log.Error("expire queue leases failed", "error", err.Error(), "repo", repo)
		writeHTTPError(w, r, http.StatusInternalServerError, sanitizedError(err))
		return
	}

	// Get lane counts
	counts, err := queue.GetLaneCounts(r.Context(), repo)
	if err != nil {
		log.Error("get lane counts failed", "error", err.Error(), "repo", repo)
		writeHTTPError(w, r, http.StatusInternalServerError, sanitizedError(err))
		return
	}

	// Convert to response format
	respCounts := make(map[types.ActionLane]laneCount)
	for l, c := range counts {
		// If lane filter specified, only include that lane
		if laneFilter != "" && l != lane {
			continue
		}
		respCounts[l] = laneCount{
			Claimable: c.Claimable,
			Claimed:   c.Claimed,
		}
	}

	writeHTTPJSON(w, http.StatusOK, statusResponse{
		Repo:        repo,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Counts:      respCounts,
	})
}

// handleQueueProof handles POST /queue/proof
func handleQueueProof(w http.ResponseWriter, r *http.Request, queue *workqueue.Queue) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	log := logger.FromContext(r.Context())

	var req proofAttachRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeHTTPError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}
	defer r.Body.Close()

	workerID := strings.TrimSpace(req.WorkerID)
	if workerID == "" {
		writeHTTPError(w, r, http.StatusBadRequest, "worker_id is required")
		return
	}
	workItemID := strings.TrimSpace(req.WorkItemID)
	if workItemID == "" {
		writeHTTPError(w, r, http.StatusBadRequest, "work_item_id is required")
		return
	}
	if req.ProofBundle.ID == "" {
		writeHTTPError(w, r, http.StatusBadRequest, "proof_bundle.id is required")
		return
	}

	// Attach proof bundle
	proof, err := queue.AttachProof(r.Context(), workItemID, workerID, req.ProofBundle)
	if err != nil {
		log.Error("queue proof attach failed", "error", err.Error(), "id", workItemID, "worker", workerID)
		// Map error to appropriate HTTP status
		if strings.Contains(err.Error(), "not claimed by") {
			writeHTTPError(w, r, http.StatusForbidden, "work item not claimed by this worker")
		} else if strings.Contains(err.Error(), "not claimed") {
			writeHTTPError(w, r, http.StatusConflict, "work item not claimed")
		} else if strings.Contains(err.Error(), "not found") {
			writeHTTPError(w, r, http.StatusNotFound, "work item not found")
		} else if strings.Contains(err.Error(), "has expired") {
			writeHTTPError(w, r, http.StatusConflict, "lease expired")
		} else if strings.Contains(err.Error(), "proof bundle validation failed") {
			writeHTTPError(w, r, http.StatusBadRequest, sanitizedError(err))
		} else {
			writeHTTPError(w, r, http.StatusInternalServerError, sanitizedError(err))
		}
		return
	}

	// Retrieve updated work item (optional)
	item, err := queue.Get(r.Context(), workItemID)
	if err != nil {
		log.Error("failed to retrieve work item after proof attach", "error", err.Error(), "id", workItemID)
		// Still return success with proof bundle
		item = types.ActionWorkItem{}
	}

	writeHTTPJSON(w, http.StatusOK, proofAttachResponse{
		Repo:        "", // not known from request
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		ProofBundle: proof,
		WorkItem:    item,
	})
}
