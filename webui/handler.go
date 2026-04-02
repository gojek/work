package webui

import (
	"net/http"

	"github.com/gojek/work"
)

// NewHandler return *http.ServeMux for the given work.Client.
// The web UI rely on relative path, so http.Handler should be mounted on a path with trailing `/`.
// For example, if you want to mount the web UI on `/workerui`, you should use:
// ```
// handler := webui.NewHandler(client)
// mux.Handle("/workerui/", http.StripPrefix("/workerui", handler))
// ```
func NewHandler(client *work.Client) *http.ServeMux {
	ctx := context{client: client}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", ctx.ping)
	mux.HandleFunc("GET /queues", ctx.queues)
	mux.HandleFunc("GET /worker_pools", ctx.workerPools)
	mux.HandleFunc("GET /busy_workers", ctx.busyWorkers)
	mux.HandleFunc("GET /retry_jobs", ctx.retryJobs)
	mux.HandleFunc("GET /scheduled_jobs", ctx.scheduledJobs)
	mux.HandleFunc("GET /dead_jobs", ctx.deadJobs)
	mux.HandleFunc("POST /delete_dead_job/{died_at}/{job_id}", ctx.deleteDeadJob)
	mux.HandleFunc("POST /retry_dead_job/{died_at}/{job_id}", ctx.retryDeadJob)
	mux.HandleFunc("POST /delete_all_dead_jobs", ctx.deleteAllDeadJobs)
	mux.HandleFunc("POST /retry_all_dead_jobs", ctx.retryAllDeadJobs)
	mux.HandleFunc("GET /{$}", ctx.indexPage)
	mux.HandleFunc("GET /work.js", ctx.workJS)

	return mux
}
