package work

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rafaeljusto/redigomock"
	"github.com/stretchr/testify/assert"
)

var zero = int64(0)
var one = int64(1)
var two = int64(2)
var three = int64(3)

func TestEnqueue(t *testing.T) {
	pool := newTestPool(t)
	ns := "work"
	cleanKeyspace(ns, pool)
	enqueuer := NewEnqueuer(ns, pool)
	job, err := enqueuer.Enqueue("wat", Q{"a": 1, "b": "cool"})
	assert.Nil(t, err)
	assert.Equal(t, "wat", job.Name)
	assert.True(t, len(job.ID) > 10)                        // Something is in it
	assert.True(t, job.EnqueuedAt > (time.Now().Unix()-10)) // Within 10 seconds
	assert.True(t, job.EnqueuedAt < (time.Now().Unix()+10)) // Within 10 seconds
	assert.Equal(t, "cool", job.ArgString("b"))
	assert.EqualValues(t, 1, job.ArgInt64("a"))
	assert.NoError(t, job.ArgError())

	// Make sure "wat" is in the known jobs
	assert.EqualValues(t, []string{"wat"}, knownJobs(pool, redisKeyKnownJobs(ns)))

	// Make sure the cache is set
	expiresAt := enqueuer.knownJobs["wat"]
	assert.True(t, expiresAt > (time.Now().Unix()+290))

	// Make sure the length of the queue is 1
	assert.EqualValues(t, 1, listSize(pool, redisKeyJobs(ns, "wat")))

	// Get the job
	j := jobOnQueue(pool, redisKeyJobs(ns, "wat"))
	assert.Equal(t, "wat", j.Name)
	assert.True(t, len(j.ID) > 10)                        // Something is in it
	assert.True(t, j.EnqueuedAt > (time.Now().Unix()-10)) // Within 10 seconds
	assert.True(t, j.EnqueuedAt < (time.Now().Unix()+10)) // Within 10 seconds
	assert.Equal(t, "cool", j.ArgString("b"))
	assert.EqualValues(t, 1, j.ArgInt64("a"))
	assert.NoError(t, j.ArgError())

	// Now enqueue another job, make sure that we can enqueue multiple
	_, err = enqueuer.Enqueue("wat", Q{"a": 1, "b": "cool"})
	_, err = enqueuer.Enqueue("wat", Q{"a": 1, "b": "cool"})
	assert.Nil(t, err)
	assert.EqualValues(t, 2, listSize(pool, redisKeyJobs(ns, "wat")))
}

