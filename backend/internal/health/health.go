package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type Check struct {
	Name string
	Run  func(context.Context) error
}
type Service struct {
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	LatencyMS float64 `json:"latency_ms"`
	Message   string  `json:"message"`
}
type Report struct {
	Status    string    `json:"status"`
	CheckedAt string    `json:"checked_at"`
	Services  []Service `json:"services"`
}
type Handler struct {
	Checks  []Check
	Timeout time.Duration
}

func (h Handler) report(ctx context.Context) Report {
	ctx, cancel := context.WithTimeout(ctx, h.Timeout)
	defer cancel()
	report := Report{Status: "ok", Services: make([]Service, len(h.Checks)+1)}
	report.Services[0] = Service{Name: "backend", Status: "up", Message: "HTTP server is running"}
	var wg sync.WaitGroup
	for i, check := range h.Checks {
		wg.Add(1)
		go func(i int, check Check) {
			defer wg.Done()
			start := time.Now()
			err := check.Run(ctx)
			s := Service{Name: check.Name, Status: "up", Message: "Connection check passed", LatencyMS: float64(time.Since(start).Microseconds()) / 1000}
			if err != nil {
				s.Status = "down"
				// Never return driver errors: they may contain credentials or connection strings.
				s.Message = "Connection check failed; verify service, port forwarding and credentials"
				if ctx.Err() != nil {
					s.Message = "Connection check timed out or was cancelled"
				}
			}
			report.Services[i+1] = s
		}(i, check)
	}
	wg.Wait()
	for _, s := range report.Services {
		if s.Status == "down" {
			report.Status = "degraded"
		}
	}
	report.CheckedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return report
}

func writeJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}
func (h Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/health/ready", func(w http.ResponseWriter, r *http.Request) {
		report := h.report(r.Context())
		code := http.StatusOK
		if report.Status != "ok" {
			code = http.StatusServiceUnavailable
		}
		writeJSON(w, code, report)
	})
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, h.report(r.Context())) })
	return mux
}
