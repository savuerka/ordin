package framework

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ScheduledFunc is executed by the scheduler.
type ScheduledFunc func(context.Context) error

// Scheduler runs in-process recurring jobs. For distributed deployments, guard
// jobs with Redis locks or run the scheduler in one worker process only.
type Scheduler struct {
	mu   sync.RWMutex
	jobs []*ScheduledJob
}

// NewScheduler creates an empty in-process scheduler.
func NewScheduler() *Scheduler {
	return &Scheduler{}
}

// ScheduledJob is a registered scheduler task.
type ScheduledJob struct {
	Name      string
	Trigger   Trigger
	Handler   ScheduledFunc
	Options   ScheduleOptions
	lastError error
	mu        sync.RWMutex
}

// LastError returns the last handler error, if any.
func (j *ScheduledJob) LastError() error {
	if j == nil {
		return nil
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.lastError
}

func (j *ScheduledJob) setLastError(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.lastError = err
}

// Trigger computes the next time a job should run after a given moment.
type Trigger interface {
	Next(after time.Time) time.Time
}

type intervalTrigger struct {
	interval time.Duration
}

func (t intervalTrigger) Next(after time.Time) time.Time {
	if t.interval <= 0 {
		return time.Time{}
	}
	return after.Add(t.interval)
}

type dailyTrigger struct {
	hour   int
	minute int
	second int
	loc    *time.Location
}

func (t dailyTrigger) Next(after time.Time) time.Time {
	loc := t.loc
	if loc == nil {
		loc = time.Local
	}
	now := after.In(loc)
	next := time.Date(now.Year(), now.Month(), now.Day(), t.hour, t.minute, t.second, 0, loc)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

// ScheduleOptions configures scheduled jobs.
type ScheduleOptions struct {
	RunImmediately bool
	Singleton      bool
	Timeout        time.Duration
	OnError        func(string, error)
}

// ScheduleOption updates ScheduleOptions.
type ScheduleOption func(*ScheduleOptions)

// RunImmediately starts the job once as soon as the scheduler starts.
func RunImmediately() ScheduleOption {
	return func(options *ScheduleOptions) {
		options.RunImmediately = true
	}
}

// Singleton prevents overlapping executions of the same job.
func Singleton() ScheduleOption {
	return func(options *ScheduleOptions) {
		options.Singleton = true
	}
}

// WithScheduleTimeout limits each job execution time.
func WithScheduleTimeout(timeout time.Duration) ScheduleOption {
	return func(options *ScheduleOptions) {
		options.Timeout = timeout
	}
}

// WithScheduleErrorHandler receives job errors without stopping the scheduler.
func WithScheduleErrorHandler(handler func(string, error)) ScheduleOption {
	return func(options *ScheduleOptions) {
		options.OnError = handler
	}
}

// Every registers a recurring job by interval.
func (s *Scheduler) Every(name string, interval time.Duration, handler ScheduledFunc, options ...ScheduleOption) *ScheduledJob {
	return s.Schedule(name, intervalTrigger{interval: interval}, handler, options...)
}

// DailyAt registers a daily job. Accepted time formats: HH:MM and HH:MM:SS.
func (s *Scheduler) DailyAt(name, at string, handler ScheduledFunc, options ...ScheduleOption) (*ScheduledJob, error) {
	parsed, err := parseDailyTime(at)
	if err != nil {
		return nil, err
	}
	return s.Schedule(name, dailyTrigger{hour: parsed[0], minute: parsed[1], second: parsed[2], loc: time.Local}, handler, options...), nil
}

// Schedule registers a job with a custom trigger.
func (s *Scheduler) Schedule(name string, trigger Trigger, handler ScheduledFunc, options ...ScheduleOption) *ScheduledJob {
	if s == nil {
		return nil
	}
	cfg := ScheduleOptions{Singleton: true}
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}
	job := &ScheduledJob{Name: name, Trigger: trigger, Handler: handler, Options: cfg}
	s.mu.Lock()
	s.jobs = append(s.jobs, job)
	s.mu.Unlock()
	return job
}

// Jobs returns a snapshot of registered jobs.
func (s *Scheduler) Jobs() []*ScheduledJob {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]*ScheduledJob, len(s.jobs))
	copy(jobs, s.jobs)
	return jobs
}

// Start runs registered jobs until ctx is cancelled. It blocks.
func (s *Scheduler) Start(ctx context.Context) error {
	if s == nil {
		return errors.New("scheduler is not configured")
	}
	jobs := s.Jobs()
	var wg sync.WaitGroup
	for _, job := range jobs {
		if job == nil || job.Handler == nil || job.Trigger == nil {
			continue
		}
		wg.Add(1)
		go func(job *ScheduledJob) {
			defer wg.Done()
			s.runJobLoop(ctx, job)
		}(job)
	}
	<-ctx.Done()
	wg.Wait()
	return ctx.Err()
}

// StartAsync starts the scheduler in the background and reports the final error.
func (s *Scheduler) StartAsync(ctx context.Context) <-chan error {
	errs := make(chan error, 1)
	go func() {
		defer close(errs)
		errs <- s.Start(ctx)
	}()
	return errs
}

func (s *Scheduler) runJobLoop(ctx context.Context, job *ScheduledJob) {
	var running sync.Mutex
	if job.Options.RunImmediately {
		s.executeJob(ctx, job, &running)
	}
	for {
		next := job.Trigger.Next(time.Now())
		if next.IsZero() {
			job.setLastError(errors.New("scheduler trigger returned zero time"))
			return
		}
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.executeJob(ctx, job, &running)
		}
	}
}

func (s *Scheduler) executeJob(ctx context.Context, job *ScheduledJob, running *sync.Mutex) {
	if job.Options.Singleton {
		running.Lock()
		defer running.Unlock()
	}

	runCtx := ctx
	cancel := func() {}
	if job.Options.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, job.Options.Timeout)
	}
	defer cancel()

	err := job.Handler(runCtx)
	job.setLastError(err)
	if err != nil && job.Options.OnError != nil {
		job.Options.OnError(job.Name, err)
	}
}

func parseDailyTime(value string) ([3]int, error) {
	var result [3]int
	for _, layout := range []string{"15:04:05", "15:04"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			result[0] = parsed.Hour()
			result[1] = parsed.Minute()
			result[2] = parsed.Second()
			return result, nil
		}
	}
	return result, fmt.Errorf("invalid daily time %q, expected HH:MM or HH:MM:SS", value)
}
