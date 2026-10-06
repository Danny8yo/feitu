package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEndpointsAndCredentialRedaction(t *testing.T) {
	for _, failed := range []bool{false, true} {
		h := Handler{Timeout: time.Second, Checks: []Check{{Name: "mysql", Run: func(context.Context) error {
			if failed {
				return errors.New("secret-password@host")
			}
			return nil
		}}, {Name: "kafka", Run: func(context.Context) error { return nil }}, {Name: "redis", Run: func(context.Context) error { return nil }}}}.Routes()
		for _, path := range []string{"/api/health/ready", "/api/status"} {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
			wantCode := 200
			if failed && path == "/api/health/ready" {
				wantCode = 503
			}
			if w.Code != wantCode {
				t.Fatalf("%s: got %d want %d", path, w.Code, wantCode)
			}
			if strings.Contains(w.Body.String(), "secret-password") {
				t.Fatal("driver error leaked")
			}
			var report Report
			if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			want := "ok"
			if failed {
				want = "degraded"
			}
			if report.Status != want {
				t.Fatalf("got %s", report.Status)
			}
			if len(report.Services) != 4 || report.Services[0].Status != "up" {
				t.Fatal("missing service state")
			}
			if _, err := time.Parse(time.RFC3339Nano, report.CheckedAt); err != nil {
				t.Fatal(err)
			}
		}
	}
}
func TestLivenessDoesNotCheckDependencies(t *testing.T) {
	h := Handler{Timeout: time.Second, Checks: []Check{{Name: "mysql", Run: func(context.Context) error { t.Fatal("liveness checked dependency"); return nil }}}}.Routes()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/health/live", nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
}
func TestChecksRunConcurrentlyAndTimeOut(t *testing.T) {
	started := make(chan struct{}, 3)
	check := func(ctx context.Context) error { started <- struct{}{}; <-ctx.Done(); return ctx.Err() }
	h := Handler{Timeout: 100 * time.Millisecond, Checks: []Check{{"mysql", check}, {"kafka", check}, {"redis", check}}}
	done := make(chan Report, 1)
	go func() { done <- h.report(context.Background()) }()
	for range 3 {
		select {
		case <-started:
		case <-time.After(80 * time.Millisecond):
			t.Fatal("checks did not start concurrently")
		}
	}
	select {
	case report := <-done:
		if report.Status != "degraded" {
			t.Fatal(report.Status)
		}
	case <-time.After(time.Second):
		t.Fatal("deadline not respected")
	}
}
