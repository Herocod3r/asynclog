package asynclog

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrLoggerClosed = errors.New("logger is closed")
	ErrBufferFull   = errors.New("buffer is full")
)

// AsyncLogger is an in-memory async logger that buffers log messages
type AsyncLogger struct {
	config     *Config
	buffer     chan *LogMessage
	done       chan struct{}
	wg         sync.WaitGroup
	once       sync.Once
	closed     bool
	closeMutex sync.RWMutex
}

// New creates a new AsyncLogger with the given configuration
func New(config *Config) (*AsyncLogger, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	logger := &AsyncLogger{
		config: config,
		buffer: make(chan *LogMessage, config.BufferSize),
		done:   make(chan struct{}),
	}

	// Start worker goroutines
	for i := 0; i < config.MaxGoroutines; i++ {
		logger.wg.Add(1)
		go logger.worker()
	}

	// Start flush ticker if flush interval is set
	if config.FlushInterval > 0 {
		logger.wg.Add(1)
		go logger.flushTicker()
	}

	return logger, nil
}

func validateConfig(config *Config) error {
	if config.BufferSize <= 0 {
		return errors.New("buffer size must be positive")
	}
	if config.MaxGoroutines <= 0 {
		return errors.New("max goroutines must be positive")
	}
	if config.MaxRetries < 0 {
		return errors.New("max retries cannot be negative")
	}
	return nil
}

// Log adds a log message to the buffer
func (al *AsyncLogger) Log(level LogLevel, message string, fields map[string]interface{}) error {
	return al.LogMessage(&LogMessage{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Fields:    fields,
	})
}

// LogMessage adds a pre-constructed log message to the buffer
func (al *AsyncLogger) LogMessage(msg *LogMessage) error {
	al.closeMutex.RLock()
	defer al.closeMutex.RUnlock()

	if al.closed {
		return ErrLoggerClosed
	}

	switch al.config.BackpressureStrategy {
	case Block:
		select {
		case al.buffer <- msg:
			if al.config.Hooks.OnMessageBuffered != nil {
				al.config.Hooks.OnMessageBuffered(msg)
			}
			return nil
		case <-al.done:
			return ErrLoggerClosed
		}
	case Drop:
		select {
		case al.buffer <- msg:
			if al.config.Hooks.OnMessageBuffered != nil {
				al.config.Hooks.OnMessageBuffered(msg)
			}
			return nil
		default:
			if al.config.Hooks.OnMessageDropped != nil {
				al.config.Hooks.OnMessageDropped(msg)
			}
			return ErrBufferFull
		}
	default:
		return errors.New("unknown backpressure strategy")
	}
}

// Debug logs a debug message
func (al *AsyncLogger) Debug(message string, fields map[string]interface{}) error {
	return al.Log(DEBUG, message, fields)
}

// Info logs an info message
func (al *AsyncLogger) Info(message string, fields map[string]interface{}) error {
	return al.Log(INFO, message, fields)
}

// Warn logs a warning message
func (al *AsyncLogger) Warn(message string, fields map[string]interface{}) error {
	return al.Log(WARN, message, fields)
}

// Error logs an error message
func (al *AsyncLogger) Error(message string, fields map[string]interface{}) error {
	return al.Log(ERROR, message, fields)
}

// Fatal logs a fatal message
func (al *AsyncLogger) Fatal(message string, fields map[string]interface{}) error {
	return al.Log(FATAL, message, fields)
}

// Flush forces all buffered messages to be written to sinks
func (al *AsyncLogger) Flush() error {
	al.closeMutex.RLock()
	defer al.closeMutex.RUnlock()

	if al.closed {
		return ErrLoggerClosed
	}

	start := time.Now()
	var messages []*LogMessage

	// Collect all buffered messages
	for {
		select {
		case msg := <-al.buffer:
			messages = append(messages, msg)
		default:
			goto flush
		}
	}

flush:
	if len(messages) > 0 {
		al.writeToSinks(context.Background(), messages)
	}

	al.callHookFlushCompleted(len(messages), time.Since(start))
	return nil
}

