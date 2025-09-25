# AsyncLog

AsyncLog is a high-performance Go package that provides an in-memory async logger with buffering capabilities and multiple output sinks, including Parquet file support.

## Features

- **Asynchronous logging** with configurable buffer size and goroutine pool
- **Multiple output sinks**: Console, File, and Parquet
- **Configurable backpressure handling**: Block or Drop strategies
- **Retry mechanism** with configurable max retries and delay
- **File rotation** for Parquet files based on size or time
- **Telemetry hooks** for monitoring logger operations
- **Structured logging** with JSON serialization
- **Thread-safe** operations

## Installation

```bash
go get github.com/Herocod3r/asynclog
```

## Quick Start

```go
package main

import (
    "github.com/Herocod3r/asynclog"
    "log"
)

func main() {
    // Create logger with default configuration
    config := asynclog.DefaultConfig()
    
    // Add a console sink
    config.Sinks = []asynclog.Sink{
        asynclog.NewConsoleSink(false), // false = stdout, true = stderr
    }
    
    logger, err := asynclog.New(config)
    if err != nil {
        log.Fatal(err)
    }
    defer logger.Close()
    
    // Log messages
    logger.Info("Hello, AsyncLog!", map[string]interface{}{
        "version": "1.0.0",
        "user_id": 12345,
    })
}
```

## Configuration

The `Config` struct provides comprehensive configuration options:

```go
type Config struct {
    // BufferSize is the maximum number of messages to buffer in memory
    BufferSize int
    
    // MaxGoroutines is the maximum number of goroutines in the worker pool
    MaxGoroutines int
    
    // BackpressureStrategy determines behavior when buffer is full
    BackpressureStrategy BackpressureStrategy // Block or Drop
    
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
```

### Default Configuration

```go
config := asynclog.DefaultConfig()
// Sets:
// - BufferSize: 1000
// - MaxGoroutines: 10
// - BackpressureStrategy: Block
// - MaxRetries: 3
// - RetryDelay: 100ms
// - FlushInterval: 5 seconds
```

## Sinks

AsyncLog supports multiple output sinks that can be used simultaneously:

### Console Sink

```go
consoleSink := asynclog.NewConsoleSink(false) // stdout
consoleSink := asynclog.NewConsoleSink(true)  // stderr
```

### File Sink

```go
fileSink, err := asynclog.NewFileSink("./logs/app.log")
if err != nil {
    log.Fatal(err)
}
```

### Parquet Sink

```go
parquetSink, err := asynclog.NewParquetSink(asynclog.ParquetSinkConfig{
    FilePath:   "./logs/app_data",
    RotateSize: 10 * 1024 * 1024, // 10MB rotation
    RotateTime: 1 * time.Hour,    // 1 hour rotation
})
if err != nil {
    log.Fatal(err)
}
```

## Logging Methods

AsyncLog provides methods for different log levels:

```go
// Standard logging methods with fields
logger.Debug("Debug message", map[string]interface{}{"key": "value"})
logger.Info("Info message", map[string]interface{}{"key": "value"})
logger.Warn("Warning message", map[string]interface{}{"key": "value"})
logger.Error("Error message", map[string]interface{}{"key": "value"})
logger.Fatal("Fatal message", map[string]interface{}{"key": "value"})

// Generic logging with custom level
logger.Log(asynclog.INFO, "Custom message", map[string]interface{}{"key": "value"})

// Pre-constructed log message
msg := &asynclog.LogMessage{
    Timestamp: time.Now(),
    Level:     asynclog.INFO,
    Message:   "Custom message",
    Fields:    map[string]interface{}{"key": "value"},
}
logger.LogMessage(msg)
```

## Backpressure Handling

Configure how the logger behaves when the buffer is full:

```go
config.BackpressureStrategy = asynclog.Block // Wait for space (default)
config.BackpressureStrategy = asynclog.Drop  // Drop new messages
```

## Telemetry and Hooks

Monitor logger operations with hooks:

```go
config.Hooks = asynclog.Hooks{
    OnMessageBuffered: func(msg *asynclog.LogMessage) {
        // Called when a message is buffered
    },
    OnMessageDropped: func(msg *asynclog.LogMessage) {
        // Called when a message is dropped (Drop strategy)
    },
    OnBatchWritten: func(sink string, count int, duration time.Duration) {
        // Called when a batch is successfully written
    },
    OnBatchFailed: func(sink string, count int, err error, attempts int) {
        // Called when a batch write fails after all retries
    },
    OnFlushCompleted: func(totalMessages int, duration time.Duration) {
        // Called when a flush operation completes
    },
}
```

## Error Handling

The logger provides graceful error handling:

- **Retry Logic**: Failed writes are retried up to `MaxRetries` times
- **Non-blocking**: Errors don't stop the logger from processing other messages
- **Telemetry**: Failed operations are reported through hooks

## Performance Considerations

- **Buffering**: Messages are buffered in memory and written in batches
- **Goroutine Pool**: Configurable number of worker goroutines for parallel processing
- **Async Processing**: Logging calls return immediately, processing happens asynchronously
- **Batching**: Multiple messages are written together to reduce I/O overhead

## Thread Safety

AsyncLog is fully thread-safe. Multiple goroutines can log concurrently without additional synchronization.

## Graceful Shutdown

Always close the logger to ensure all messages are flushed:

```go
defer logger.Close() // Flushes remaining messages and closes sinks
```

Or manually flush when needed:

```go
if err := logger.Flush(); err != nil {
    log.Printf("Error during flush: %v", err)
}
```

## Complete Example

See the [example](example/main.go) for a comprehensive demonstration of all features.

## License

MIT License