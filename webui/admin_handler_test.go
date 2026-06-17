package webui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gojek/work"
	"github.com/gomodule/redigo/redis"
	"github.com/stretchr/testify/suite"
)

type TestWebUIAdminSuite struct {
	suite.Suite

	ns       string
	pool     *redis.Pool
	server   *httptest.Server
	enqueuer *work.Enqueuer
}

func (s *TestWebUIAdminSuite) SetupSuite() {
	s.pool = newTestPool(s.T())
	s.ns = "work"

	handler := NewHandler(work.NewClient(s.ns, s.pool), WithAdminBasicAuth("admin", "secret"))
	mux := http.NewServeMux()
	mux.Handle("/", handler)
	s.server = httptest.NewServer(mux)

	s.enqueuer = work.NewEnqueuer(s.ns, s.pool)
}

func (s *TestWebUIAdminSuite) TearDownSuite() {
	s.server.Close()
}

func (s *TestWebUIAdminSuite) SetupTest() {
	cleanKeyspace(s.ns, s.pool)
}

func TestWebUIAdmin(t *testing.T) {
	suite.Run(t, new(TestWebUIAdminSuite))
}

func (s *TestWebUIAdminSuite) registerKnownJobs(names ...string) {
	conn := s.pool.Get()
	defer conn.Close()
	args := make([]any, 0, len(names)+1)
	args = append(args, "work:known_jobs")
	for _, n := range names {
		args = append(args, n)
	}
	_, err := conn.Do("SADD", args...)
	s.NoError(err)
}

func (s *TestWebUIAdminSuite) doRequest(method, path string, body any, withAuth bool) *http.Response {
	var rdr *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		s.NoError(err)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, s.server.URL+path, rdr)
	s.NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if withAuth {
		req.SetBasicAuth("admin", "secret")
	}
	resp, err := s.server.Client().Do(req)
	s.NoError(err)
	return resp
}

func (s *TestWebUIAdminSuite) TestUnauthenticated_ReturnsWWWAuthenticate() {
	s.registerKnownJobs("alpha")
	resp := s.doRequest(http.MethodPost, "/queues/alpha/pause", nil, false)
	defer resp.Body.Close()
	s.Equal(http.StatusUnauthorized, resp.StatusCode)
	s.Contains(resp.Header.Get("WWW-Authenticate"), `Basic realm="work-admin"`)
}

func (s *TestWebUIAdminSuite) TestWrongPassword_Unauthorized() {
	s.registerKnownJobs("alpha")
	req, err := http.NewRequest(http.MethodPost, s.server.URL+"/queues/alpha/pause", nil)
	s.NoError(err)
	req.SetBasicAuth("admin", "wrong")
	resp, err := s.server.Client().Do(req)
	s.NoError(err)
	defer resp.Body.Close()
	s.Equal(http.StatusUnauthorized, resp.StatusCode)
}

func (s *TestWebUIAdminSuite) TestUnknownJob_NotFound() {
	resp := s.doRequest(http.MethodPost, "/queues/ghost/pause", nil, true)
	defer resp.Body.Close()
	s.Equal(http.StatusNotFound, resp.StatusCode)
}

func (s *TestWebUIAdminSuite) TestPauseResume_HappyPath() {
	s.registerKnownJobs("alpha")

	resp := s.doRequest(http.MethodPost, "/queues/alpha/pause", nil, true)
	defer resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)
	var body map[string]any
	s.NoError(json.NewDecoder(resp.Body).Decode(&body))
	s.Equal(true, body["paused"])
	s.Equal(false, body["previously_paused"])

	// /queues should now report paused: true
	r2, err := s.server.Client().Get(s.server.URL + "/queues")
	s.NoError(err)
	defer r2.Body.Close()
	var queues []map[string]any
	s.NoError(json.NewDecoder(r2.Body).Decode(&queues))
	if s.Equal(1, len(queues)) {
		s.Equal("alpha", queues[0]["job_name"])
		s.Equal(true, queues[0]["paused"])
	}

	// Resume
	resp2 := s.doRequest(http.MethodPost, "/queues/alpha/resume", nil, true)
	defer resp2.Body.Close()
	s.Equal(http.StatusOK, resp2.StatusCode)
	var body2 map[string]any
	s.NoError(json.NewDecoder(resp2.Body).Decode(&body2))
	s.Equal(false, body2["paused"])
	s.Equal(true, body2["previously_paused"])
}

