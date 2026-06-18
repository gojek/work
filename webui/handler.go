package webui

import (
	"net/http"

	"github.com/gojek/work"
)

// HandlerOption configures optional behavior of NewHandler / NewServer.
type HandlerOption func(*handlerConfig)

type handlerConfig struct {
	auth AuthOptions
}

// WithBasicAuth protects the entire Web UI — every page, the JSON API, and the mutating
// queue endpoints — with HTTP Basic Auth. When this option is not provided, the Web UI is
// served openly (the historical behavior).
func WithBasicAuth(username, password string) HandlerOption {
	return func(c *handlerConfig) {
		c.auth = AuthOptions{Username: username, Password: password}
	}
}

// WithAuthOptions is like WithBasicAuth but also lets the caller set the realm.
func WithAuthOptions(o AuthOptions) HandlerOption {
	return func(c *handlerConfig) { c.auth = o }
}

// NewHandler returns the Web UI HTTP handler for the given work.Client.
// The web UI relies on relative paths, so it should be mounted on a path with a trailing `/`.
// For example, to mount the web UI on `/workerui`:
// ```
// handler := webui.NewHandler(client)
// mux.Handle("/workerui/", http.StripPrefix("/workerui", handler))
// ```
// Pass WithBasicAuth to require HTTP Basic Auth on every route.
func NewHandler(client *work.Client, opts ...HandlerOption) http.Handler {
	cfg := &handlerConfig{}
	for _, o := range opts {
		o(cfg)
	}
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

	// Queue-management (mutating) endpoints. Always registered; protection, if any, comes
	// from WithBasicAuth wrapping the whole handler below.
	mux.HandleFunc("PUT /queues/{job_name}/max_concurrency", ctx.setMaxConcurrency)
	mux.HandleFunc("POST /queues/{job_name}/pause", ctx.pauseQueue)
	mux.HandleFunc("POST /queues/{job_name}/resume", ctx.resumeQueue)
	mux.HandleFunc("POST /queues/{job_name}/purge", ctx.purgeQueue)
	mux.HandleFunc("POST /queues/{job_name}/reset_lock", ctx.resetLock)

	mux.HandleFunc("GET /{$}", ctx.indexPage)
	mux.HandleFunc("GET /work.js", ctx.workJS)

	if cfg.auth.Enabled() {
		return requireBasicAuth(cfg.auth, mux)
	}
	return mux
}
