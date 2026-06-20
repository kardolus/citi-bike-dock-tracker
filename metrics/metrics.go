// Package metrics exposes a tiny, dependency-free Prometheus endpoint plus
// liveness/readiness handlers for the long-running `ts` ingester. Hand-rolled
// (text exposition format) to avoid pulling in the full client_golang tree for
// a handful of counters.
package metrics

import (
	"fmt"
	"net/http"
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

	// readiness grace: not ready until a recent successful ingest
	readyGrace = 300 * time.Second
)

func IncPolls()             { atomic.AddUint64(&polls, 1) }
func AddRows(n int)         { atomic.AddUint64(&rowsWritten, uint64(n)) }
func IncFetchError()        { atomic.AddUint64(&fetchErrors, 1) }
func IncDBError()           { atomic.AddUint64(&dbErrors, 1) }
func SetStations(n int)     { atomic.StoreInt64(&stations, int64(n)) }
func MarkSuccess(t time.Time) { atomic.StoreInt64(&lastSuccess, t.Unix()) }

// Handler returns the mux serving /metrics, /healthz (liveness) and /ready.
func Handler() http.Handler {
	mux := http.NewServeMux()

	// liveness: process is up
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	// readiness: a successful ingest happened recently
	mux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		last := atomic.LoadInt64(&lastSuccess)
		if last > 0 && time.Since(time.Unix(last, 0)) < readyGrace {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ready\n"))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("not ready: no recent successful ingest\n"))
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