func (s *TestWebUIAdminSuite) TestSetMaxConcurrency_HappyPath() {
	s.registerKnownJobs("alpha")

	resp := s.doRequest(http.MethodPut, "/queues/alpha/max_concurrency",
		map[string]int{"max_concurrency": 9}, true)
	defer resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)
	var body map[string]any
	s.NoError(json.NewDecoder(resp.Body).Decode(&body))
	s.EqualValues(9, body["max_concurrency"])
	s.EqualValues(0, body["previous_max_concurrency"])

	resp2 := s.doRequest(http.MethodPut, "/queues/alpha/max_concurrency",
		map[string]int{"max_concurrency": 12}, true)
	defer resp2.Body.Close()
	s.Equal(http.StatusOK, resp2.StatusCode)
	var body2 map[string]any
	s.NoError(json.NewDecoder(resp2.Body).Decode(&body2))
	s.EqualValues(12, body2["max_concurrency"])
	s.EqualValues(9, body2["previous_max_concurrency"])
}

func (s *TestWebUIAdminSuite) TestSetMaxConcurrency_BadJSON() {
	s.registerKnownJobs("alpha")
	req, err := http.NewRequest(http.MethodPut, s.server.URL+"/queues/alpha/max_concurrency",
		bytes.NewReader([]byte("not-json")))
	s.NoError(err)
	req.SetBasicAuth("admin", "secret")
	resp, err := s.server.Client().Do(req)
	s.NoError(err)
	defer resp.Body.Close()
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *TestWebUIAdminSuite) TestPurgeQueue_RemovesJobs() {
	enq := s.enqueuer
	for range 3 {
		_, err := enq.Enqueue("alpha", nil)
		s.NoError(err)
	}
	s.registerKnownJobs("alpha")

	resp := s.doRequest(http.MethodPost, "/queues/alpha/purge", nil, true)
	defer resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)
	var body map[string]any
	s.NoError(json.NewDecoder(resp.Body).Decode(&body))
	s.EqualValues(3, body["purged"])

	r2, err := s.server.Client().Get(s.server.URL + "/queues")
	s.NoError(err)
	defer r2.Body.Close()
	var queues []map[string]any
	s.NoError(json.NewDecoder(r2.Body).Decode(&queues))
	s.Equal(1, len(queues))
	s.EqualValues(0, queues[0]["count"])
}

func (s *TestWebUIAdminSuite) TestResetLock_HappyPath() {
	s.registerKnownJobs("alpha")

	conn := s.pool.Get()
	_, err := conn.Do("SET", "work:jobs:alpha:lock", 8)
	conn.Close()
	s.NoError(err)

	resp := s.doRequest(http.MethodPost, "/queues/alpha/reset_lock", nil, true)
	defer resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)
	var body map[string]any
	s.NoError(json.NewDecoder(resp.Body).Decode(&body))
	s.EqualValues(0, body["lock_count"])
	s.EqualValues(8, body["previous_lock_count"])
}

func (s *TestWebUIAdminSuite) TestExistingDeadJobMutations_RemainOpen() {
	// The one-pager + plan explicitly leave existing dead-job mutations unauthenticated.
	enq := s.enqueuer
	_, err := enq.Enqueue("alpha", nil)
	s.NoError(err)

	wp := work.NewWorkerPool(TestContext{}, 2, s.ns, s.pool)
	wp.JobWithOptions("alpha", work.JobOptions{Priority: 1, MaxFails: 1}, func(_ *work.Job) error {
		return fmt.Errorf("ohno")
	})
	wp.Start()
	wp.Drain()
	wp.Stop()

	// retry_all_dead_jobs without auth should still be 200
	req, err := http.NewRequest(http.MethodPost, s.server.URL+"/retry_all_dead_jobs", nil)
	s.NoError(err)
	resp, err := s.server.Client().Do(req)
	s.NoError(err)
	defer resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)
}

