package asynclog

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

// TestSink is a test sink that collects messages for verification
type TestSink struct {
	messages []*LogMessage
	mu       sync.RWMutex
	failures int
}

func NewTestSink() *TestSink {
	return &TestSink{
		messages: make([]*LogMessage, 0),
	}
}

func (ts *TestSink) Write(ctx context.Context, messages []*LogMessage) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	
	if ts.failures > 0 {
		ts.failures--
		return context.DeadlineExceeded // Simulate a retryable error
	}
	
	ts.messages = append(ts.messages, messages...)
	return nil
}

func (ts *TestSink) Close() error {
	return nil
}

func (ts *TestSink) Name() string {
	return "test-sink"
}

func (ts *TestSink) GetMessages() []*LogMessage {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	
	result := make([]*LogMessage, len(ts.messages))
	copy(result, ts.messages)
	return result
}

func (ts *TestSink) SetFailures(count int) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.failures = count
}

func TestAsyncLogger_BasicLogging(t *testing.T) {
	config := DefaultConfig()
	config.BufferSize = 10
	config.MaxGoroutines = 2
	
	testSink := NewTestSink()
	config.Sinks = []Sink{testSink}
	
	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	
	// Log some messages
	err = logger.Info("test message 1", map[string]interface{}{"key1": "value1"})
	if err != nil {
		t.Errorf("Failed to log message: %v", err)
	}
	
	err = logger.Error("test message 2", map[string]interface{}{"key2": "value2"})
	if err != nil {
		t.Errorf("Failed to log message: %v", err)
	}
	
	// Wait for processing
	time.Sleep(200 * time.Millisecond)
	
	// Flush to ensure all messages are processed
	err = logger.Flush()
	if err != nil {
		t.Errorf("Failed to flush: %v", err)
	}
	
	// Check messages
	messages := testSink.GetMessages()
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}
	
	if messages[0].Level != INFO {
		t.Errorf("Expected INFO level, got %v", messages[0].Level)
	}
	
	if messages[1].Level != ERROR {
		t.Errorf("Expected ERROR level, got %v", messages[1].Level)
	}
}

func TestAsyncLogger_BackpressureDrop(t *testing.T) {
	config := DefaultConfig()
	config.BufferSize = 2
	config.MaxGoroutines = 1
	config.BackpressureStrategy = Drop
	
	testSink := NewTestSink()
	config.Sinks = []Sink{testSink}
	
	var droppedCount int
	config.Hooks.OnMessageDropped = func(msg *LogMessage) {
		droppedCount++
	}
	
	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	
	// Fill buffer and overflow
	for i := 0; i < 10; i++ {
		logger.Info("message", map[string]interface{}{"i": i})
	}
	
	if droppedCount == 0 {
		t.Error("Expected some messages to be dropped")
	}
}

func TestAsyncLogger_Hooks(t *testing.T) {
	config := DefaultConfig()
	config.BufferSize = 10
	config.MaxGoroutines = 1
	
	testSink := NewTestSink()
	config.Sinks = []Sink{testSink}
	
	var bufferedCount, writtenCount int
	var writtenMessages int
	
	config.Hooks.OnMessageBuffered = func(msg *LogMessage) {
		bufferedCount++
	}
	
	config.Hooks.OnBatchWritten = func(sink string, count int, duration time.Duration) {
		writtenCount++
		writtenMessages += count
	}
	
	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	
	// Log messages
	logger.Info("message 1", nil)
	logger.Info("message 2", nil)
	
	// Wait for processing
	time.Sleep(200 * time.Millisecond)
	logger.Flush()
	
	if bufferedCount != 2 {
		t.Errorf("Expected 2 buffered messages, got %d", bufferedCount)
	}
	
	if writtenMessages != 2 {
		t.Errorf("Expected 2 written messages, got %d", writtenMessages)
	}
}

func TestAsyncLogger_RetryLogic(t *testing.T) {
	config := DefaultConfig()
	config.BufferSize = 10
	config.MaxGoroutines = 1
	config.MaxRetries = 2
	config.RetryDelay = 10 * time.Millisecond
	
	testSink := NewTestSink()
	testSink.SetFailures(1) // Fail once, then succeed
	config.Sinks = []Sink{testSink}
	
	var failureCount int
	config.Hooks.OnBatchFailed = func(sink string, count int, err error, attempts int) {
		failureCount++
	}
	
	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()
	
	logger.Info("test message", nil)
	
	// Wait for processing and retries
	time.Sleep(200 * time.Millisecond)
	logger.Flush()
	
	// Message should eventually succeed after retry
	messages := testSink.GetMessages()
	if len(messages) != 1 {
		t.Errorf("Expected 1 message to succeed after retry, got %d", len(messages))
	}
}

func TestConsoleSink(t *testing.T) {
	sink := NewConsoleSink(false)
	
	if sink.Name() != "console(stdout)" {
		t.Errorf("Expected console(stdout), got %s", sink.Name())
	}
	
	message := &LogMessage{
		Timestamp: time.Now(),
		Level:     INFO,
		Message:   "test message",
		Fields:    map[string]interface{}{"key": "value"},
	}
	
	err := sink.Write(context.Background(), []*LogMessage{message})
	if err != nil {
		t.Errorf("Failed to write to console sink: %v", err)
	}
}

func TestFileSink(t *testing.T) {
	tmpFile := "/tmp/test_log.json"
	defer os.Remove(tmpFile)
	
	sink, err := NewFileSink(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create file sink: %v", err)
	}
	defer sink.Close()
	
	message := &LogMessage{
		Timestamp: time.Now(),
		Level:     INFO,
		Message:   "test message",
		Fields:    map[string]interface{}{"key": "value"},
	}
	
	err = sink.Write(context.Background(), []*LogMessage{message})
	if err != nil {
		t.Errorf("Failed to write to file sink: %v", err)
	}
	
	// Verify file exists
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Error("Log file was not created")
	}
}

func TestParquetSink(t *testing.T) {
	tmpDir := "/tmp"
	config := ParquetSinkConfig{
		FilePath:   tmpDir + "/test_log",
		RotateSize: 1024 * 1024, // 1MB
	}
	
	sink, err := NewParquetSink(config)
	if err != nil {
		t.Fatalf("Failed to create parquet sink: %v", err)
	}
	defer sink.Close()
	
	message := &LogMessage{
		Timestamp: time.Now(),
		Level:     INFO,
		Message:   "test message",
		Fields:    map[string]interface{}{"key": "value"},
	}
	
	err = sink.Write(context.Background(), []*LogMessage{message})
	if err != nil {
		t.Errorf("Failed to write to parquet sink: %v", err)
	}
}