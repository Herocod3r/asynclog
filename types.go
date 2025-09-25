package asynclog

import (
	"context"
	"encoding/json"
	"time"
)

// LogLevel represents the severity level of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// LogMessage represents a single log entry
type LogMessage struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     LogLevel               `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// ToJSON converts the log message to JSON bytes
func (lm *LogMessage) ToJSON() ([]byte, error) {
	return json.Marshal(lm)
}

// Sink represents an output destination for log messages
type Sink interface {
	// Write writes a batch of log messages
	Write(ctx context.Context, messages []*LogMessage) error
	// Close closes the sink and flushes any remaining data
	Close() error
	// Name returns a human-readable name for the sink
	Name() string
}

// Hooks provides callbacks for various logger events
type Hooks struct {
	// OnMessageBuffered is called when a message is successfully buffered
	OnMessageBuffered func(message *LogMessage)
	// OnMessageDropped is called when a message is dropped due to backpressure
	OnMessageDropped func(message *LogMessage)
	// OnBatchWritten is called when a batch is successfully written to a sink
	OnBatchWritten func(sink string, count int, duration time.Duration)
	// OnBatchFailed is called when a batch write fails
	OnBatchFailed func(sink string, count int, err error, attempts int)
	// OnFlushCompleted is called when a flush operation completes
	OnFlushCompleted func(totalMessages int, duration time.Duration)
}