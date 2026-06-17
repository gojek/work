package webui

import (
	gocontext "context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gojek/work"
	"github.com/gojek/work/webui/internal/assets"
	"github.com/gomodule/redigo/redis"
)

// Server implements an HTTP server which exposes a JSON API to view and manage gojek/work items.
type Server struct {
	server *http.Server
}

type context struct {
	client *work.Client
}

// NewServer creates and returns a new server. The 'namespace' param is the redis namespace to use. The hostPort param is the address to bind on to expose the API.
//
// Pass HandlerOptions (e.g. WithAdminBasicAuth) to enable the admin (mutating) endpoints.
// When no options are supplied the server is read-only — preserving the historical behavior
// for existing callers.
func NewServer(namespace string, pool *redis.Pool, hostPort string, opts ...HandlerOption) *Server {
	client := work.NewClient(namespace, pool)
	return &Server{
		server: &http.Server{Addr: hostPort, Handler: NewHandler(client, opts...)},
	}
}

func mustAsset(name string) []byte {
	b, err := assets.Asset(name)
	if err != nil {
		panic(err)
	}
	return b
}

// Start starts the server listening for requests on the hostPort specified in NewServer.
func (w *Server) Start() {
	go func() {
		_ = w.server.ListenAndServe()
	}()
}

// Stop stops the server and blocks until it has finished.
func (w *Server) Stop() {
	_ = w.server.Shutdown(gocontext.Background())
}

func (c *context) ping(rw http.ResponseWriter, _ *http.Request) {
	render(rw, map[string]string{"ping": "pong", "current_time": time.Now().Format(time.RFC3339)}, nil)
}

func (c *context) queues(rw http.ResponseWriter, _ *http.Request) {
	response, err := c.client.Queues()
	render(rw, response, err)
}

func (c *context) workerPools(rw http.ResponseWriter, _ *http.Request) {
	response, err := c.client.WorkerPoolHeartbeats()
	render(rw, response, err)
}

func (c *context) busyWorkers(rw http.ResponseWriter, _ *http.Request) {
	observations, err := c.client.WorkerObservations()
	if err != nil {
		renderError(rw, err)
		return
	}

	var busyObservations []*work.WorkerObservation
	for _, ob := range observations {
		if ob.IsBusy {
			busyObservations = append(busyObservations, ob)
		}
	}

	render(rw, busyObservations, err)
}

func (c *context) retryJobs(rw http.ResponseWriter, r *http.Request) {
	page, err := parsePage(r)
	if err != nil {
		renderError(rw, err)
		return
	}

	jobs, count, err := c.client.RetryJobs(page)
	if err != nil {
		renderError(rw, err)
		return
	}

	response := struct {
		Count int64            `json:"count"`
		Jobs  []*work.RetryJob `json:"jobs"`
	}{Count: count, Jobs: jobs}

	render(rw, response, err)
}

func (c *context) scheduledJobs(rw http.ResponseWriter, r *http.Request) {
	page, err := parsePage(r)
	if err != nil {
		renderError(rw, err)
		return
	}

	jobs, count, err := c.client.ScheduledJobs(page)
	if err != nil {
		renderError(rw, err)
		return
	}

	response := struct {
		Count int64                `json:"count"`
		Jobs  []*work.ScheduledJob `json:"jobs"`
	}{Count: count, Jobs: jobs}

	render(rw, response, err)
}

func (c *context) deadJobs(rw http.ResponseWriter, r *http.Request) {
	page, err := parsePage(r)
	if err != nil {
		renderError(rw, err)
		return
	}

	jobs, count, err := c.client.DeadJobs(page)
	if err != nil {
		renderError(rw, err)
		return
	}

	response := struct {
		Count int64           `json:"count"`
		Jobs  []*work.DeadJob `json:"jobs"`
	}{Count: count, Jobs: jobs}

	render(rw, response, err)
}

func (c *context) deleteDeadJob(rw http.ResponseWriter, r *http.Request) {
	diedAt, err := strconv.ParseInt(r.PathValue("died_at"), 10, 64)
	if err != nil {
		renderError(rw, err)
		return
	}

	err = c.client.DeleteDeadJob(diedAt, r.PathValue("job_id"))

	render(rw, map[string]string{"status": "ok"}, err)
}

func (c *context) retryDeadJob(rw http.ResponseWriter, r *http.Request) {
	diedAt, err := strconv.ParseInt(r.PathValue("died_at"), 10, 64)
	if err != nil {
		renderError(rw, err)
		return
	}

	err = c.client.RetryDeadJob(diedAt, r.PathValue("job_id"))

	render(rw, map[string]string{"status": "ok"}, err)
}

