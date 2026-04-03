package work

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
)

var ErrReplicationFailed = errors.New("replication failed")

// Enqueuer can enqueue jobs.
type Enqueuer struct {
	Namespace string // eg, "myapp-work"
	Pool      *redis.Pool
	Option    EnqueuerOption

	queuePrefix           string // eg, "myapp-work:jobs:"
	knownJobs             map[string]int64
	enqueueUniqueScript   *redis.Script
	enqueueUniqueInScript *redis.Script
	mtx                   sync.RWMutex
}

// EnqueuerOption can be passed to NewEnqueuerWithOptions.
type EnqueuerOption struct {
	MinWaitReplicas  int // MinWaitReplicas is passed as numreplicas in redis wait command, if zero then skips wait command altogether
	MaxWaitTimeoutMS int // MaxWaitTimeoutMS is passed as timeout in redis wait command
}

// BulkEnqueueParam is a struct that specifies parameters for bulk enqueueing. See BulkEnqueue.
type BulkEnqueueParam struct {
	Name string
	Args map[string]any

	RunAtEpoch int64

	Unique       bool
	UniqueKeyMap map[string]any // Will only be used if Unique is true
}

// BulkEnqueueResult is a struct that specifies the result of bulk enqueueing for a single job. See BulkEnqueue.
type BulkEnqueueResult struct {
	ID             string
	EnqueuedAt     int64
	EnqueueSkipped bool
	UniqueKey      string
}

// NewEnqueuer creates a new enqueuer with the specified Redis namespace and Redis pool.
func NewEnqueuer(namespace string, pool *redis.Pool) *Enqueuer {
	return NewEnqueuerWithOptions(namespace, pool, EnqueuerOption{})
}

// NewEnqueuerWithOptions creates a new enqueuer with the specified Redis namespace, Redis pool and Enqueuer option.
func NewEnqueuerWithOptions(namespace string, pool *redis.Pool, opt EnqueuerOption) *Enqueuer {
	if pool == nil {
		panic("NewEnqueuer needs a non-nil *redis.Pool")
	}

	return &Enqueuer{
		Namespace:             namespace,
		Pool:                  pool,
		Option:                opt,
		queuePrefix:           redisKeyJobsPrefix(namespace),
		knownJobs:             make(map[string]int64),
		enqueueUniqueScript:   redis.NewScript(2, redisLuaEnqueueUnique),
		enqueueUniqueInScript: redis.NewScript(2, redisLuaEnqueueUniqueIn),
	}
}

// Enqueue will enqueue the specified job name and arguments. The args param can be nil if no args ar needed.
// Example: e.Enqueue("send_email", work.Q{"addr": "test@example.com"})
func (e *Enqueuer) Enqueue(jobName string, args map[string]any) (*Job, error) {
	job := &Job{
		Name:       jobName,
		ID:         makeIdentifier(),
		EnqueuedAt: nowEpochSeconds(),
		Args:       args,
	}

	rawJSON, err := job.serialize()
	if err != nil {
		return nil, err
	}

	conn := e.Pool.Get()
	defer conn.Close()

	if _, err := e.redisDoHelper(conn, "LPUSH", e.queuePrefix+jobName, rawJSON); err != nil {
		return nil, err
	}

	if err := e.addToKnownJobs(conn, jobName); err != nil {
		return job, err
	}

	return job, nil
}

// EnqueueIn enqueues a job in the scheduled job queue for execution in secondsFromNow seconds.
func (e *Enqueuer) EnqueueIn(jobName string, secondsFromNow int64, args map[string]any) (*ScheduledJob, error) {
	return e.EnqueueAt(jobName, epochAfterSeconds(secondsFromNow), args)
}

func (e *Enqueuer) EnqueueAt(jobName string, epochSeconds int64, args map[string]any) (*ScheduledJob, error) {
	job := &Job{
		Name:       jobName,
		ID:         makeIdentifier(),
		EnqueuedAt: nowEpochSeconds(),
		Args:       args,
	}
	rawJSON, err := job.serialize()
	if err != nil {
		return nil, err
	}

	conn := e.Pool.Get()
	defer conn.Close()

	scheduledJob := &ScheduledJob{
		RunAt: epochSeconds,
		Job:   job,
	}

	_, err = e.redisDoHelper(conn, "ZADD", redisKeyScheduled(e.Namespace), scheduledJob.RunAt, rawJSON)
	if err != nil {
		return nil, err
	}

	if err := e.addToKnownJobs(conn, jobName); err != nil {
		return scheduledJob, err
	}

	return scheduledJob, nil
}

