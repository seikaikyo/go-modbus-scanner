package scanner

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/goburrow/modbus"
)

// Job represents an async scan job.
type Job struct {
	ID        string      `json:"job_id"`
	Status    string      `json:"status"` // running, completed, failed
	Request   ScanRequest `json:"request"`
	Result    *ScanResult `json:"result,omitempty"`
	Error     string      `json:"error,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
}

type jobStore struct {
	mu   sync.RWMutex
	jobs map[string]*Job
	seq  int
}

var store = &jobStore{jobs: make(map[string]*Job)}

func (s *jobStore) create(req ScanRequest) *Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	id := fmt.Sprintf("scan-%d", s.seq)
	job := &Job{
		ID:        id,
		Status:    "running",
		Request:   req,
		CreatedAt: time.Now(),
	}
	s.jobs[id] = job
	return job
}

func (s *jobStore) get(id string) *Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.jobs[id]
}

func (s *jobStore) list() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]*Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		jobs = append(jobs, j)
	}
	return jobs
}

// Router returns the chi router with API + embedded UI.
func Router() chi.Router {
	r := chi.NewRouter()

	// API
	r.Post("/api/scan", handleScan)
	r.Post("/api/scan/quick", handleQuickScan)
	r.Post("/api/read", handleRead)
	r.Get("/api/jobs", handleListJobs)
	r.Get("/api/jobs/{id}", handleGetJob)

	// Embedded frontend
	r.HandleFunc("/*", staticHandler())
	r.HandleFunc("/", staticHandler())

	return r
}

func handleScan(w http.ResponseWriter, r *http.Request) {
	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Host == "" {
		respondErr(w, http.StatusBadRequest, "host is required")
		return
	}

	req.applyDefaults()
	job := store.create(req)

	go func() {
		slog.Info("scan started", "job_id", job.ID, "host", req.Host, "port", req.Port)
		result, err := Scan(req)

		store.mu.Lock()
		defer store.mu.Unlock()

		if err != nil {
			job.Status = "failed"
			job.Error = err.Error()
			slog.Error("scan failed", "job_id", job.ID, "error", err)
		} else {
			job.Status = "completed"
			job.Result = result
			slog.Info("scan completed", "job_id", job.ID,
				"responsive", result.Summary.Responsive,
				"dynamic", result.Summary.Dynamic,
				"duration_ms", result.DurationMs,
			)
		}
	}()

	respondOK(w, map[string]any{
		"job_id":  job.ID,
		"status":  job.Status,
		"message": "scan started, poll GET /api/jobs/" + job.ID + " for results",
	})
}

func handleQuickScan(w http.ResponseWriter, r *http.Request) {
	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Host == "" {
		respondErr(w, http.StatusBadRequest, "host is required")
		return
	}

	req.applyDefaults()
	req.ScanTypes = []string{"holding"}
	if req.AddressEnd > 999 {
		req.AddressEnd = 999
	}
	req.Samples = 1

	job := store.create(req)

	go func() {
		slog.Info("quick scan started", "job_id", job.ID, "host", req.Host)
		result, err := Scan(req)

		store.mu.Lock()
		defer store.mu.Unlock()

		if err != nil {
			job.Status = "failed"
			job.Error = err.Error()
		} else {
			job.Status = "completed"
			job.Result = result
		}
	}()

	respondOK(w, map[string]any{
		"job_id":  job.ID,
		"status":  job.Status,
		"message": "quick scan started",
	})
}

func handleRead(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host    string `json:"host"`
		Port    int    `json:"port"`
		UnitID  uint8  `json:"unit_id"`
		Type    string `json:"type"`
		Address uint16 `json:"address"`
		Count   uint16 `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Host == "" {
		respondErr(w, http.StatusBadRequest, "host is required")
		return
	}
	if req.Port == 0 {
		req.Port = 502
	}
	if req.UnitID == 0 {
		req.UnitID = 1
	}
	if req.Type == "" {
		req.Type = "holding"
	}
	if req.Count == 0 {
		req.Count = 1
	}
	if req.Count > 125 {
		req.Count = 125
	}

	addr := fmt.Sprintf("%s:%d", req.Host, req.Port)
	handler := modbus.NewTCPClientHandler(addr)
	handler.Timeout = 500 * time.Millisecond
	handler.SlaveId = req.UnitID

	if err := handler.Connect(); err != nil {
		respondErr(w, http.StatusBadGateway, "connect failed: "+err.Error())
		return
	}
	defer handler.Close()

	client := modbus.NewClient(handler)
	data, err := readBatch(client, req.Type, req.Address, req.Count)
	if err != nil {
		respondErr(w, http.StatusBadGateway, "read failed: "+err.Error())
		return
	}

	values := bytesToUint16(data)
	respondOK(w, map[string]any{
		"device":  addr,
		"unit_id": req.UnitID,
		"type":    req.Type,
		"address": req.Address,
		"count":   len(values),
		"values":  values,
	})
}

func handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobs := store.list()
	summaries := make([]map[string]any, len(jobs))
	for i, j := range jobs {
		s := map[string]any{
			"job_id":     j.ID,
			"status":     j.Status,
			"device":     fmt.Sprintf("%s:%d", j.Request.Host, j.Request.Port),
			"created_at": j.CreatedAt,
		}
		if j.Result != nil {
			s["summary"] = j.Result.Summary
			s["duration_ms"] = j.Result.DurationMs
		}
		if j.Error != "" {
			s["error"] = j.Error
		}
		summaries[i] = s
	}
	respondOK(w, summaries)
}

func handleGetJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	job := store.get(id)
	if job == nil {
		respondErr(w, http.StatusNotFound, "job not found")
		return
	}
	respondOK(w, job)
}
