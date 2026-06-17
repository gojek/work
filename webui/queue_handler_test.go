package webui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gojek/work"
	"github.com/gomodule/redigo/redis"
	"github.com/stretchr/testify/suite"
)

const (
	testAuthUser = "admin"
	testAuthPass = "secret"
)

type TestWebUIQueueSuite struct {
	suite.Suite

	ns       string
	pool     *redis.Pool
	server   *httptest.Server
	enqueuer *work.Enqueuer
}

func TestWebUIQueue(t *testing.T) {
	suite.Run(t, new(TestWebUIQueueSuite))
}

func (s *TestWebUIQueueSuite) SetupSuite() {
	s.pool = newTestPool(s.T())
	s.ns = "work"
	handler := NewHandler(work.NewClient(s.ns, s.pool), WithBasicAuth(testAuthUser, testAuthPass))
	s.server = httptest.NewServer(handler)
	s.enqueuer = work.NewEnqueuer(s.ns, s.pool)
}

func (s *TestWebUIQueueSuite) TearDownSuite() {
	s.server.Close()
}

func (s *TestWebUIQueueSuite) SetupTest() {
	cleanKeyspace(s.ns, s.pool)
}

func (s *TestWebUIQueueSuite) registerKnownJobs(names ...string) {
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

// doRequest issues an HTTP request, optionally with valid Basic Auth credentials.
func (s *TestWebUIQueueSuite) doRequest(method, path string, body any, auth bool) *http.Response {
	var rdr *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		s.NoError(err)
		rdr = bytes.NewReader(raw)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, s.server.URL+path, rdr)
	s.NoError(err)
	if auth {
		req.SetBasicAuth(testAuthUser, testAuthPass)
	}
	resp, err := s.server.Client().Do(req)
	s.NoError(err)
	return resp
}

func (s *TestWebUIQueueSuite) TestSetMaxConcurrency_HappyPath() {
	s.registerKnownJobs("alpha")

	resp := s.doRequest(http.MethodPut, "/queues/alpha/max_concurrency", map[string]int{"max_concurrency": 9}, true)
	defer resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)
	var body map[string]any
	s.NoError(json.NewDecoder(resp.Body).Decode(&body))
	s.EqualValues(9, body["max_concurrency"])
	s.EqualValues(0, body["previous_max_concurrency"])

	resp2 := s.doRequest(http.MethodPut, "/queues/alpha/max_concurrency", map[string]int{"max_concurrency": 12}, true)
	defer resp2.Body.Close()
	var body2 map[string]any
	s.NoError(json.NewDecoder(resp2.Body).Decode(&body2))
	s.EqualValues(12, body2["max_concurrency"])
	s.EqualValues(9, body2["previous_max_concurrency"])
}

func (s *TestWebUIQueueSuite) TestSetMaxConcurrency_BadJSON() {
	s.registerKnownJobs("alpha")
	req, err := http.NewRequest(http.MethodPut, s.server.URL+"/queues/alpha/max_concurrency", bytes.NewReader([]byte("not json")))
	s.NoError(err)
	req.SetBasicAuth(testAuthUser, testAuthPass)
	resp, err := s.server.Client().Do(req)
	s.NoError(err)
	defer resp.Body.Close()
	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *TestWebUIQueueSuite) TestPauseResume_HappyPath() {
	s.registerKnownJobs("alpha")

	resp := s.doRequest(http.MethodPost, "/queues/alpha/pause", nil, true)
	defer resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)
	var body map[string]any
	s.NoError(json.NewDecoder(resp.Body).Decode(&body))
	s.Equal(true, body["paused"])

	resp2 := s.doRequest(http.MethodPost, "/queues/alpha/resume", nil, true)
	defer resp2.Body.Close()
	s.Equal(http.StatusOK, resp2.StatusCode)
	var body2 map[string]any
	s.NoError(json.NewDecoder(resp2.Body).Decode(&body2))
	s.Equal(false, body2["paused"])
}

func (s *TestWebUIQueueSuite) TestPurgeQueue_RemovesJobs() {
	s.registerKnownJobs("alpha")
	for range 3 {
		_, err := s.enqueuer.Enqueue("alpha", nil)
		s.NoError(err)
	}

	resp := s.doRequest(http.MethodPost, "/queues/alpha/purge", nil, true)
	defer resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)
	var body map[string]any
	s.NoError(json.NewDecoder(resp.Body).Decode(&body))
	s.EqualValues(3, body["purged"])

	conn := s.pool.Get()
	defer conn.Close()
	llen, err := redis.Int64(conn.Do("LLEN", "work:jobs:alpha"))
	s.NoError(err)
	s.EqualValues(0, llen)
}

func (s *TestWebUIQueueSuite) TestResetLock_HappyPath() {
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

func (s *TestWebUIQueueSuite) TestUnknownJob_NotFound() {
	resp := s.doRequest(http.MethodPost, "/queues/nope/pause", nil, true)
	defer resp.Body.Close()
	s.Equal(http.StatusNotFound, resp.StatusCode)
}

// Auth wraps the WHOLE UI: even read-only routes require credentials when enabled.
func (s *TestWebUIQueueSuite) TestReadRoute_RequiresAuth() {
	resp := s.doRequest(http.MethodGet, "/queues", nil, false)
	defer resp.Body.Close()
	s.Equal(http.StatusUnauthorized, resp.StatusCode)
	s.NotEmpty(resp.Header.Get("WWW-Authenticate"))
}

func (s *TestWebUIQueueSuite) TestWrongPassword_Unauthorized() {
	req, err := http.NewRequest(http.MethodGet, s.server.URL+"/queues", nil)
	s.NoError(err)
	req.SetBasicAuth(testAuthUser, "wrong")
	resp, err := s.server.Client().Do(req)
	s.NoError(err)
	defer resp.Body.Close()
	s.Equal(http.StatusUnauthorized, resp.StatusCode)
}

func (s *TestWebUIQueueSuite) TestWithCredentials_OK() {
	resp := s.doRequest(http.MethodGet, "/queues", nil, true)
	defer resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)
}

// When auth is not configured, every route — including the mutating queue endpoints — is open.
func TestAuthDisabled_RoutesOpen(t *testing.T) {
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

	// Read route open.
	resp, err := srv.Client().Get(srv.URL + "/queues")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /queues without auth, got %d", resp.StatusCode)
	}

	// Mutating route registered and open.
	resp2, err := srv.Client().Post(srv.URL+"/queues/alpha/pause", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for pause without auth, got %d", resp2.StatusCode)
	}
}