// EnqueueUnique enqueues a job unless a job is already enqueued with the same name and arguments.
// The already-enqueued job can be in the normal work queue or in the scheduled job queue.
// Once a worker begins processing a job, another job with the same name and arguments can be enqueued again.
// Any failed jobs in the retry queue or dead queue don't count against the uniqueness -- so if a job fails and is retried, two unique jobs with the same name and arguments can be enqueued at once.
// In order to add robustness to the system, jobs are only unique for 24 hours after they're enqueued. This is mostly relevant for scheduled jobs.
// EnqueueUnique returns the job if it was enqueued and nil if it wasn't
func (e *Enqueuer) EnqueueUnique(jobName string, args map[string]any) (*Job, error) {
	return e.EnqueueUniqueByKey(jobName, args, nil)
}

// EnqueueUniqueIn enqueues a unique job in the scheduled job queue for execution in secondsFromNow seconds. See EnqueueUnique for the semantics of unique jobs.
func (e *Enqueuer) EnqueueUniqueIn(jobName string, secondsFromNow int64, args map[string]any) (*ScheduledJob, error) {
	return e.EnqueueUniqueInByKey(jobName, secondsFromNow, args, nil)
}

// EnqueueUniqueByKey enqueues a job unless a job is already enqueued with the same name and key, updating arguments.
// The already-enqueued job can be in the normal work queue or in the scheduled job queue.
// Once a worker begins processing a job, another job with the same name and key can be enqueued again.
// Any failed jobs in the retry queue or dead queue don't count against the uniqueness -- so if a job fails and is retried, two unique jobs with the same name and arguments can be enqueued at once.
// In order to add robustness to the system, jobs are only unique for 24 hours after they're enqueued. This is mostly relevant for scheduled jobs.
// EnqueueUniqueByKey returns the job if it was enqueued and nil if it wasn't
func (e *Enqueuer) EnqueueUniqueByKey(jobName string, args map[string]any, keyMap map[string]any) (*Job, error) {
	enqueue, job, err := e.uniqueJobHelper(jobName, args, keyMap)
	if err != nil {
		return nil, err
	}

	res, err := enqueue(nil)

	if res == "ok" && err == nil {
		return job, nil
	}
	return nil, err
}

// EnqueueUniqueInByKey enqueues a job in the scheduled job queue that is unique on specified key for execution in secondsFromNow seconds. See EnqueueUnique for the semantics of unique jobs.
// Subsequent calls with same key will update arguments
func (e *Enqueuer) EnqueueUniqueInByKey(jobName string, secondsFromNow int64, args map[string]any, keyMap map[string]any) (*ScheduledJob, error) {
	return e.EnqueueUniqueAtByKey(jobName, epochAfterSeconds(secondsFromNow), args, keyMap)
}

// EnqueueUniqueAt enqueues a unique job at the specified absolute epoch time in seconds. See EnqueueUnique for semantics of unique jobs.
func (e *Enqueuer) EnqueueUniqueAt(jobName string, epochSeconds int64, args map[string]any) (*ScheduledJob, error) {
	return e.EnqueueUniqueAtByKey(jobName, epochSeconds, args, nil)
}

// EnqueueUniqueAtByKey enqueues a job unique on specified key at the specified absolute epoch time in seconds, updating arguments. See EnqueueUnique for semantics of unique jobs.
func (e *Enqueuer) EnqueueUniqueAtByKey(jobName string, epochSeconds int64, args map[string]any, keyMap map[string]any) (*ScheduledJob, error) {
	enqueue, job, err := e.uniqueJobHelper(jobName, args, keyMap)
	if err != nil {
		return nil, err
	}

	scheduledJob := &ScheduledJob{
		RunAt: epochSeconds,
		Job:   job,
	}

	res, err := enqueue(&scheduledJob.RunAt)
	if res == "ok" && err == nil {
		return scheduledJob, nil
	}
	return nil, err
}

