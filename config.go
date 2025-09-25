package asynclog

import (
	"time"
)

// BackpressureStrategy defines how to handle buffer overflow
type BackpressureStrategy int

const (
	// Block waits until buffer has space
	Block BackpressureStrategy = iota
	// Drop discards new messages when buffer is full
	Drop
)

// Config holds configuration for the async logger
type Config struct {
	// BufferSize is the maximum number of messages to buffer in memory
	BufferSize int

	// MaxGoroutines is the maximum number of goroutines in the worker pool
	MaxGoroutines int

	// BackpressureStrategy determines behavior when buffer is full
	BackpressureStrategy BackpressureStrategy

	// MaxRetries is the maximum number of retry attempts for failed writes
	MaxRetries int

	// RetryDelay is the delay between retry attempts
	RetryDelay time.Duration

	// FlushInterval is how often to flush buffered messages
	FlushInterval time.Duration

	// Sinks contains the output sinks for log messages
	Sinks []Sink

	// Hooks contains event hooks for telemetry
	Hooks Hooks
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		BufferSize:           1000,
		MaxGoroutines:        10,
		BackpressureStrategy: Block,
		MaxRetries:           3,
		RetryDelay:          100 * time.Millisecond,
		FlushInterval:       5 * time.Second,
		Sinks:               []Sink{},
		Hooks:               Hooks{},
	}
}