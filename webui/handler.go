package webui

import (
	"net/http"

	"github.com/gojek/work"
)

// HandlerOption configures the optional behavior of NewHandler / NewServer. It is the
// extension point for future server knobs (e.g. additional middleware) without breaking
// the existing constructor signature.
type HandlerOption func(*handlerConfig)

type handlerConfig struct {
	admin AdminOptions
}

// WithAdminBasicAuth enables the admin (mutating) endpoints behind HTTP Basic Auth using
// the supplied credentials. When this option is not provided, the admin endpoints are
// not registered at all — preserving the historical read-only handler shape.
func WithAdminBasicAuth(username, password string) HandlerOption {
	return func(c *handlerConfig) {
		c.admin = AdminOptions{Username: username, Password: password}
	}
}

// WithAdminOptions enables the admin endpoints with full control over realm and credentials.
// Prefer WithAdminBasicAuth for the common case.
func WithAdminOptions(o AdminOptions) HandlerOption {
	return func(c *handlerConfig) { c.admin = o }
}

// NewHandler return *http.ServeMux for the given work.Client.
// The web UI rely on relative path, so http.Handler should be mounted on a path with trailing `/`.
// For example, if you want to mount the web UI on `/workerui`, you should use:
// ```
// handler := webui.NewHandler(client)
// mux.Handle("/workerui/", http.StripPrefix("/workerui", handler))
// ```
func NewHandler(client *work.Client, opts ...HandlerOption) *http.ServeMux {
	cfg := &handlerConfig{}
	for _, o := range opts {
		o(cfg)
	}
	ctx := context{client: client}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", ctx.ping)
	mux.HandleFunc("GET /admin_status", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if cfg.admin.Enabled() {
			_, _ = w.Write([]byte(`{"enabled":true}`))
		} else {
			_, _ = w.Write([]byte(`{"enabled":false}`))
		}
	})
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

	if cfg.admin.Enabled() {
		gate := func(h http.HandlerFunc) http.Handler {
			return requireBasicAuth(cfg.admin, h)
		}
		mux.Handle("PUT /queues/{job_name}/max_concurrency", gate(ctx.setMaxConcurrency))
		mux.Handle("POST /queues/{job_name}/pause", gate(ctx.pauseQueue))
		mux.Handle("POST /queues/{job_name}/resume", gate(ctx.resumeQueue))
		mux.Handle("POST /queues/{job_name}/purge", gate(ctx.purgeQueue))
		mux.Handle("POST /queues/{job_name}/reset_lock", gate(ctx.resetLock))
	}

	return mux
}
