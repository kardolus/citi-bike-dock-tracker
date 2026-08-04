// Package metrics exposes a tiny, dependency-free Prometheus endpoint plus
// liveness/readiness handlers for the long-running `ts` ingester. Hand-rolled
// (text exposition format) to avoid pulling in the full client_golang tree for
// a handful of counters.
package metrics

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

var (
	polls       uint64
	rowsWritten uint64
	fetchErrors uint64
	dbErrors    uint64
	stations    int64
	lastSuccess int64 // unix seconds of the last successful fetch+write

	// readiness check: reports whether the pod's dependencies (the DB) are
	// reachable. Registered by the ingester once its DB handle is open.
	// Deliberately NOT tied to ingest freshness — see the /ready handler.
	readyMu    sync.RWMutex
	readyCheck func() error
)

func IncPolls()             { atomic.AddUint64(&polls, 1) }
func AddRows(n int)         { atomic.AddUint64(&rowsWritten, uint64(n)) }
func IncFetchError()        { atomic.AddUint64(&fetchErrors, 1) }
func IncDBError()           { atomic.AddUint64(&dbErrors, 1) }
func SetStations(n int)     { atomic.StoreInt64(&stations, int64(n)) }
func MarkSuccess(t time.Time) { atomic.StoreInt64(&lastSuccess, t.Unix()) }

// SetReadyCheck registers the function /ready consults to decide readiness.
// A nil error means ready. Passing nil clears the check (readiness falls back
// to "process is up").
func SetReadyCheck(fn func() error) {
	readyMu.Lock()
	readyCheck = fn
	readyMu.Unlock()
}

// Handler returns the mux serving /metrics, /healthz (liveness) and /ready.
func Handler() http.Handler {
	mux := http.NewServeMux()

	// liveness: process is up
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	// readiness: the process is up and its dependencies (the DB) are reachable.
	// Deliberately NOT gated on ingest freshness — a transient upstream GBFS
	// outage must not flip the pod NotReady, which would page critically for a
	// remote data-source blip. Ingest staleness is surfaced separately via the
	// citibike_poll_success_timestamp_seconds metric + the CitibikeIngestStalled
	// warning alert.
	mux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		readyMu.RLock()
		chk := readyCheck
		readyMu.RUnlock()
		if chk != nil {
			if err := chk(); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = fmt.Fprintf(w, "not ready: %v\n", err)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready\n"))
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "# HELP citibike_poll_success_timestamp_seconds Unix time of the last successful fetch+write.\n")
		fmt.Fprintf(w, "# TYPE citibike_poll_success_timestamp_seconds gauge\n")
		fmt.Fprintf(w, "citibike_poll_success_timestamp_seconds %d\n", atomic.LoadInt64(&lastSuccess))
		fmt.Fprintf(w, "# HELP citibike_polls_total Total poll attempts.\n# TYPE citibike_polls_total counter\n")
		fmt.Fprintf(w, "citibike_polls_total %d\n", atomic.LoadUint64(&polls))
		fmt.Fprintf(w, "# HELP citibike_rows_written_total Total rows written to Postgres.\n# TYPE citibike_rows_written_total counter\n")
		fmt.Fprintf(w, "citibike_rows_written_total %d\n", atomic.LoadUint64(&rowsWritten))
		fmt.Fprintf(w, "# HELP citibike_fetch_errors_total Total GBFS fetch errors.\n# TYPE citibike_fetch_errors_total counter\n")
		fmt.Fprintf(w, "citibike_fetch_errors_total %d\n", atomic.LoadUint64(&fetchErrors))
		fmt.Fprintf(w, "# HELP citibike_db_errors_total Total Postgres write errors.\n# TYPE citibike_db_errors_total counter\n")
		fmt.Fprintf(w, "citibike_db_errors_total %d\n", atomic.LoadUint64(&dbErrors))
		fmt.Fprintf(w, "# HELP citibike_stations_ingested Stations written in the last successful poll.\n# TYPE citibike_stations_ingested gauge\n")
		fmt.Fprintf(w, "citibike_stations_ingested %d\n", atomic.LoadInt64(&stations))
	})

	return mux
}

// Serve starts the metrics/health server in the background. addr like ":2112".
func Serve(addr string) {
	go func() { _ = http.ListenAndServe(addr, Handler()) }()
}