// BulkEnqueue is a more efficient way to enqueue many jobs at once.
// It takes in a slice of BulkEnqueueParam and returns a slice of BulkEnqueueResult.
// The order of the results corresponds to the order of the params.
func (e *Enqueuer) BulkEnqueue(params []BulkEnqueueParam) ([]BulkEnqueueResult, error) {
	jobs := make([]Job, len(params))
	results := make([]BulkEnqueueResult, len(jobs))
	var err error

	epochSeconds := nowEpochSeconds()
	for i, p := range params {
		jobs[i], results[i], err = p.buildJobAndResult(e.Namespace, epochSeconds)
		if err != nil {
			return nil, err
		}
	}

	c := e.Pool.Get()
	defer c.Close()

	for i := range params {
		p, j := params[i], jobs[i]

		switch {
		case p.Unique:
			script, args := e.bulkUniqueHelper(p, j)
			if err := script.SendHash(c, args...); err != nil {
				return nil, err
			}
		case p.RunAtEpoch != 0:
			if err := c.Send("ZADD", redisKeyScheduled(e.Namespace), p.RunAtEpoch, j.rawJSON); err != nil {
				return nil, err
			}
		default:
			if err := c.Send("LPUSH", e.queuePrefix+p.Name, j.rawJSON); err != nil {
				return nil, err
			}
		}
	}

	knownJobReceiver := e.sendBulkKnownJobs(c, params)

	if err := e.sendWait(c); err != nil {
		return nil, err
	}
	if err := c.Flush(); err != nil {
		return nil, err
	}

	var sendHashFailedIndices []int
	for i, j := range jobs {
		if !j.Unique {
			if _, err := c.Receive(); err != nil {
				return nil, err
			}
			continue
		}

		status, err := redis.String(c.Receive())
		if err != nil {
			if strings.Contains(err.Error(), "NOSCRIPT ") {
				sendHashFailedIndices = append(sendHashFailedIndices, i)
				continue
			}
			return nil, err
		}

		results[i].EnqueueSkipped = status != "ok"
	}

	if knownJobReceiver != nil {
		if err := knownJobReceiver(); err != nil {
			return nil, err
		}
	}

	if err := e.receiveWait(c); err != nil {
		return nil, err
	}

	if len(sendHashFailedIndices) == 0 {
		return results, nil
	}

	for _, i := range sendHashFailedIndices {
		p, j := params[i], jobs[i]
		script, args := e.bulkUniqueHelper(p, j)
		if err := script.Send(c, args...); err != nil {
			return nil, err
		}
	}
	if err := c.Flush(); err != nil {
		return nil, err
	}
	for _, i := range sendHashFailedIndices {
		status, err := redis.String(c.Receive())
		if err != nil {
			return nil, err
		}
		results[i].EnqueueSkipped = status != "ok"
	}

	return results, nil
}

func (e *Enqueuer) bulkUniqueHelper(p BulkEnqueueParam, j Job) (*redis.Script, []any) {
	var updatedArg any = "1" // skips updating args
	if p.UniqueKeyMap != nil {
		updatedArg = j.rawJSON
	}

	if p.RunAtEpoch != 0 {
		return e.enqueueUniqueInScript, []any{
			redisKeyScheduled(e.Namespace),
			j.UniqueKey,
			j.rawJSON,
			updatedArg,
			p.RunAtEpoch,
		}
	}

	return e.enqueueUniqueScript, []any{
		e.queuePrefix + j.Name,
		j.UniqueKey,
		j.rawJSON,
		updatedArg,
	}
}

func (e *Enqueuer) addToKnownJobs(conn redis.Conn, jobName string) error {
	needSadd := true
	now := time.Now().Unix()

	e.mtx.RLock()
	t, ok := e.knownJobs[jobName]
	e.mtx.RUnlock()

	if ok {
		if now < t {
			needSadd = false
		}
	}
	if needSadd {
		if _, err := conn.Do("SADD", redisKeyKnownJobs(e.Namespace), jobName); err != nil {
			return err
		}

		e.mtx.Lock()
		e.knownJobs[jobName] = now + 300
		e.mtx.Unlock()
	}

	return nil
}

func (e *Enqueuer) sendBulkKnownJobs(conn redis.Conn, params []BulkEnqueueParam) func() error {
	pendingJobNames := map[string]struct{}{}
	now := time.Now().Unix()
	e.mtx.RLock()
	for _, j := range params {
		t, ok := e.knownJobs[j.Name]
		if !ok || t < now {
			pendingJobNames[j.Name] = struct{}{}
		}
	}
	e.mtx.RUnlock()

	if len(pendingJobNames) == 0 {
		return nil
	}

	args := make([]any, 1, len(pendingJobNames)+1)
	args[0] = redisKeyKnownJobs(e.Namespace)
	for job := range pendingJobNames {
		args = append(args, job)
	}

	if err := conn.Send("SADD", args...); err != nil {
		return func() error {
			return err // defer returning error
		}
	}

	return func() error {
		if _, err := conn.Receive(); err != nil {
			return err
		}

		e.mtx.Lock()
		for jobName := range pendingJobNames {
			e.knownJobs[jobName] = now + 300
		}
		e.mtx.Unlock()
		return nil
	}
}

type enqueueFnType func(*int64) (string, error)

