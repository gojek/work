package work

import (
	"errors"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/stretchr/testify/assert"
)

// makeKnownJobs registers the named jobs in the known_jobs set so the queue-management Client
// methods will accept them. The helper does not start any worker.
func makeKnownJobs(t *testing.T, ns string, pool *redis.Pool, names ...string) {
	t.Helper()
	conn := pool.Get()
	defer conn.Close()
	args := make([]any, 0, len(names)+1)
	args = append(args, redisKeyKnownJobs(ns))
	for _, n := range names {
		args = append(args, n)
	}
	_, err := conn.Do("SADD", args...)
	assert.NoError(t, err)
}

func getRedisString(t *testing.T, pool *redis.Pool, key string) (string, bool) {
	t.Helper()
	conn := pool.Get()
	defer conn.Close()
	v, err := redis.String(conn.Do("GET", key))
	if err == redis.ErrNil {
		return "", false
	}
	assert.NoError(t, err)
	return v, true
}

func TestClient_jobNameKnown(t *testing.T) {
	pool := newTestPool(t)
	ns := "work"
	cleanKeyspace(ns, pool)

	c := NewClient(ns, pool)
	makeKnownJobs(t, ns, pool, "alpha")

	ok, err := c.jobNameKnown("alpha")
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = c.jobNameKnown("beta")
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestClient_SetMaxConcurrency(t *testing.T) {
	pool := newTestPool(t)
	ns := "work"
	cleanKeyspace(ns, pool)

	c := NewClient(ns, pool)
	makeKnownJobs(t, ns, pool, "alpha")

	prev, err := c.SetMaxConcurrency("alpha", 7)
	assert.NoError(t, err)
	assert.EqualValues(t, 0, prev) // nothing set yet -> ErrNil swallowed -> 0

	v, ok := getRedisString(t, pool, redisKeyJobsConcurrency(ns, "alpha"))
	assert.True(t, ok)
	assert.Equal(t, "7", v)

	prev, err = c.SetMaxConcurrency("alpha", 12)
	assert.NoError(t, err)
	assert.EqualValues(t, 7, prev)

	_, err = c.SetMaxConcurrency("nope", 1)
	assert.True(t, errors.Is(err, ErrUnknownJob))
}

func TestClient_PauseResumeJob(t *testing.T) {
	pool := newTestPool(t)
	ns := "work"
	cleanKeyspace(ns, pool)

	c := NewClient(ns, pool)
	makeKnownJobs(t, ns, pool, "alpha")

	wasPaused, err := c.PauseJob("alpha")
	assert.NoError(t, err)
	assert.False(t, wasPaused)

	_, ok := getRedisString(t, pool, redisKeyJobsPaused(ns, "alpha"))
	assert.True(t, ok, "paused key should be present after PauseJob")

	wasPaused, err = c.PauseJob("alpha")
	assert.NoError(t, err)
	assert.True(t, wasPaused, "second pause should report previously-paused")

	wasPaused, err = c.ResumeJob("alpha")
	assert.NoError(t, err)
	assert.True(t, wasPaused, "first resume removed the key")

	_, ok = getRedisString(t, pool, redisKeyJobsPaused(ns, "alpha"))
	assert.False(t, ok, "paused key should be gone after ResumeJob")

	wasPaused, err = c.ResumeJob("alpha")
	assert.NoError(t, err)
	assert.False(t, wasPaused, "second resume should be a no-op")

	_, err = c.PauseJob("nope")
	assert.True(t, errors.Is(err, ErrUnknownJob))
}

// TestClient_PauseJob_FetcherSkipsQueue verifies that the existing Lua fetcher honors the
// pause flag set by the queue-management endpoint. We enqueue a job, pause the queue, then
// briefly run a worker pool and confirm the job is still pending.
func TestClient_PauseJob_FetcherSkipsQueue(t *testing.T) {
	pool := newTestPool(t)
	ns := "work"
	cleanKeyspace(ns, pool)

	enq := NewEnqueuer(ns, pool)
	_, err := enq.Enqueue("alpha", nil)
	assert.NoError(t, err)

	wp := NewWorkerPool(TestContext{}, 2, ns, pool)
	executed := make(chan struct{}, 1)
	wp.Job("alpha", func(_ *Job) error {
		select {
		case executed <- struct{}{}:
		default:
		}
		return nil
	})

	// Register and pause BEFORE starting the worker pool.
	c := NewClient(ns, pool)
	makeKnownJobs(t, ns, pool, "alpha")
	_, err = c.PauseJob("alpha")
	assert.NoError(t, err)

	wp.Start()
	defer wp.Stop()

	select {
	case <-executed:
		t.Fatal("paused queue should not have been processed")
	case <-time.After(150 * time.Millisecond):
	}

	// Resume and the job should run.
	_, err = c.ResumeJob("alpha")
	assert.NoError(t, err)

	select {
	case <-executed:
	case <-time.After(2 * time.Second):
		t.Fatal("resumed queue should have been processed")
	}
}

func TestClient_ResetLockCount(t *testing.T) {
	pool := newTestPool(t)
	ns := "work"
	cleanKeyspace(ns, pool)

	makeKnownJobs(t, ns, pool, "alpha")

	conn := pool.Get()
	_, err := conn.Do("SET", redisKeyJobsLock(ns, "alpha"), 5)
	conn.Close()
	assert.NoError(t, err)

	c := NewClient(ns, pool)
	prev, err := c.ResetLockCount("alpha")
	assert.NoError(t, err)
	assert.EqualValues(t, 5, prev)

	v, ok := getRedisString(t, pool, redisKeyJobsLock(ns, "alpha"))
	assert.True(t, ok)
	assert.Equal(t, "0", v)

	_, err = c.ResetLockCount("nope")
	assert.True(t, errors.Is(err, ErrUnknownJob))
}

func TestClient_PurgeQueue(t *testing.T) {
	pool := newTestPool(t)
	ns := "work"
	cleanKeyspace(ns, pool)

	enq := NewEnqueuer(ns, pool)
	for range 4 {
		_, err := enq.Enqueue("alpha", nil)
		assert.NoError(t, err)
	}
	makeKnownJobs(t, ns, pool, "alpha")

	c := NewClient(ns, pool)
	purged, err := c.PurgeQueue("alpha")
	assert.NoError(t, err)
	assert.EqualValues(t, 4, purged)

	conn := pool.Get()
	defer conn.Close()
	llen, err := redis.Int64(conn.Do("LLEN", redisKeyJobs(ns, "alpha")))
	assert.NoError(t, err)
	assert.EqualValues(t, 0, llen)

	purged, err = c.PurgeQueue("alpha")
	assert.NoError(t, err)
	assert.EqualValues(t, 0, purged)

	_, err = c.PurgeQueue("nope")
	assert.True(t, errors.Is(err, ErrUnknownJob))
}

func TestClient_QueuesIncludesPaused(t *testing.T) {
	pool := newTestPool(t)
	ns := "work"
	cleanKeyspace(ns, pool)

	enq := NewEnqueuer(ns, pool)
	_, err := enq.Enqueue("alpha", nil)
	assert.NoError(t, err)
	_, err = enq.Enqueue("beta", nil)
	assert.NoError(t, err)
	makeKnownJobs(t, ns, pool, "alpha", "beta")

	c := NewClient(ns, pool)
	_, err = c.PauseJob("alpha")
	assert.NoError(t, err)

	queues, err := c.Queues()
	assert.NoError(t, err)
	assert.Equal(t, 2, len(queues))

	byName := map[string]*Queue{}
	for _, q := range queues {
		byName[q.JobName] = q
	}
	if assert.Contains(t, byName, "alpha") {
		assert.True(t, byName["alpha"].Paused)
	}
	if assert.Contains(t, byName, "beta") {
		assert.False(t, byName["beta"].Paused)
	}
}