// TestAdminDisabled_RoutesNotRegistered builds a separate server with no admin options and
// asserts the new routes are simply not present (404). Existing routes still work.
func TestAdminDisabled_RoutesNotRegistered(t *testing.T) {
	pool := newTestPool(t)
	ns := "work"
	cleanKeyspace(ns, pool)

	conn := pool.Get()
	_, err := conn.Do("SADD", "work:known_jobs", "alpha")
	conn.Close()
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(NewHandler(work.NewClient(ns, pool)))
	defer srv.Close()

	// Admin endpoint -> 404.
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/queues/alpha/pause", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for admin endpoint when admin is disabled, got %d", resp.StatusCode)
	}

	// Existing GET still works.
	resp2, err := srv.Client().Get(srv.URL + "/ping")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected /ping to still work, got %d", resp2.StatusCode)
	}
}

func (s *TestWebUIAdminSuite) TestDeleteAllDeadJobs_ByJobName() {
	conn := s.pool.Get()
	defer conn.Close()
	for i, name := range []string{"alpha", "beta", "alpha", "alpha"} {
		job := &work.Job{
			Name:     name,
			ID:       fmt.Sprintf("id-%d", i),
			Fails:    1,
			LastErr:  "oops",
			FailedAt: int64(2000 + i),
		}
		raw, err := jobBytes(job)
		s.NoError(err)
		_, err = conn.Do("ZADD", "work:dead", job.FailedAt, raw)
		s.NoError(err)
	}

	resp := s.doRequest(http.MethodPost, "/delete_all_dead_jobs?job_name=alpha", nil, false)
	defer resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)
	var body map[string]any
	s.NoError(json.NewDecoder(resp.Body).Decode(&body))
	s.EqualValues(3, body["deleted"])
	s.Equal("alpha", body["job_name"])

	left, err := redis.Int64(conn.Do("ZCARD", "work:dead"))
	s.NoError(err)
	s.EqualValues(1, left)
}

func (s *TestWebUIAdminSuite) TestRetryAllDeadJobs_ByJobName() {
	s.registerKnownJobs("alpha", "beta")

	conn := s.pool.Get()
	defer conn.Close()
	for i, name := range []string{"alpha", "beta", "alpha"} {
		job := &work.Job{
			Name:     name,
			ID:       fmt.Sprintf("id-%d", i),
			Fails:    1,
			LastErr:  "oops",
			FailedAt: int64(2000 + i),
		}
		raw, err := jobBytes(job)
		s.NoError(err)
		_, err = conn.Do("ZADD", "work:dead", job.FailedAt, raw)
		s.NoError(err)
	}

	resp := s.doRequest(http.MethodPost, "/retry_all_dead_jobs?job_name=alpha", nil, false)
	defer resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)
	var body map[string]any
	s.NoError(json.NewDecoder(resp.Body).Decode(&body))
	s.EqualValues(2, body["retried"])

	left, err := redis.Int64(conn.Do("ZCARD", "work:dead"))
	s.NoError(err)
	s.EqualValues(1, left)

	llen, err := redis.Int64(conn.Do("LLEN", "work:jobs:alpha"))
	s.NoError(err)
	s.EqualValues(2, llen)
}

// jobBytes serializes a work.Job using the package's JSON marshaling. Because Job.serialize
// is unexported, we build the JSON ourselves matching the same wire format the tests use.
func jobBytes(j *work.Job) ([]byte, error) {
	type wireJob struct {
		Name       string         `json:"name"`
		ID         string         `json:"id"`
		EnqueuedAt int64          `json:"t"`
		Args       map[string]any `json:"args"`
		Unique     bool           `json:"unique,omitempty"`
		Fails      int64          `json:"fails,omitempty"`
		LastErr    string         `json:"err,omitempty"`
		FailedAt   int64          `json:"failed_at,omitempty"`
	}
	return json.Marshal(wireJob{
		Name:       j.Name,
		ID:         j.ID,
		EnqueuedAt: j.EnqueuedAt,
		Args:       j.Args,
		Unique:     j.Unique,
		Fails:      j.Fails,
		LastErr:    j.LastErr,
		FailedAt:   j.FailedAt,
	})
}

// (kept here so future test additions can reuse it without re-importing time)
var _ = time.Second
