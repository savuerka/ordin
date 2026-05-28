package framework

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// PipelineFunc executes one pipeline stage.
type PipelineFunc func(*PipelineContext) error

// Pipeline is a sequential task pipeline. It is useful for file delivery,
// ETL-like jobs and queue/scheduler workflows.
type Pipeline struct {
	Name  string
	steps []PipelineStep
}

// NewPipeline creates a named pipeline.
func NewPipeline(name string) *Pipeline {
	return &Pipeline{Name: name}
}

// PipelineStep is one executable pipeline stage.
type PipelineStep struct {
	Name    string
	Handler PipelineFunc
	Options PipelineStepOptions
}

// PipelineStepOptions configures a pipeline step.
type PipelineStepOptions struct {
	Timeout         time.Duration
	Retries         int
	RetryDelay      time.Duration
	ContinueOnError bool
}

// PipelineStepOption updates PipelineStepOptions.
type PipelineStepOption func(*PipelineStepOptions)

// WithStepTimeout limits one pipeline step.
func WithStepTimeout(timeout time.Duration) PipelineStepOption {
	return func(options *PipelineStepOptions) {
		options.Timeout = timeout
	}
}

// WithStepRetries retries a failing step.
func WithStepRetries(retries int, delay time.Duration) PipelineStepOption {
	return func(options *PipelineStepOptions) {
		options.Retries = retries
		options.RetryDelay = delay
	}
}

// ContinueOnStepError records an error and continues to the next step.
func ContinueOnStepError() PipelineStepOption {
	return func(options *PipelineStepOptions) {
		options.ContinueOnError = true
	}
}

// Use appends a step and returns the same pipeline for fluent construction.
func (p *Pipeline) Use(name string, handler PipelineFunc, options ...PipelineStepOption) *Pipeline {
	if p == nil {
		return nil
	}
	cfg := PipelineStepOptions{}
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}
	p.steps = append(p.steps, PipelineStep{Name: name, Handler: handler, Options: cfg})
	return p
}

// Steps returns a snapshot of pipeline steps.
func (p *Pipeline) Steps() []PipelineStep {
	if p == nil {
		return nil
	}
	steps := make([]PipelineStep, len(p.steps))
	copy(steps, p.steps)
	return steps
}

// PipelineContext carries context and mutable data across pipeline steps.
type PipelineContext struct {
	context.Context
	Pipeline string
	Data     Data
	Events   []PipelineEvent
}

// PipelineEvent describes one finished pipeline step.
type PipelineEvent struct {
	Step     string
	Attempt  int
	Duration time.Duration
	Error    error
}

// Set stores a value in pipeline data.
func (c *PipelineContext) Set(key string, value any) {
	if c.Data == nil {
		c.Data = Data{}
	}
	c.Data[key] = value
}

// Get returns a value from pipeline data.
func (c *PipelineContext) Get(key string) (any, bool) {
	if c == nil || c.Data == nil {
		return nil, false
	}
	value, ok := c.Data[key]
	return value, ok
}

// String returns a string value from pipeline data.
func (c *PipelineContext) String(key string) string {
	value, ok := c.Get(key)
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

// Run executes the pipeline sequentially.
func (p *Pipeline) Run(ctx context.Context, data Data) (*PipelineContext, error) {
	if p == nil {
		return nil, errors.New("pipeline is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if data == nil {
		data = Data{}
	}
	pipeCtx := &PipelineContext{Context: ctx, Pipeline: p.Name, Data: data, Events: []PipelineEvent{}}

	for _, step := range p.steps {
		if step.Handler == nil {
			return pipeCtx, fmt.Errorf("pipeline step %q handler is nil", step.Name)
		}
		err := runPipelineStep(pipeCtx, step)
		if err != nil && !step.Options.ContinueOnError {
			return pipeCtx, err
		}
	}
	return pipeCtx, nil
}

func runPipelineStep(pipeCtx *PipelineContext, step PipelineStep) error {
	attempts := step.Options.Retries + 1
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := pipeCtx.Err(); err != nil {
			return err
		}
		stepCtx := pipeCtx.Context
		cancel := func() {}
		if step.Options.Timeout > 0 {
			stepCtx, cancel = context.WithTimeout(pipeCtx.Context, step.Options.Timeout)
		}
		current := *pipeCtx
		current.Context = stepCtx

		started := time.Now()
		err := step.Handler(&current)
		cancel()
		duration := time.Since(started)
		pipeCtx.Data = current.Data
		pipeCtx.Events = append(pipeCtx.Events, PipelineEvent{Step: step.Name, Attempt: attempt, Duration: duration, Error: err})
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < attempts && step.Options.RetryDelay > 0 {
			select {
			case <-pipeCtx.Done():
				return pipeCtx.Err()
			case <-time.After(step.Options.RetryDelay):
			}
		}
	}
	return fmt.Errorf("pipeline step %q failed: %w", step.Name, lastErr)
}