func TestEnqueue_WithMock(t *testing.T) {
	ns := "work"
	jobName := "test"
	jobArgs := map[string]interface{}{"arg": "value"}
	var cases = []struct {
		name           string
		enqueuerOption EnqueuerOption
		mockLpush      *int64
		mockLpushErr   error
		mockWait       *int64
		mockWaitErr    error

		expectedError error
	}{
		{
			name:      "Success without wait",
			mockLpush: &one,
		}, {
			name:          "Failure without wait",
			mockLpushErr:  errors.New("lpush failure"),
			expectedError: errors.New("lpush failure"),
		}, {
			name: "Failure with wait",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLpush:     &one,
			mockWaitErr:   errors.New("wait failure"),
			expectedError: errors.New("wait failure"),
		}, {
			name: "When wait return zero",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLpush:     &one,
			mockWait:      &zero,
			expectedError: ErrReplicationFailed,
		}, {
			name: "When wait return less than MinWaitReplicas",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLpush:     &one,
			mockWait:      &one,
			expectedError: ErrReplicationFailed,
		}, {
			name: "When wait return same as MinWaitReplicas",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLpush: &one,
			mockWait:  &two,
		}, {
			name: "When wait return more than MinWaitReplicas",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLpush: &one,
			mockWait:  &three,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			pool, conn := newMockTestPool(t)
			enqueuer := NewEnqueuerWithOptions(ns, pool, tt.enqueuerOption)
			if tt.mockLpush != nil {
				conn.Command("LPUSH", "work:jobs:test", redigomock.NewAnyData()).Expect(*tt.mockLpush)
			}
			if tt.mockLpushErr != nil {
				conn.Command("LPUSH", "work:jobs:test", redigomock.NewAnyData()).ExpectError(tt.mockLpushErr)
			}
			if tt.mockWait != nil {
				conn.Command("WAIT", tt.enqueuerOption.MinWaitReplicas, tt.enqueuerOption.MaxWaitTimeoutMS).Expect(*tt.mockWait)
			}
			if tt.mockWaitErr != nil {
				conn.Command("WAIT", tt.enqueuerOption.MinWaitReplicas, tt.enqueuerOption.MaxWaitTimeoutMS).ExpectError(tt.mockWaitErr)
			}
			conn.Command("SADD", "work:known_jobs", jobName).Expect(1)

			_, err := enqueuer.Enqueue(jobName, jobArgs)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestEnqueueIn(t *testing.T) {
	pool := newTestPool(t)
	ns := "work"
	cleanKeyspace(ns, pool)
	enqueuer := NewEnqueuer(ns, pool)

	// Set to expired value to make sure we update the set of known jobs
	enqueuer.knownJobs["wat"] = 4

	job, err := enqueuer.EnqueueIn("wat", 300, Q{"a": 1, "b": "cool"})
	assert.Nil(t, err)
	if assert.NotNil(t, job) {
		assert.Equal(t, "wat", job.Name)
		assert.True(t, len(job.ID) > 10)                        // Something is in it
		assert.True(t, job.EnqueuedAt > (time.Now().Unix()-10)) // Within 10 seconds
		assert.True(t, job.EnqueuedAt < (time.Now().Unix()+10)) // Within 10 seconds
		assert.Equal(t, "cool", job.ArgString("b"))
		assert.EqualValues(t, 1, job.ArgInt64("a"))
		assert.NoError(t, job.ArgError())
		assert.EqualValues(t, job.EnqueuedAt+300, job.RunAt)
	}

	// Make sure "wat" is in the known jobs
	assert.EqualValues(t, []string{"wat"}, knownJobs(pool, redisKeyKnownJobs(ns)))

	// Make sure the cache is set
	expiresAt := enqueuer.knownJobs["wat"]
	assert.True(t, expiresAt > (time.Now().Unix()+290))

	// Make sure the length of the scheduled job queue is 1
	assert.EqualValues(t, 1, zsetSize(pool, redisKeyScheduled(ns)))

	// Get the job
	score, j := jobOnZset(pool, redisKeyScheduled(ns))

	assert.True(t, score > time.Now().Unix()+290)
	assert.True(t, score <= time.Now().Unix()+300)

	assert.Equal(t, "wat", j.Name)
	assert.True(t, len(j.ID) > 10)                        // Something is in it
	assert.True(t, j.EnqueuedAt > (time.Now().Unix()-10)) // Within 10 seconds
	assert.True(t, j.EnqueuedAt < (time.Now().Unix()+10)) // Within 10 seconds
	assert.Equal(t, "cool", j.ArgString("b"))
	assert.EqualValues(t, 1, j.ArgInt64("a"))
	assert.NoError(t, j.ArgError())
}

func TestEnqueueIn_WithMock(t *testing.T) {
	ns := "work"
	jobName := "test"
	jobArgs := map[string]interface{}{"arg": "value"}
	secondsFromNow := int64(100)
	now := time.Now().Unix()
	setNowEpochSecondsMock(now)
	defer resetNowEpochSecondsMock()
	var cases = []struct {
		name           string
		enqueuerOption EnqueuerOption
		mockZadd       *int64
		mockZaddErr    error
		mockWait       *int64
		mockWaitErr    error

		expectedError error
	}{
		{
			name:     "Success without wait",
			mockZadd: &one,
		}, {
			name:          "Failure without wait",
			mockZaddErr:   errors.New("lpush failure"),
			expectedError: errors.New("lpush failure"),
		}, {
			name: "Failure with wait",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockZadd:      &one,
			mockWaitErr:   errors.New("wait failure"),
			expectedError: errors.New("wait failure"),
		}, {
			name: "When wait return zero",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockZadd:      &one,
			mockWait:      &zero,
			expectedError: ErrReplicationFailed,
		}, {
			name: "When wait return less than MinWaitReplicas",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockZadd:      &one,
			mockWait:      &one,
			expectedError: ErrReplicationFailed,
		}, {
			name: "When wait return same as MinWaitReplicas",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockZadd: &one,
			mockWait: &two,
		}, {
			name: "When wait return more than MinWaitReplicas",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockZadd: &one,
			mockWait: &three,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			pool, conn := newMockTestPool(t)
			enqueuer := NewEnqueuerWithOptions(ns, pool, tt.enqueuerOption)
			if tt.mockZadd != nil {
				conn.Command("ZADD", "work:scheduled", now+secondsFromNow, redigomock.NewAnyData()).Expect(*tt.mockZadd)
			}
			if tt.mockZaddErr != nil {
				conn.Command("ZADD", "work:scheduled", now+secondsFromNow, redigomock.NewAnyData()).ExpectError(tt.mockZaddErr)
			}
			if tt.mockWait != nil {
				conn.Command("WAIT", tt.enqueuerOption.MinWaitReplicas, tt.enqueuerOption.MaxWaitTimeoutMS).Expect(*tt.mockWait)
			}
			if tt.mockWaitErr != nil {
				conn.Command("WAIT", tt.enqueuerOption.MinWaitReplicas, tt.enqueuerOption.MaxWaitTimeoutMS).ExpectError(tt.mockWaitErr)
			}
			conn.Command("SADD", "work:known_jobs", jobName).Expect(1)

			_, err := enqueuer.EnqueueIn(jobName, secondsFromNow, jobArgs)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestEnqueueUnique(t *testing.T) {
	pool := newTestPool(t)
	ns := "work"
	cleanKeyspace(ns, pool)
	enqueuer := NewEnqueuer(ns, pool)
	var mutex = &sync.Mutex{}
	job, err := enqueuer.EnqueueUnique("wat", Q{"a": 1, "b": "cool"})
	assert.NoError(t, err)
	if assert.NotNil(t, job) {
		assert.Equal(t, "wat", job.Name)
		assert.True(t, len(job.ID) > 10)                        // Something is in it
		assert.True(t, job.EnqueuedAt > (time.Now().Unix()-10)) // Within 10 seconds
		assert.True(t, job.EnqueuedAt < (time.Now().Unix()+10)) // Within 10 seconds
		assert.Equal(t, "cool", job.ArgString("b"))
		assert.EqualValues(t, 1, job.ArgInt64("a"))
		assert.NoError(t, job.ArgError())
	}

	job, err = enqueuer.EnqueueUnique("wat", Q{"a": 1, "b": "cool"})
	assert.NoError(t, err)
	assert.Nil(t, job)

	job, err = enqueuer.EnqueueUnique("wat", Q{"a": 1, "b": "coolio"})
	assert.NoError(t, err)
	assert.NotNil(t, job)

	job, err = enqueuer.EnqueueUnique("wat", nil)
	assert.NoError(t, err)
	assert.NotNil(t, job)

	job, err = enqueuer.EnqueueUnique("wat", nil)
	assert.NoError(t, err)
	assert.Nil(t, job)

	job, err = enqueuer.EnqueueUnique("taw", nil)
	assert.NoError(t, err)
	assert.NotNil(t, job)

	// Process the queues. Ensure the right number of jobs were processed
	var wats, taws int64
	wp := NewWorkerPool(TestContext{}, 3, ns, pool)
	wp.JobWithOptions("wat", JobOptions{Priority: 1, MaxFails: 1}, func(job *Job) error {
		mutex.Lock()
		wats++
		mutex.Unlock()
		return nil
	})
	wp.JobWithOptions("taw", JobOptions{Priority: 1, MaxFails: 1}, func(job *Job) error {
		mutex.Lock()
		taws++
		mutex.Unlock()
		return fmt.Errorf("ohno")
	})
	wp.Start()
	wp.Drain()
	wp.Stop()

	assert.EqualValues(t, 3, wats)
	assert.EqualValues(t, 1, taws)

	// Enqueue again. Ensure we can.
	job, err = enqueuer.EnqueueUnique("wat", Q{"a": 1, "b": "cool"})
	assert.NoError(t, err)
	assert.NotNil(t, job)

	job, err = enqueuer.EnqueueUnique("wat", Q{"a": 1, "b": "coolio"})
	assert.NoError(t, err)
	assert.NotNil(t, job)

	// Even though taw resulted in an error, we should still be able to re-queue it.
	// This could result in multiple taws enqueued at the same time in a production system.
	job, err = enqueuer.EnqueueUnique("taw", nil)
	assert.NoError(t, err)
	assert.NotNil(t, job)
}

func TestEnqueueUnique_WithMock(t *testing.T) {
	ns := "work"
	jobName := "test"
	jobArgs := map[string]interface{}{"arg": "value"}

	ok := "ok"
	dup := "ok"
	var cases = []struct {
		name            string
		enqueuerOption  EnqueuerOption
		mockLEvalsha    *string
		mockLEvalshaErr error
		mockWait        *int64
		mockWaitErr     error

		expectedError error
	}{
		{
			name:         "Success without wait",
			mockLEvalsha: &ok,
		}, {
			name:         "Duplicate without wait",
			mockLEvalsha: &dup,
		}, {
			name:            "Failure without wait",
			mockLEvalshaErr: errors.New("lpush failure"),
			expectedError:   errors.New("lpush failure"),
		}, {
			name: "Failure with wait",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha:  &ok,
			mockWaitErr:   errors.New("wait failure"),
			expectedError: errors.New("wait failure"),
		}, {
			name: "When wait return zero",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha:  &dup,
			mockWait:      &zero,
			expectedError: ErrReplicationFailed,
		}, {
			name: "When wait return less than MinWaitReplicas",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha:  &ok,
			mockWait:      &one,
			expectedError: ErrReplicationFailed,
		}, {
			name: "When wait return same as MinWaitReplicas",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha: &dup,
			mockWait:     &two,
		}, {
			name: "When wait return more than MinWaitReplicas",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha: &ok,
			mockWait:     &three,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			pool, conn := newMockTestPool(t)
			enqueuer := NewEnqueuerWithOptions(ns, pool, tt.enqueuerOption)
			uniqueKey := `work:unique:test:{"arg":"value"}
`
			if tt.mockLEvalsha != nil {
				conn.Command("EVALSHA", "f38b6aef74017e799294b1ec4b74eb707deb0c17", 2, "work:jobs:test", uniqueKey, redigomock.NewAnyData(), "1").Expect(*tt.mockLEvalsha)
			}
			if tt.mockLEvalshaErr != nil {
				conn.Command("EVALSHA", "f38b6aef74017e799294b1ec4b74eb707deb0c17", 2, "work:jobs:test", uniqueKey, redigomock.NewAnyData(), "1").ExpectError(tt.mockLEvalshaErr)
			}
			if tt.mockWait != nil {
				conn.Command("WAIT", tt.enqueuerOption.MinWaitReplicas, tt.enqueuerOption.MaxWaitTimeoutMS).Expect(*tt.mockWait)
			}
			if tt.mockWaitErr != nil {
				conn.Command("WAIT", tt.enqueuerOption.MinWaitReplicas, tt.enqueuerOption.MaxWaitTimeoutMS).ExpectError(tt.mockWaitErr)
			}
			conn.Command("SADD", "work:known_jobs", jobName).Expect(1)

			_, err := enqueuer.EnqueueUnique(jobName, jobArgs)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestEnqueueUniqueIn(t *testing.T) {
	pool := newTestPool(t)
	ns := "work"
	cleanKeyspace(ns, pool)
	enqueuer := NewEnqueuer(ns, pool)

	// Enqueue two unique jobs -- ensure one job sticks.
	job, err := enqueuer.EnqueueUniqueIn("wat", 300, Q{"a": 1, "b": "cool"})
	assert.NoError(t, err)
	if assert.NotNil(t, job) {
		assert.Equal(t, "wat", job.Name)
		assert.True(t, len(job.ID) > 10)                        // Something is in it
		assert.True(t, job.EnqueuedAt > (time.Now().Unix()-10)) // Within 10 seconds
		assert.True(t, job.EnqueuedAt < (time.Now().Unix()+10)) // Within 10 seconds
		assert.Equal(t, "cool", job.ArgString("b"))
		assert.EqualValues(t, 1, job.ArgInt64("a"))
		assert.NoError(t, job.ArgError())
		assert.EqualValues(t, job.EnqueuedAt+300, job.RunAt)
	}

	job, err = enqueuer.EnqueueUniqueIn("wat", 10, Q{"a": 1, "b": "cool"})
	assert.NoError(t, err)
	assert.Nil(t, job)

	// Get the job
	score, j := jobOnZset(pool, redisKeyScheduled(ns))

	assert.True(t, score > time.Now().Unix()+290) // We don't want to overwrite the time
	assert.True(t, score <= time.Now().Unix()+300)

	assert.Equal(t, "wat", j.Name)
	assert.True(t, len(j.ID) > 10)                        // Something is in it
	assert.True(t, j.EnqueuedAt > (time.Now().Unix()-10)) // Within 10 seconds
	assert.True(t, j.EnqueuedAt < (time.Now().Unix()+10)) // Within 10 seconds
	assert.Equal(t, "cool", j.ArgString("b"))
	assert.EqualValues(t, 1, j.ArgInt64("a"))
	assert.NoError(t, j.ArgError())
	assert.True(t, j.Unique)

	// Now try to enqueue more stuff and ensure it
	job, err = enqueuer.EnqueueUniqueIn("wat", 300, Q{"a": 1, "b": "coolio"})
	assert.NoError(t, err)
	assert.NotNil(t, job)

	job, err = enqueuer.EnqueueUniqueIn("wat", 300, nil)
	assert.NoError(t, err)
	assert.NotNil(t, job)

	job, err = enqueuer.EnqueueUniqueIn("wat", 300, nil)
	assert.NoError(t, err)
	assert.Nil(t, job)

	job, err = enqueuer.EnqueueUniqueIn("taw", 300, nil)
	assert.NoError(t, err)
	assert.NotNil(t, job)
}

func TestEnqueueUniqueIn_WithMock(t *testing.T) {
	ns := "work"
	jobName := "test"
	jobArgs := map[string]interface{}{"arg": "value"}
	secondsFromNow := int64(100)
	now := time.Now().Unix()
	setNowEpochSecondsMock(now)
	defer resetNowEpochSecondsMock()

	ok := "ok"
	dup := "ok"
	var cases = []struct {
		name            string
		enqueuerOption  EnqueuerOption
		mockLEvalsha    *string
		mockLEvalshaErr error
		mockWait        *int64
		mockWaitErr     error

		expectedError error
	}{
		{
			name:         "Success without wait",
			mockLEvalsha: &ok,
		}, {
			name:         "Duplicate without wait",
			mockLEvalsha: &dup,
		}, {
			name:            "Failure without wait",
			mockLEvalshaErr: errors.New("lpush failure"),
			expectedError:   errors.New("lpush failure"),
		}, {
			name: "Failure with wait",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha:  &ok,
			mockWaitErr:   errors.New("wait failure"),
			expectedError: errors.New("wait failure"),
		}, {
			name: "When wait return zero",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha:  &dup,
			mockWait:      &zero,
			expectedError: ErrReplicationFailed,
		}, {
			name: "When wait return less than MinWaitReplicas",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha:  &ok,
			mockWait:      &one,
			expectedError: ErrReplicationFailed,
		}, {
			name: "When wait return same as MinWaitReplicas",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha: &dup,
			mockWait:     &two,
		}, {
			name: "When wait return more than MinWaitReplicas",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha: &ok,
			mockWait:     &three,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			pool, conn := newMockTestPool(t)
			enqueuer := NewEnqueuerWithOptions(ns, pool, tt.enqueuerOption)
			uniqueKey := `work:unique:test:{"arg":"value"}
`
			if tt.mockLEvalsha != nil {
				conn.Command("EVALSHA", "7b32230026d2ba0d5aa0b5451237f6c086e3072c", 2, "work:scheduled", uniqueKey, redigomock.NewAnyData(), "1", now+secondsFromNow).Expect(*tt.mockLEvalsha)
			}
			if tt.mockLEvalshaErr != nil {
				conn.Command("EVALSHA", "7b32230026d2ba0d5aa0b5451237f6c086e3072c", 2, "work:scheduled", uniqueKey, redigomock.NewAnyData(), "1", now+secondsFromNow).ExpectError(tt.mockLEvalshaErr)
			}
			if tt.mockWait != nil {
				conn.Command("WAIT", tt.enqueuerOption.MinWaitReplicas, tt.enqueuerOption.MaxWaitTimeoutMS).Expect(*tt.mockWait)
			}
			if tt.mockWaitErr != nil {
				conn.Command("WAIT", tt.enqueuerOption.MinWaitReplicas, tt.enqueuerOption.MaxWaitTimeoutMS).ExpectError(tt.mockWaitErr)
			}
			conn.Command("SADD", "work:known_jobs", jobName).Expect(1)

			_, err := enqueuer.EnqueueUniqueIn(jobName, secondsFromNow, jobArgs)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestEnqueueUniqueByKey(t *testing.T) {
	var arg3 string
	var arg4 string

	pool := newTestPool(t)
	ns := "work"
	cleanKeyspace(ns, pool)
	enqueuer := NewEnqueuer(ns, pool)
	var mutex = &sync.Mutex{}
	job, err := enqueuer.EnqueueUniqueByKey("wat", Q{"a": 3, "b": "foo"}, Q{"key": "123"})
	assert.NoError(t, err)
	if assert.NotNil(t, job) {
		assert.Equal(t, "wat", job.Name)
		assert.True(t, len(job.ID) > 10)                        // Something is in it
		assert.True(t, job.EnqueuedAt > (time.Now().Unix()-10)) // Within 10 seconds
		assert.True(t, job.EnqueuedAt < (time.Now().Unix()+10)) // Within 10 seconds
		assert.Equal(t, "foo", job.ArgString("b"))
		assert.EqualValues(t, 3, job.ArgInt64("a"))
		assert.NoError(t, job.ArgError())
	}

	job, err = enqueuer.EnqueueUniqueByKey("wat", Q{"a": 3, "b": "bar"}, Q{"key": "123"})
	assert.NoError(t, err)
	assert.Nil(t, job)

	job, err = enqueuer.EnqueueUniqueByKey("wat", Q{"a": 4, "b": "baz"}, Q{"key": "124"})
	assert.NoError(t, err)
	assert.NotNil(t, job)

	job, err = enqueuer.EnqueueUniqueByKey("taw", nil, Q{"key": "125"})
	assert.NoError(t, err)
	assert.NotNil(t, job)

	// Process the queues. Ensure the right number of jobs were processed
	var wats, taws int64
	wp := NewWorkerPool(TestContext{}, 3, ns, pool)
	wp.JobWithOptions("wat", JobOptions{Priority: 1, MaxFails: 1}, func(job *Job) error {
		mutex.Lock()
		argA := job.Args["a"].(float64)
		argB := job.Args["b"].(string)
		if argA == 3 {
			arg3 = argB
		}
		if argA == 4 {
			arg4 = argB
		}

		wats++
		mutex.Unlock()
		return nil
	})
	wp.JobWithOptions("taw", JobOptions{Priority: 1, MaxFails: 1}, func(job *Job) error {
		mutex.Lock()
		taws++
		mutex.Unlock()
		return fmt.Errorf("ohno")
	})
	wp.Start()
	wp.Drain()
	wp.Stop()

	assert.EqualValues(t, 2, wats)
	assert.EqualValues(t, 1, taws)

	// Check that arguments got updated to new value
	assert.EqualValues(t, "bar", arg3)
	assert.EqualValues(t, "baz", arg4)

	// Enqueue again. Ensure we can.
	job, err = enqueuer.EnqueueUniqueByKey("wat", Q{"a": 1, "b": "cool"}, Q{"key": "123"})
	assert.NoError(t, err)
	assert.NotNil(t, job)

	job, err = enqueuer.EnqueueUniqueByKey("wat", Q{"a": 1, "b": "coolio"}, Q{"key": "124"})
	assert.NoError(t, err)
	assert.NotNil(t, job)

	// Even though taw resulted in an error, we should still be able to re-queue it.
	// This could result in multiple taws enqueued at the same time in a production system.
	job, err = enqueuer.EnqueueUniqueByKey("taw", nil, Q{"key": "123"})
	assert.NoError(t, err)
	assert.NotNil(t, job)
}

func TestEnqueueUniqueByKey_WithMock(t *testing.T) {
	ns := "work"
	jobName := "test"
	jobArgs := map[string]interface{}{"arg": "value"}
	jobKeyMap := map[string]interface{}{"key": "value"}

	ok := "ok"
	dup := "ok"
	var cases = []struct {
		name            string
		enqueuerOption  EnqueuerOption
		mockLEvalsha    *string
		mockLEvalshaErr error
		mockWait        *int64
		mockWaitErr     error

		expectedError error
	}{
		{
			name:         "Success without wait",
			mockLEvalsha: &ok,
		}, {
			name:         "Duplicate without wait",
			mockLEvalsha: &dup,
		}, {
			name:            "Failure without wait",
			mockLEvalshaErr: errors.New("lpush failure"),
			expectedError:   errors.New("lpush failure"),
		}, {
			name: "Failure with wait",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha:  &ok,
			mockWaitErr:   errors.New("wait failure"),
			expectedError: errors.New("wait failure"),
		}, {
			name: "When wait return zero",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha:  &dup,
			mockWait:      &zero,
			expectedError: ErrReplicationFailed,
		}, {
			name: "When wait return less than MinWaitReplicas",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha:  &ok,
			mockWait:      &one,
			expectedError: ErrReplicationFailed,
		}, {
			name: "When wait return same as MinWaitReplicas",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha: &dup,
			mockWait:     &two,
		}, {
			name: "When wait return more than MinWaitReplicas",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha: &ok,
			mockWait:     &three,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			pool, conn := newMockTestPool(t)
			enqueuer := NewEnqueuerWithOptions(ns, pool, tt.enqueuerOption)
			uniqueKey := `work:unique:test:{"key":"value"}
`
			if tt.mockLEvalsha != nil {
				conn.Command("EVALSHA", "f38b6aef74017e799294b1ec4b74eb707deb0c17", 2, "work:jobs:test", uniqueKey, redigomock.NewAnyData(), redigomock.NewAnyData()).Expect(*tt.mockLEvalsha)
			}
			if tt.mockLEvalshaErr != nil {
				conn.Command("EVALSHA", "f38b6aef74017e799294b1ec4b74eb707deb0c17", 2, "work:jobs:test", uniqueKey, redigomock.NewAnyData(), redigomock.NewAnyData()).ExpectError(tt.mockLEvalshaErr)
			}
			if tt.mockWait != nil {
				conn.Command("WAIT", tt.enqueuerOption.MinWaitReplicas, tt.enqueuerOption.MaxWaitTimeoutMS).Expect(*tt.mockWait)
			}
			if tt.mockWaitErr != nil {
				conn.Command("WAIT", tt.enqueuerOption.MinWaitReplicas, tt.enqueuerOption.MaxWaitTimeoutMS).ExpectError(tt.mockWaitErr)
			}
			conn.Command("SADD", "work:known_jobs", jobName).Expect(1)

			_, err := enqueuer.EnqueueUniqueByKey(jobName, jobArgs, jobKeyMap)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestEnqueueUniqueInByKey(t *testing.T) {
	pool := newTestPool(t)
	ns := "work"
	cleanKeyspace(ns, pool)
	enqueuer := NewEnqueuer(ns, pool)

	// Enqueue two unique jobs -- ensure one job sticks.
	job, err := enqueuer.EnqueueUniqueInByKey("wat", 300, Q{"a": 1, "b": "cool"}, Q{"key": "123"})
	assert.NoError(t, err)
	if assert.NotNil(t, job) {
		assert.Equal(t, "wat", job.Name)
		assert.True(t, len(job.ID) > 10)                        // Something is in it
		assert.True(t, job.EnqueuedAt > (time.Now().Unix()-10)) // Within 10 seconds
		assert.True(t, job.EnqueuedAt < (time.Now().Unix()+10)) // Within 10 seconds
		assert.Equal(t, "cool", job.ArgString("b"))
		assert.EqualValues(t, 1, job.ArgInt64("a"))
		assert.NoError(t, job.ArgError())
		assert.EqualValues(t, job.EnqueuedAt+300, job.RunAt)
	}

	job, err = enqueuer.EnqueueUniqueInByKey("wat", 10, Q{"a": 1, "b": "cool"}, Q{"key": "123"})
	assert.NoError(t, err)
	assert.Nil(t, job)

	// Get the job
	score, j := jobOnZset(pool, redisKeyScheduled(ns))

	assert.True(t, score > time.Now().Unix()+290) // We don't want to overwrite the time
	assert.True(t, score <= time.Now().Unix()+300)

	assert.Equal(t, "wat", j.Name)
	assert.True(t, len(j.ID) > 10)                        // Something is in it
	assert.True(t, j.EnqueuedAt > (time.Now().Unix()-10)) // Within 10 seconds
	assert.True(t, j.EnqueuedAt < (time.Now().Unix()+10)) // Within 10 seconds
	assert.Equal(t, "cool", j.ArgString("b"))
	assert.EqualValues(t, 1, j.ArgInt64("a"))
	assert.NoError(t, j.ArgError())
	assert.True(t, j.Unique)
}

func TestEnqueueUniqueInByKey_WithMock(t *testing.T) {
	ns := "work"
	jobName := "test"
	jobArgs := map[string]interface{}{"arg": "value"}
	jobKeyMap := map[string]interface{}{"key": "value"}
	secondsFromNow := int64(100)
	now := time.Now().Unix()
	setNowEpochSecondsMock(now)
	defer resetNowEpochSecondsMock()

	ok := "ok"
	dup := "ok"
	var cases = []struct {
		name            string
		enqueuerOption  EnqueuerOption
		mockLEvalsha    *string
		mockLEvalshaErr error
		mockWait        *int64
		mockWaitErr     error

		expectedError error
	}{
		{
			name:         "Success without wait",
			mockLEvalsha: &ok,
		}, {
			name:         "Duplicate without wait",
			mockLEvalsha: &dup,
		}, {
			name:            "Failure without wait",
			mockLEvalshaErr: errors.New("lpush failure"),
			expectedError:   errors.New("lpush failure"),
		}, {
			name: "Failure with wait",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha:  &ok,
			mockWaitErr:   errors.New("wait failure"),
			expectedError: errors.New("wait failure"),
		}, {
			name: "When wait return zero",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha:  &dup,
			mockWait:      &zero,
			expectedError: ErrReplicationFailed,
		}, {
			name: "When wait return less than MinWaitReplicas",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha:  &ok,
			mockWait:      &one,
			expectedError: ErrReplicationFailed,
		}, {
			name: "When wait return same as MinWaitReplicas",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha: &dup,
			mockWait:     &two,
		}, {
			name: "When wait return more than MinWaitReplicas",
			enqueuerOption: EnqueuerOption{
				MinWaitReplicas:  2,
				MaxWaitTimeoutMS: 1000,
			},
			mockLEvalsha: &ok,
			mockWait:     &three,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			pool, conn := newMockTestPool(t)
			enqueuer := NewEnqueuerWithOptions(ns, pool, tt.enqueuerOption)
			uniqueKey := `work:unique:test:{"key":"value"}
`
			if tt.mockLEvalsha != nil {
				conn.Command("EVALSHA", "7b32230026d2ba0d5aa0b5451237f6c086e3072c", 2, "work:scheduled", uniqueKey, redigomock.NewAnyData(), redigomock.NewAnyData(), now+secondsFromNow).Expect(*tt.mockLEvalsha)
			}
			if tt.mockLEvalshaErr != nil {
				conn.Command("EVALSHA", "7b32230026d2ba0d5aa0b5451237f6c086e3072c", 2, "work:scheduled", uniqueKey, redigomock.NewAnyData(), redigomock.NewAnyData(), now+secondsFromNow).ExpectError(tt.mockLEvalshaErr)
			}
			if tt.mockWait != nil {
				conn.Command("WAIT", tt.enqueuerOption.MinWaitReplicas, tt.enqueuerOption.MaxWaitTimeoutMS).Expect(*tt.mockWait)
			}
			if tt.mockWaitErr != nil {
				conn.Command("WAIT", tt.enqueuerOption.MinWaitReplicas, tt.enqueuerOption.MaxWaitTimeoutMS).ExpectError(tt.mockWaitErr)
			}
			conn.Command("SADD", "work:known_jobs", jobName).Expect(1)

			_, err := enqueuer.EnqueueUniqueInByKey(jobName, secondsFromNow, jobArgs, jobKeyMap)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}