func (e *Enqueuer) uniqueJobHelper(jobName string, args map[string]any, keyMap map[string]any) (enqueueFnType, *Job, error) {
	useDefaultKeys := false
	if keyMap == nil {
		useDefaultKeys = true
		keyMap = args
	}

	uniqueKey, err := redisKeyUniqueJob(e.Namespace, jobName, keyMap)
	if err != nil {
		return nil, nil, err
	}

	job := &Job{
		Name:       jobName,
		ID:         makeIdentifier(),
		EnqueuedAt: nowEpochSeconds(),
		Args:       args,
		Unique:     true,
		UniqueKey:  uniqueKey,
	}

	rawJSON, err := job.serialize()
	if err != nil {
		return nil, nil, err
	}

	enqueueFn := func(runAt *int64) (string, error) {
		conn := e.Pool.Get()
		defer conn.Close()

		if err := e.addToKnownJobs(conn, jobName); err != nil {
			return "", err
		}

		scriptArgs := []any{}
		script := e.enqueueUniqueScript

		scriptArgs = append(scriptArgs, e.queuePrefix+jobName) // KEY[1]
		scriptArgs = append(scriptArgs, uniqueKey)             // KEY[2]
		scriptArgs = append(scriptArgs, rawJSON)               // ARGV[1]
		if useDefaultKeys {
			// keying on arguments so arguments can't be updated
			// we'll just get them off the original job so to save space, make this "1"
			scriptArgs = append(scriptArgs, "1") // ARGV[2]
		} else {
			// we'll use this for updated arguments since the job on the queue
			// doesn't get updated
			scriptArgs = append(scriptArgs, rawJSON) // ARGV[2]
		}

		if runAt != nil { // Scheduled job so different job queue with additional arg
			scriptArgs[0] = redisKeyScheduled(e.Namespace) // KEY[1]
			scriptArgs = append(scriptArgs, *runAt)        // ARGV[3]

			script = e.enqueueUniqueInScript
		}

		status, err := redis.String(script.Do(conn, scriptArgs...))
		if err != nil {
			return "", err
		}
		if e.MinWaitEnabled() {
			numReplicas, err := redis.Int(conn.Do("WAIT", e.Option.MinWaitReplicas, e.Option.MaxWaitTimeoutMS))
			if err != nil {
				return "", err
			}
			if numReplicas < e.Option.MinWaitReplicas {
				return "", ErrReplicationFailed
			}
		}
		return status, err
	}

	return enqueueFn, job, nil
}

func (e *Enqueuer) MinWaitEnabled() bool {
	return e.Option.MinWaitReplicas > 0
}

func (e *Enqueuer) redisDoHelper(c redis.Conn, cmdName string, args ...any) (reply any, err error) {
	if err = c.Send(cmdName, args...); err != nil {
		return
	}
	if err = e.sendWait(c); err != nil {
		return
	}

	c.Flush()

	reply, err = c.Receive()
	if err != nil {
		return
	}
	err = e.receiveWait(c)
	return
}

func (e *Enqueuer) sendWait(c redis.Conn) error {
	if !e.MinWaitEnabled() {
		return nil
	}

	return c.Send("WAIT", e.Option.MinWaitReplicas, e.Option.MaxWaitTimeoutMS)
}

func (e *Enqueuer) receiveWait(c redis.Conn) error {
	if !e.MinWaitEnabled() {
		return nil
	}

	numReplicas, err := redis.Int(c.Receive())
	if err != nil {
		return err
	}
	if numReplicas < e.Option.MinWaitReplicas {
		return ErrReplicationFailed
	}
	return nil
}

func (p BulkEnqueueParam) buildJobAndResult(namespace string, epochSeconds int64) (Job, BulkEnqueueResult, error) {
	uniqueKey, err := p.buildUniqueKey(namespace)
	if err != nil {
		return Job{}, BulkEnqueueResult{}, err
	}

	id := makeIdentifier()
	job := Job{
		Name:       p.Name,
		ID:         id,
		EnqueuedAt: epochSeconds,
		Args:       p.Args,
		Unique:     p.Unique,
		UniqueKey:  uniqueKey,
	}
	job.rawJSON, err = job.serialize()

	return job, BulkEnqueueResult{
		ID:         id,
		EnqueuedAt: epochSeconds,
		UniqueKey:  uniqueKey,
	}, err
}

func (p BulkEnqueueParam) buildUniqueKey(namespace string) (string, error) {
	if !p.Unique {
		return "", nil
	}
	args := p.Args
	if p.UniqueKeyMap != nil {
		args = p.UniqueKeyMap
	}

	return redisKeyUniqueJob(namespace, p.Name, args)
}
