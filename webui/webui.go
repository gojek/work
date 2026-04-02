package webui

import (
	gocontext "context"
	"encoding/json"
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
func NewServer(namespace string, pool *redis.Pool, hostPort string) *Server {
	client := work.NewClient(namespace, pool)
	return &Server{
		server: &http.Server{Addr: hostPort, Handler: NewHandler(client)},
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
	w.server.Shutdown(gocontext.Background())
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

func (c *context) deleteAllDeadJobs(rw http.ResponseWriter, _ *http.Request) {
	err := c.client.DeleteAllDeadJobs()
	render(rw, map[string]string{"status": "ok"}, err)
}

func (c *context) retryAllDeadJobs(rw http.ResponseWriter, _ *http.Request) {
	err := c.client.RetryAllDeadJobs()
	render(rw, map[string]string{"status": "ok"}, err)
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