func (c *context) deleteAllDeadJobs(rw http.ResponseWriter, r *http.Request) {
	if jobName := r.URL.Query().Get("job_name"); jobName != "" {
		deleted, err := c.client.DeleteAllDeadJobsByJobName(jobName)
		render(rw, map[string]any{"status": "ok", "deleted": deleted, "job_name": jobName}, err)
		return
	}
	err := c.client.DeleteAllDeadJobs()
	render(rw, map[string]string{"status": "ok"}, err)
}

func (c *context) retryAllDeadJobs(rw http.ResponseWriter, r *http.Request) {
	if jobName := r.URL.Query().Get("job_name"); jobName != "" {
		retried, err := c.client.RetryAllDeadJobsByJobName(jobName)
		render(rw, map[string]any{"status": "ok", "retried": retried, "job_name": jobName}, err)
		return
	}
	err := c.client.RetryAllDeadJobs()
	render(rw, map[string]string{"status": "ok"}, err)
}

func (c *context) setMaxConcurrency(rw http.ResponseWriter, r *http.Request) {
	jobName := r.PathValue("job_name")

	var body struct {
		MaxConcurrency uint `json:"max_concurrency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(rw, "invalid json body: "+err.Error(), http.StatusBadRequest)
		return
	}

	prev, err := c.client.SetMaxConcurrency(jobName, body.MaxConcurrency)
	if renderJobError(rw, err) {
		return
	}

	render(rw, map[string]any{
		"job_name":                 jobName,
		"max_concurrency":          body.MaxConcurrency,
		"previous_max_concurrency": prev,
	}, nil)
}

func (c *context) pauseQueue(rw http.ResponseWriter, r *http.Request) {
	jobName := r.PathValue("job_name")
	wasPaused, err := c.client.PauseJob(jobName)
	if renderJobError(rw, err) {
		return
	}
	render(rw, map[string]any{
		"job_name":          jobName,
		"paused":            true,
		"previously_paused": wasPaused,
	}, nil)
}

func (c *context) resumeQueue(rw http.ResponseWriter, r *http.Request) {
	jobName := r.PathValue("job_name")
	wasPaused, err := c.client.ResumeJob(jobName)
	if renderJobError(rw, err) {
		return
	}
	render(rw, map[string]any{
		"job_name":          jobName,
		"paused":            false,
		"previously_paused": wasPaused,
	}, nil)
}

func (c *context) purgeQueue(rw http.ResponseWriter, r *http.Request) {
	jobName := r.PathValue("job_name")
	purged, err := c.client.PurgeQueue(jobName)
	if renderJobError(rw, err) {
		return
	}
	render(rw, map[string]any{
		"job_name": jobName,
		"purged":   purged,
	}, nil)
}

func (c *context) resetLock(rw http.ResponseWriter, r *http.Request) {
	jobName := r.PathValue("job_name")
	prev, err := c.client.ResetLockCount(jobName)
	if renderJobError(rw, err) {
		return
	}
	render(rw, map[string]any{
		"job_name":            jobName,
		"lock_count":          0,
		"previous_lock_count": prev,
	}, nil)
}

func (c *context) indexPage(rw http.ResponseWriter, _ *http.Request) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = rw.Write(mustAsset("index.html"))
}

func (c *context) workJS(rw http.ResponseWriter, _ *http.Request) {
	rw.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	_, _ = rw.Write(mustAsset("work.js"))
}

func render(rw http.ResponseWriter, jsonable any, err error) {
	if err != nil {
		renderError(rw, err)
		return
	}

	jsonData, err := json.MarshalIndent(jsonable, "", "\t")
	if err != nil {
		renderError(rw, err)
		return
	}
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = rw.Write(jsonData)
}

func renderError(rw http.ResponseWriter, err error) {
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	rw.WriteHeader(500)
	_, _ = fmt.Fprintf(rw, `{"error": "%s"}`, err.Error())
}

func renderJobError(rw http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, work.ErrUnknownJob) {
		http.Error(rw, err.Error(), http.StatusNotFound)
		return true
	}
	renderError(rw, err)
	return true
}

func parsePage(r *http.Request) (uint, error) {
	err := r.ParseForm()
	if err != nil {
		return 0, err
	}

	pageStr := r.Form.Get("page")
	if pageStr == "" {
		pageStr = "1"
	}

	page, err := strconv.ParseUint(pageStr, 10, 0)
	return uint(page), err
}
