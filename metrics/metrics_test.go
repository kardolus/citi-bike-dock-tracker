package metrics

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// get issues a GET against the metrics mux and returns the status code + body.
func get(t *testing.T, path string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

// Readiness must NOT depend on ingest freshness: a stale (or never-set)
// lastSuccess — e.g. during an upstream GBFS outage — must still be ready as
// long as no dependency check fails. This is the whole point of the decoupling.
func TestReadyIgnoresIngestStaleness(t *testing.T) {
	SetReadyCheck(nil)                      // no dependency check registered
	MarkSuccess(time.Now().Add(-time.Hour)) // ingest is very stale

	if code, body := get(t, "/ready"); code != http.StatusOK {
		t.Fatalf("stale ingest with no dep check: want 200, got %d (%q)", code, body)
	}
}

// A failing dependency check (e.g. DB unreachable — a genuine local problem)
// must mark the pod NotReady so it still pages.
func TestReadyReflectsDependencyCheck(t *testing.T) {
	SetReadyCheck(func() error { return errors.New("db down") })
	defer SetReadyCheck(nil)

	if code, _ := get(t, "/ready"); code != http.StatusServiceUnavailable {
		t.Fatalf("failing dep check: want 503, got %d", code)
	}

	SetReadyCheck(func() error { return nil }) // dependency healthy
	if code, _ := get(t, "/ready"); code != http.StatusOK {
		t.Fatalf("healthy dep check: want 200, got %d", code)
	}
}

// Liveness is always OK while the process serves.
func TestHealthzAlwaysOK(t *testing.T) {
	if code, _ := get(t, "/healthz"); code != http.StatusOK {
		t.Fatalf("healthz: want 200, got %d", code)
	}
}