// Close shuts down the logger and flushes remaining messages
func (al *AsyncLogger) Close() error {
	al.closeMutex.Lock()
	defer al.closeMutex.Unlock()

	if al.closed {
		return nil
	}

	al.once.Do(func() {
		close(al.done)
		al.closed = true
	})

	// Wait for workers to finish
	al.wg.Wait()

	// Flush remaining messages
	var remaining []*LogMessage
	for {
		select {
		case msg := <-al.buffer:
			remaining = append(remaining, msg)
		default:
			goto done
		}
	}

done:
	if len(remaining) > 0 {
		al.writeToSinks(context.Background(), remaining)
	}

	// Close all sinks
	for _, sink := range al.config.Sinks {
		if err := sink.Close(); err != nil {
			// Log error but continue closing other sinks
			fmt.Printf("Error closing sink %s: %v\n", sink.Name(), err)
		}
	}

	return nil
}

// worker is a background goroutine that processes buffered messages
func (al *AsyncLogger) worker() {
	defer al.wg.Done()

	batch := make([]*LogMessage, 0, 100) // Pre-allocate batch slice
	ticker := time.NewTicker(100 * time.Millisecond) // Batch processing interval
	defer ticker.Stop()

	for {
		select {
		case msg := <-al.buffer:
			batch = append(batch, msg)
			
			// Process batch if it's full or we've been collecting for a while
			if len(batch) >= 100 {
				al.processBatch(batch)
				batch = batch[:0] // Reset slice but keep capacity
			}

		case <-ticker.C:
			if len(batch) > 0 {
				al.processBatch(batch)
				batch = batch[:0] // Reset slice but keep capacity
			}

		case <-al.done:
			// Process any remaining messages
			if len(batch) > 0 {
				al.processBatch(batch)
			}
			return
		}
	}
}

// flushTicker periodically flushes the buffer
func (al *AsyncLogger) flushTicker() {
	defer al.wg.Done()
	ticker := time.NewTicker(al.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			al.Flush()
		case <-al.done:
			return
		}
	}
}

// processBatch writes a batch of messages to all sinks
func (al *AsyncLogger) processBatch(messages []*LogMessage) {
	if len(messages) == 0 {
		return
	}

	// Make a copy to avoid race conditions
	batch := make([]*LogMessage, len(messages))
	copy(batch, messages)

	al.writeToSinks(context.Background(), batch)
}

// writeToSinks writes messages to all configured sinks with retry logic
func (al *AsyncLogger) writeToSinks(ctx context.Context, messages []*LogMessage) {
	for _, sink := range al.config.Sinks {
		al.writeToSinkWithRetry(ctx, sink, messages)
	}
}

// writeToSinkWithRetry writes to a single sink with retry logic
func (al *AsyncLogger) writeToSinkWithRetry(ctx context.Context, sink Sink, messages []*LogMessage) {
	start := time.Now()

	for attempt := 0; attempt <= al.config.MaxRetries; attempt++ {
		err := sink.Write(ctx, messages)
		if err == nil {
			// Success
			if al.config.Hooks.OnBatchWritten != nil {
				al.config.Hooks.OnBatchWritten(sink.Name(), len(messages), time.Since(start))
			}
			return
		}

		if attempt < al.config.MaxRetries {
			time.Sleep(al.config.RetryDelay)
		} else {
			// Final failure
			if al.config.Hooks.OnBatchFailed != nil {
				al.config.Hooks.OnBatchFailed(sink.Name(), len(messages), err, attempt+1)
			}
		}
	}
}

// Helper method to safely call flush completed hook
func (al *AsyncLogger) callHookFlushCompleted(totalMessages int, duration time.Duration) {
	if al.config.Hooks.OnFlushCompleted != nil {
		al.config.Hooks.OnFlushCompleted(totalMessages, duration)
	}
}