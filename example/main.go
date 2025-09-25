package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Herocod3r/asynclog"
)

func main() {
	// Create logger configuration
	config := asynclog.DefaultConfig()
	config.BufferSize = 1000
	config.MaxGoroutines = 5
	config.BackpressureStrategy = asynclog.Block
	config.MaxRetries = 3
	config.RetryDelay = 100 * time.Millisecond
	config.FlushInterval = 2 * time.Second

	// Create output directory
	if err := os.MkdirAll("./logs", 0755); err != nil {
		log.Fatal(err)
	}

	// Create multiple sinks
	consoleSink := asynclog.NewConsoleSink(false)
	
	fileSink, err := asynclog.NewFileSink("./logs/app.log")
	if err != nil {
		log.Fatal(err)
	}

	parquetSink, err := asynclog.NewParquetSink(asynclog.ParquetSinkConfig{
		FilePath:   "./logs/app_data",
		RotateSize: 10 * 1024 * 1024, // 10MB
		RotateTime: 1 * time.Hour,    // 1 hour
	})
	if err != nil {
		log.Fatal(err)
	}

	// Configure sinks
	config.Sinks = []asynclog.Sink{consoleSink, fileSink, parquetSink}

	// Configure hooks for telemetry
	config.Hooks.OnMessageBuffered = func(msg *asynclog.LogMessage) {
		fmt.Printf("Buffered: %s [%s]\n", msg.Message, msg.Level.String())
	}

	config.Hooks.OnMessageDropped = func(msg *asynclog.LogMessage) {
		fmt.Printf("DROPPED: %s [%s]\n", msg.Message, msg.Level.String())
	}

	config.Hooks.OnBatchWritten = func(sink string, count int, duration time.Duration) {
		fmt.Printf("Wrote %d messages to %s in %v\n", count, sink, duration)
	}

	config.Hooks.OnBatchFailed = func(sink string, count int, err error, attempts int) {
		fmt.Printf("Failed to write %d messages to %s after %d attempts: %v\n", 
			count, sink, attempts, err)
	}

	config.Hooks.OnFlushCompleted = func(totalMessages int, duration time.Duration) {
		fmt.Printf("Flushed %d messages in %v\n", totalMessages, duration)
	}

	// Create the logger
	logger, err := asynclog.New(config)
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Close()

	// Log some messages with different levels
	logger.Info("Application started", map[string]interface{}{
		"version": "1.0.0",
		"port":    8080,
	})

	logger.Debug("Debug information", map[string]interface{}{
		"user_id":   12345,
		"operation": "login",
	})

	logger.Warn("Warning message", map[string]interface{}{
		"memory_usage": "85%",
		"threshold":    "80%",
	})

	logger.Error("Error occurred", map[string]interface{}{
		"error":      "connection timeout",
		"service":    "database",
		"retry_count": 3,
	})

	// Simulate some load
	for i := 0; i < 100; i++ {
		logger.Info("Processing request", map[string]interface{}{
			"request_id": i,
			"method":     "GET",
			"path":       "/api/users",
			"duration_ms": 150 + i,
		})
		
		if i%10 == 0 {
			time.Sleep(10 * time.Millisecond) // Brief pause
		}
	}

	fmt.Println("Logging completed. Flushing remaining messages...")
	
	// Final flush before closing
	if err := logger.Flush(); err != nil {
		log.Printf("Error during flush: %v", err)
	}

	fmt.Println("Example completed successfully!")
}